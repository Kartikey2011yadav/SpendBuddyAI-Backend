package domain

import (
	"time"

	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeText  MessageType = "text"
	MessageTypeImage MessageType = "image"
	MessageTypeSystem MessageType = "system"
)

type Message struct {
	ID        uuid.UUID   `json:"id"`
	GroupID   uuid.UUID   `json:"group_id"`
	UserID    uuid.UUID   `json:"user_id"`
	Content   string      `json:"content"`
	Type      MessageType `json:"type"`
	CreatedAt time.Time   `json:"created_at"`
	// Populated on read
	SenderName   string  `json:"sender_name,omitempty"`
	SenderAvatar *string `json:"sender_avatar,omitempty"`
}

// WSMessage is the envelope sent over WebSocket connections.
type WSMessage struct {
	Type    string      `json:"type"`
	GroupID string      `json:"group_id"`
	Payload interface{} `json:"payload"`
}
