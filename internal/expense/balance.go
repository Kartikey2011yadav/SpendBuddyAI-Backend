package expense

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

// GetNetBalance returns the caller's net balance in a group (in cents).
// Positive → others owe the user. Negative → user owes others.
func (s *Service) GetNetBalance(ctx context.Context, groupID, userID uuid.UUID) (int64, error) {
	return s.repo.GetNetBalance(ctx, groupID, userID)
}

// GetGroupBalances returns every member's net balance in the group (in cents).
func (s *Service) GetGroupBalances(ctx context.Context, groupID uuid.UUID) ([]*domain.UserBalance, error) {
	return s.repo.GetGroupBalances(ctx, groupID)
}

// SimplifyDebts takes raw per-user balances (in cents) and returns the minimum
// set of transactions that settles all debts (greedy min-cash-flow algorithm).
func SimplifyDebts(balances []*domain.UserBalance) []*domain.DebtSummary {
	type entry struct {
		id   uuid.UUID
		name string
		bal  int64
	}

	entries := make([]entry, len(balances))
	for i, b := range balances {
		entries[i] = entry{id: b.UserID, name: b.DisplayName, bal: b.NetBalance}
	}

	var debts []*domain.DebtSummary

	for {
		sort.Slice(entries, func(i, j int) bool { return entries[i].bal < entries[j].bal })

		debtor := &entries[0]                    // most negative
		creditor := &entries[len(entries)-1]     // most positive

		if debtor.bal == 0 && creditor.bal == 0 {
			break
		}
		if debtor.bal == 0 || creditor.bal == 0 {
			break
		}

		amount := minInt64(-debtor.bal, creditor.bal)

		debts = append(debts, &domain.DebtSummary{
			FromUserID:   debtor.id,
			FromUserName: debtor.name,
			ToUserID:     creditor.id,
			ToUserName:   creditor.name,
			Amount:       amount,
		})

		debtor.bal += amount
		creditor.bal -= amount
	}

	return debts
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
