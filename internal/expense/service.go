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
	repo      domain.ExpenseRepository
	groupRepo domain.GroupRepository
}

func NewService(repo domain.ExpenseRepository, groupRepo domain.GroupRepository) *Service {
	return &Service{repo: repo, groupRepo: groupRepo}
}

type CreateExpenseInput struct {
	GroupID     uuid.UUID
	PayerID     uuid.UUID
	Amount      int64 // integer cents
	Description string
	SplitMethod domain.SplitMethod
	// For SplitExact: map userID → cents. For SplitPercentage: map userID → basis points (1/100 of a percent, i.e. 5000 = 50%).
	// Callers passing percentages should use float64 values * 100 rounded to nearest int,
	// but we keep it simple: for SplitPercentage the values are the percentage * 100 (integer basis points).
	// Actually: for SplitPercentage, values are the raw percentage as int64 * 100 (e.g. 50.25% → 5025).
	// Simpler: callers pass percentages as float64-derived int64 via the handler. See handler for details.
	Splits map[uuid.UUID]int64
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
		n := int64(len(members))
		share := in.Amount / n
		remainder := in.Amount - share*n
		splits := make([]*domain.ExpenseSplit, 0, len(members))
		for i, m := range members {
			amt := share
			if int64(i) == n-1 {
				amt += remainder // last member absorbs rounding remainder
			}
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
		var total int64
		for _, v := range in.Splits {
			total += v
		}
		if total != in.Amount {
			return nil, fmt.Errorf("exact splits sum (%d) != expense amount (%d)", total, in.Amount)
		}
		splits := make([]*domain.ExpenseSplit, 0, len(in.Splits))
		for uid, amt := range in.Splits {
			splits = append(splits, &domain.ExpenseSplit{
				ID:         uuid.New(),
				UserID:     uid,
				AmountOwed: amt,
			})
		}
		return splits, nil

	case domain.SplitPercentage:
		// in.Splits maps userID → percentage as float64 * 100 stored as int64 (basis points).
		// e.g. 50% → 5000, 33.33% → 3333.
		// We convert back: pct = float64(basisPoints) / 100.0
		if len(in.Splits) == 0 {
			return nil, fmt.Errorf("splits map required for percentage split")
		}
		var totalBP int64
		for _, v := range in.Splits {
			totalBP += v
		}
		if totalBP != 10000 {
			return nil, fmt.Errorf("percentages must sum to 100 (got %d basis points, want 10000)", totalBP)
		}
		splits := make([]*domain.ExpenseSplit, 0, len(in.Splits))
		var allocated int64
		// Two-pass: compute all shares, give remainder to largest share holder.
		type bpEntry struct {
			uid uuid.UUID
			bp  int64
			amt int64
		}
		entries := make([]bpEntry, 0, len(in.Splits))
		for uid, bp := range in.Splits {
			amt := int64(math.Round(float64(in.Amount) * float64(bp) / 10000.0))
			allocated += amt
			entries = append(entries, bpEntry{uid: uid, bp: bp, amt: amt})
		}
		// Distribute rounding remainder to the entry with the largest basis points.
		diff := in.Amount - allocated
		if diff != 0 {
			maxIdx := 0
			for i, e := range entries {
				if e.bp > entries[maxIdx].bp {
					maxIdx = i
				}
			}
			entries[maxIdx].amt += diff
		}
		for _, e := range entries {
			splits = append(splits, &domain.ExpenseSplit{
				ID:         uuid.New(),
				UserID:     e.uid,
				AmountOwed: e.amt,
			})
		}
		return splits, nil

	default:
		return nil, fmt.Errorf("unknown split method: %s", in.SplitMethod)
	}
}
