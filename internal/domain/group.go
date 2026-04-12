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
	CreatedAt   time.Time `json:"created_at"`
}

type GroupMember struct {
	GroupID  uuid.UUID `json:"group_id"`
	UserID   uuid.UUID `json:"user_id"`
	Role     GroupRole `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}
