package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages.
// It is safe for concurrent use.
type Hub struct {
	// rooms maps groupID → set of clients in that group
	rooms map[uuid.UUID]map[*Client]struct{}
	mu    sync.RWMutex

	// Inbound channels
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMsg
}

// BroadcastMsg is an envelope pushed onto the broadcast channel.
type BroadcastMsg struct {
	GroupID uuid.UUID
	Payload []byte // pre-serialised JSON
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan *BroadcastMsg, 256),
	}
}

// Run is the event loop — start it in a dedicated goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case c := <-h.register:
			h.mu.Lock()
			if h.rooms[c.GroupID] == nil {
				h.rooms[c.GroupID] = make(map[*Client]struct{})
			}
			h.rooms[c.GroupID][c] = struct{}{}
			h.mu.Unlock()
			slog.Info("ws client registered", "user", c.UserID, "group", c.GroupID)

		case c := <-h.unregister:
			h.mu.Lock()
			if room, ok := h.rooms[c.GroupID]; ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, c.GroupID)
				}
			}
			h.mu.Unlock()
			close(c.send) // signal the write pump to stop
			slog.Info("ws client unregistered", "user", c.UserID, "group", c.GroupID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			room := h.rooms[msg.GroupID]
			// Copy to avoid holding the lock during sends
			targets := make([]*Client, 0, len(room))
			for c := range room {
				targets = append(targets, c)
			}
			h.mu.RUnlock()

			for _, c := range targets {
				select {
				case c.send <- msg.Payload:
				default:
					// Client's send buffer is full — drop & evict
					h.unregister <- c
					slog.Warn("ws client send buffer full, evicting", "user", c.UserID)
				}
			}
		}
	}
}

// BroadcastMessage serialises msg and fans it out to all clients in msg.GroupID.
func (h *Hub) BroadcastMessage(msg *domain.Message) {
	env := domain.WSMessage{
		Type:    "new_message",
		GroupID: msg.GroupID.String(),
		Payload: msg,
	}
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("marshal ws message", "err", err)
		return
	}
	h.broadcast <- &BroadcastMsg{GroupID: msg.GroupID, Payload: data}
}

// BroadcastBalanceUpdate pushes a balance-changed event to all group members.
func (h *Hub) BroadcastBalanceUpdate(groupID uuid.UUID, balances []*domain.UserBalance) {
	env := domain.WSMessage{
		Type:    "balance_update",
		GroupID: groupID.String(),
		Payload: balances,
	}
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("marshal ws balance update", "err", err)
		return
	}
	h.broadcast <- &BroadcastMsg{GroupID: groupID, Payload: data}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) { h.register <- c }

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) { h.unregister <- c }
