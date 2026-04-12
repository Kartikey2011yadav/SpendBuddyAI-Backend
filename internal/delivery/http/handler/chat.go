package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kartikeyyadav/spendbuddy/internal/chat"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:    func(r *http.Request) bool { return true }, // tighten in production
}

type ChatHandler struct {
	hub     *chat.Hub
	msgRepo domain.MessageRepository
	grpRepo domain.GroupRepository
}

func NewChatHandler(hub *chat.Hub, msgRepo domain.MessageRepository, grpRepo domain.GroupRepository) *ChatHandler {
	return &ChatHandler{hub: hub, msgRepo: msgRepo, grpRepo: grpRepo}
}

// GET /ws/groups/:group_id  (requires JWT via query param ?token=... or header)
func (h *ChatHandler) ServeWS(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user id in token")
	}

	// Authorization: caller must be a group member
	ok, err := h.grpRepo.IsMember(c.Request().Context(), groupID, userID)
	if err != nil || !ok {
		return echo.NewHTTPError(http.StatusForbidden, "not a group member")
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	client := chat.NewClient(h.hub, conn, userID, groupID)
	h.hub.Register(client)

	// Each client gets two goroutines: one to read, one to write.
	go client.WritePump()
	go client.ReadPump(func(raw []byte) {
		var incoming struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &incoming); err != nil || incoming.Content == "" {
			return
		}

		msg := &domain.Message{
			ID:        uuid.New(),
			GroupID:   groupID,
			UserID:    userID,
			Content:   incoming.Content,
			Type:      domain.MessageTypeText,
			CreatedAt: time.Now(),
		}

		// Persist asynchronously — don't block the read loop
		go func() {
			ctx := c.Request().Context()
			if err := h.msgRepo.Save(ctx, msg); err != nil {
				return
			}
			h.hub.BroadcastMessage(msg)
		}()
	})

	return nil
}

// GET /groups/:group_id/messages
func (h *ChatHandler) GetHistory(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	msgs, err := h.msgRepo.ListByGroup(c.Request().Context(), groupID, 50, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, msgs)
}
