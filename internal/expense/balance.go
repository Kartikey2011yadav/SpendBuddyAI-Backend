package expense

import (
	"context"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

// GetNetBalance returns the caller's net balance in a group.
// Positive → others owe the user. Negative → user owes others.
func (s *Service) GetNetBalance(ctx context.Context, groupID, userID uuid.UUID) (float64, error) {
	return s.repo.GetNetBalance(ctx, groupID, userID)
}

// GetGroupBalances returns every member's net balance in the group.
func (s *Service) GetGroupBalances(ctx context.Context, groupID uuid.UUID) ([]*domain.UserBalance, error) {
	return s.repo.GetGroupBalances(ctx, groupID)
}

// SimplifyDebts takes raw per-user balances and returns the minimum set of
// transactions that settles all debts (greedy min-cash-flow algorithm).
func SimplifyDebts(balances []*domain.UserBalance) []*domain.DebtSummary {
	type entry struct {
		id   uuid.UUID
		name string
		bal  float64
	}

	entries := make([]entry, len(balances))
	for i, b := range balances {
		entries[i] = entry{id: b.UserID, name: b.DisplayName, bal: b.NetBalance}
	}

	var debts []*domain.DebtSummary

	for {
		// Find max creditor and max debtor
		sort.Slice(entries, func(i, j int) bool { return entries[i].bal < entries[j].bal })

		debtor := &entries[0]  // most negative
		creditor := &entries[len(entries)-1] // most positive

		if math.Abs(debtor.bal) < 0.01 && math.Abs(creditor.bal) < 0.01 {
			break // settled
		}
		if math.Abs(debtor.bal) < 0.01 || math.Abs(creditor.bal) < 0.01 {
			break
		}

		amount := math.Min(math.Abs(debtor.bal), creditor.bal)
		amount = math.Round(amount*100) / 100

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
