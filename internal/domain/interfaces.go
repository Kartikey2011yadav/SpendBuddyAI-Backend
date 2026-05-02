package domain

import (
	"context"

	"github.com/google/uuid"
)

// --- User Repository ---

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByGoogleSub(ctx context.Context, sub string) (*User, error)
	Update(ctx context.Context, user *User) error
}

// --- Group Repository ---

type GroupRepository interface {
	Create(ctx context.Context, group *Group) error
	FindByID(ctx context.Context, id uuid.UUID) (*Group, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*Group, error)
	AddMember(ctx context.Context, member *GroupMember) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	GetMembers(ctx context.Context, groupID uuid.UUID) ([]*GroupMember, error)
	IsMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error)
}

// --- Message Repository ---

type MessageRepository interface {
	Save(ctx context.Context, msg *Message) error
	ListByGroup(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*Message, error)
}

// --- Expense Repository ---

type ExpenseRepository interface {
	Create(ctx context.Context, expense *Expense, splits []*ExpenseSplit) error
	FindByID(ctx context.Context, id uuid.UUID) (*Expense, error)
	ListByGroup(ctx context.Context, groupID uuid.UUID) ([]*Expense, error)
	GetNetBalance(ctx context.Context, groupID, userID uuid.UUID) (int64, error)
	GetGroupBalances(ctx context.Context, groupID uuid.UUID) ([]*UserBalance, error)
}

// --- Auth Usecase ---

type AuthUsecase interface {
	LoginWithGoogle(ctx context.Context, idToken string) (*TokenPair, *User, error)
	SendEmailOTP(ctx context.Context, email string) error
	VerifyEmailOTP(ctx context.Context, email, code string) (*TokenPair, *User, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*TokenPair, error)
}

// TokenPair holds an access and refresh JWT.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
