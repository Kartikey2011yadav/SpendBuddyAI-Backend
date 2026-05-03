package domain

import (
	"time"

	"github.com/google/uuid"
)

type GroupRole string

const (
	RoleAdmin  GroupRole = "admin"
	RoleMember GroupRole = "member"
)

type Group struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	CreatedBy   uuid.UUID `json:"created_by"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

type GroupMember struct {
	GroupID  uuid.UUID `json:"group_id"`
	UserID   uuid.UUID `json:"user_id"`
	Role     GroupRole `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// GroupMemberDetail enriches GroupMember with the user's display info.
// Returned by the list-members endpoint (single JOIN query, no N+1).
type GroupMemberDetail struct {
	GroupID     uuid.UUID `json:"group_id"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Role        GroupRole `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}
