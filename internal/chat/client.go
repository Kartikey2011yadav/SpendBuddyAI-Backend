package chat

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Client is a single WebSocket connection.
type Client struct {
	UserID  uuid.UUID
	GroupID uuid.UUID

	conn *websocket.Conn
	send chan []byte // buffered channel of outbound messages
	hub  *Hub
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, groupID uuid.UUID) *Client {
	return &Client{
		UserID:  userID,
		GroupID: groupID,
		conn:    conn,
		send:    make(chan []byte, 256),
		hub:     hub,
	}
}

// ReadPump pumps messages from the WebSocket to the hub.
// Run in a goroutine per client.
func (c *Client) ReadPump(onMessage func([]byte)) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("ws unexpected close", "user", c.UserID, "err", err)
			}
			break
		}
		onMessage(msg)
	}
}

// WritePump pumps messages from the hub to the WebSocket.
// Run in a goroutine per client.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("ws write error", "user", c.UserID, "err", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
