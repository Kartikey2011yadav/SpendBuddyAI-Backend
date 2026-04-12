package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

type ExpenseRepository struct {
	db *pgxpool.Pool
}

func NewExpenseRepository(db *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{db: db}
}

// Create persists an expense and all its splits in a single transaction.
func (r *ExpenseRepository) Create(ctx context.Context, expense *domain.Expense, splits []*domain.ExpenseSplit) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO expenses (id, group_id, payer_id, amount_cents, description, split_method, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		expense.ID, expense.GroupID, expense.PayerID,
		toCents(expense.Amount), expense.Description, expense.SplitMethod, expense.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert expense: %w", err)
	}

	batch := &pgx.Batch{}
	for _, s := range splits {
		batch.Queue(`
			INSERT INTO expense_splits (id, expense_id, user_id, amount_owed_cents)
			VALUES ($1, $2, $3, $4)`,
			s.ID, expense.ID, s.UserID, toCents(s.AmountOwed),
		)
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert splits: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *ExpenseRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Expense, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, group_id, payer_id, amount_cents/100.0 AS amount,
		       description, split_method, created_at
		FROM expenses WHERE id = $1`, id)

	var e domain.Expense
	err := row.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.Amount, &e.Description, &e.SplitMethod, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ExpenseRepository) ListByGroup(ctx context.Context, groupID uuid.UUID) ([]*domain.Expense, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, group_id, payer_id, amount_cents/100.0 AS amount,
		       description, split_method, created_at
		FROM expenses WHERE group_id = $1
		ORDER BY created_at DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Expense
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(&e.ID, &e.GroupID, &e.PayerID, &e.Amount, &e.Description, &e.SplitMethod, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// GetNetBalance returns the net balance for a user in a group.
//
//   Net = (sum of splits the user PAID for others)
//       - (sum of splits the user OWES to others)
//
// Using a single aggregating query avoids N+1 round-trips.
func (r *ExpenseRepository) GetNetBalance(ctx context.Context, groupID, userID uuid.UUID) (float64, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
		  COALESCE(
		    SUM(CASE WHEN e.payer_id = $2 AND es.user_id != $2
		             THEN es.amount_owed_cents ELSE 0 END), 0
		  ) -
		  COALESCE(
		    SUM(CASE WHEN es.user_id = $2 AND e.payer_id != $2
		             THEN es.amount_owed_cents ELSE 0 END), 0
		  ) AS net_balance_cents
		FROM expense_splits es
		JOIN expenses e ON es.expense_id = e.id
		WHERE e.group_id = $1
		  AND (e.payer_id = $2 OR es.user_id = $2)
	`, groupID, userID)

	var cents int64
	if err := row.Scan(&cents); err != nil {
		return 0, err
	}
	return float64(cents) / 100.0, nil
}

// GetGroupBalances returns net balances for all members in a group.
func (r *ExpenseRepository) GetGroupBalances(ctx context.Context, groupID uuid.UUID) ([]*domain.UserBalance, error) {
	rows, err := r.db.Query(ctx, `
		WITH member_balance AS (
		  SELECT
		    m.user_id,
		    COALESCE(
		      SUM(CASE WHEN e.payer_id = m.user_id AND es.user_id != m.user_id
		               THEN es.amount_owed_cents ELSE 0 END), 0
		    ) -
		    COALESCE(
		      SUM(CASE WHEN es.user_id = m.user_id AND e.payer_id != m.user_id
		               THEN es.amount_owed_cents ELSE 0 END), 0
		    ) AS net_cents
		  FROM group_members m
		  LEFT JOIN expenses e  ON e.group_id = m.group_id
		  LEFT JOIN expense_splits es ON es.expense_id = e.id
		                              AND (es.user_id = m.user_id OR e.payer_id = m.user_id)
		  WHERE m.group_id = $1
		  GROUP BY m.user_id
		)
		SELECT mb.user_id, u.display_name, mb.net_cents / 100.0 AS net_balance
		FROM member_balance mb
		JOIN users u ON u.id = mb.user_id
		ORDER BY mb.net_cents DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.UserBalance
	for rows.Next() {
		var b domain.UserBalance
		if err := rows.Scan(&b.UserID, &b.DisplayName, &b.NetBalance); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

func toCents(amount float64) int64 {
	return int64(amount * 100)
}
