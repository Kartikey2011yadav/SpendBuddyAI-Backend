package expense

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

type Service struct {
	repo    domain.ExpenseRepository
	groupRepo domain.GroupRepository
}

func NewService(repo domain.ExpenseRepository, groupRepo domain.GroupRepository) *Service {
	return &Service{repo: repo, groupRepo: groupRepo}
}

type CreateExpenseInput struct {
	GroupID     uuid.UUID
	PayerID     uuid.UUID
	Amount      float64 // in dollars/major currency unit
	Description string
	SplitMethod domain.SplitMethod
	// For SplitExact / SplitPercentage: map userID → amount or percentage
	Splits map[uuid.UUID]float64
}

func (s *Service) CreateExpense(ctx context.Context, in CreateExpenseInput) (*domain.Expense, error) {
	if in.Amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	members, err := s.groupRepo.GetMembers(ctx, in.GroupID)
	if err != nil {
		return nil, fmt.Errorf("get members: %w", err)
	}
	if len(members) == 0 {
		return nil, fmt.Errorf("group has no members")
	}

	splits, err := s.computeSplits(in, members)
	if err != nil {
		return nil, err
	}

	expense := &domain.Expense{
		ID:          uuid.New(),
		GroupID:     in.GroupID,
		PayerID:     in.PayerID,
		Amount:      in.Amount,
		Description: in.Description,
		SplitMethod: in.SplitMethod,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, expense, splits); err != nil {
		return nil, fmt.Errorf("persist expense: %w", err)
	}

	return expense, nil
}

func (s *Service) computeSplits(in CreateExpenseInput, members []*domain.GroupMember) ([]*domain.ExpenseSplit, error) {
	switch in.SplitMethod {
	case domain.SplitEqual:
		n := float64(len(members))
		share := roundCents(in.Amount / n)
		splits := make([]*domain.ExpenseSplit, 0, len(members))
		var allocated float64
		for i, m := range members {
			amt := share
			if i == len(members)-1 {
				// Last member absorbs rounding remainder
				amt = roundCents(in.Amount - allocated)
			}
			allocated += amt
			splits = append(splits, &domain.ExpenseSplit{
				ID:         uuid.New(),
				UserID:     m.UserID,
				AmountOwed: amt,
			})
		}
		return splits, nil

	case domain.SplitExact:
		if len(in.Splits) == 0 {
			return nil, fmt.Errorf("splits map required for exact split")
		}
		var total float64
		for _, v := range in.Splits {
			total += v
		}
		if math.Abs(total-in.Amount) > 0.01 {
			return nil, fmt.Errorf("exact splits sum (%.2f) != expense amount (%.2f)", total, in.Amount)
		}
		splits := make([]*domain.ExpenseSplit, 0, len(in.Splits))
		for uid, amt := range in.Splits {
			splits = append(splits, &domain.ExpenseSplit{
				ID:         uuid.New(),
				UserID:     uid,
				AmountOwed: roundCents(amt),
			})
		}
		return splits, nil

	case domain.SplitPercentage:
		if len(in.Splits) == 0 {
			return nil, fmt.Errorf("splits map required for percentage split")
		}
		var totalPct float64
		for _, v := range in.Splits {
			totalPct += v
		}
		if math.Abs(totalPct-100) > 0.01 {
			return nil, fmt.Errorf("percentages must sum to 100, got %.2f", totalPct)
		}
		splits := make([]*domain.ExpenseSplit, 0, len(in.Splits))
		for uid, pct := range in.Splits {
			splits = append(splits, &domain.ExpenseSplit{
				ID:         uuid.New(),
				UserID:     uid,
				AmountOwed: roundCents(in.Amount * pct / 100),
			})
		}
		return splits, nil

	default:
		return nil, fmt.Errorf("unknown split method: %s", in.SplitMethod)
	}
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
