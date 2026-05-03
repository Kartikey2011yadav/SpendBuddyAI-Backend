package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

type GroupRepository struct {
	db *pgxpool.Pool
}

func NewGroupRepository(db *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) Create(ctx context.Context, g *domain.Group) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO groups (id, name, description, avatar_url, created_by, currency, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		g.ID, g.Name, g.Description, g.AvatarURL, g.CreatedBy, g.Currency, g.CreatedAt,
	)
	return err
}

func (r *GroupRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Group, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, name, description, avatar_url, created_by, currency, created_at
		FROM groups WHERE id=$1`, id)
	var g domain.Group
	if err := row.Scan(&g.ID, &g.Name, &g.Description, &g.AvatarURL, &g.CreatedBy, &g.Currency, &g.CreatedAt); err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GroupRepository) UpdateGroup(ctx context.Context, g *domain.Group) error {
	_, err := r.db.Exec(ctx, `
		UPDATE groups SET name=$1, description=$2, avatar_url=$3 WHERE id=$4`,
		g.Name, g.Description, g.AvatarURL, g.ID,
	)
	return err
}

func (r *GroupRepository) Delete(ctx context.Context, groupID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM groups WHERE id=$1`, groupID)
	return err
}

func (r *GroupRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Group, error) {
	rows, err := r.db.Query(ctx, `
		SELECT g.id, g.name, g.description, g.avatar_url, g.created_by, g.currency, g.created_at
		FROM groups g
		JOIN group_members m ON m.group_id = g.id
		WHERE m.user_id = $1
		ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Group
	for rows.Next() {
		var g domain.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.AvatarURL, &g.CreatedBy, &g.Currency, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

func (r *GroupRepository) AddMember(ctx context.Context, m *domain.GroupMember) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, user_id) DO NOTHING`,
		m.GroupID, m.UserID, m.Role, m.JoinedAt,
	)
	return err
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`, groupID, userID)
	return err
}

func (r *GroupRepository) GetMembers(ctx context.Context, groupID uuid.UUID) ([]*domain.GroupMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT group_id, user_id, role, joined_at FROM group_members WHERE group_id=$1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.GroupMember
	for rows.Next() {
		var m domain.GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`,
		groupID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is member check: %w", err)
	}
	return exists, nil
}

func (r *GroupRepository) GetMembersWithDetails(ctx context.Context, groupID uuid.UUID) ([]*domain.GroupMemberDetail, error) {
	rows, err := r.db.Query(ctx, `
		SELECT gm.group_id, gm.user_id, u.display_name, u.avatar_url, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.GroupMemberDetail
	for rows.Next() {
		var m domain.GroupMemberDetail
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.DisplayName, &m.AvatarURL, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (r *GroupRepository) GetMemberRole(ctx context.Context, groupID, userID uuid.UUID) (domain.GroupRole, error) {
	var role domain.GroupRole
	err := r.db.QueryRow(ctx,
		`SELECT role FROM group_members WHERE group_id=$1 AND user_id=$2`,
		groupID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("not a member")
		}
		return "", err
	}
	return role, nil
}

func (r *GroupRepository) UpdateMemberRole(ctx context.Context, groupID, userID uuid.UUID, role domain.GroupRole) error {
	_, err := r.db.Exec(ctx,
		`UPDATE group_members SET role=$1 WHERE group_id=$2 AND user_id=$3`,
		role, groupID, userID,
	)
	return err
}

func (r *GroupRepository) GetCurrency(ctx context.Context, groupID uuid.UUID) (string, error) {
	var currency string
	err := r.db.QueryRow(ctx, `SELECT currency FROM groups WHERE id=$1`, groupID).Scan(&currency)
	if err != nil {
		return "", fmt.Errorf("get group currency: %w", err)
	}
	return currency, nil
}
