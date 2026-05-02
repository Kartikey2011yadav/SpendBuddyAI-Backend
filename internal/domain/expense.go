package domain

import (
	"time"

	"github.com/google/uuid"
)

type SplitMethod string

const (
	SplitEqual      SplitMethod = "equal"
	SplitExact      SplitMethod = "exact"
	SplitPercentage SplitMethod = "percentage"
)

type Expense struct {
	ID          uuid.UUID   `json:"id"`
	GroupID     uuid.UUID   `json:"group_id"`
	PayerID     uuid.UUID   `json:"payer_id"`
	Amount      int64       `json:"amount"` // integer cents
	Description string      `json:"description"`
	SplitMethod SplitMethod `json:"split_method"`
	CreatedAt   time.Time   `json:"created_at"`
}

type ExpenseSplit struct {
	ID         uuid.UUID `json:"id"`
	ExpenseID  uuid.UUID `json:"expense_id"`
	UserID     uuid.UUID `json:"user_id"`
	AmountOwed int64     `json:"amount_owed"` // integer cents
}

// UserBalance represents a user's net balance within a group.
// Positive = others owe this user. Negative = this user owes others.
type UserBalance struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	NetBalance  int64     `json:"net_balance"` // integer cents
}

// DebtSummary represents a simplified debt between two users.
type DebtSummary struct {
	FromUserID   uuid.UUID `json:"from_user_id"`
	FromUserName string    `json:"from_user_name"`
	ToUserID     uuid.UUID `json:"to_user_id"`
	ToUserName   string    `json:"to_user_name"`
	Amount       int64     `json:"amount"` // integer cents
}
