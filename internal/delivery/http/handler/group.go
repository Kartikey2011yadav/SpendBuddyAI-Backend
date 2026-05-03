package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
)

type GroupHandler struct {
	groupRepo domain.GroupRepository
	userRepo  domain.UserRepository
}

func NewGroupHandler(groupRepo domain.GroupRepository, userRepo domain.UserRepository) *GroupHandler {
	return &GroupHandler{groupRepo: groupRepo, userRepo: userRepo}
}

// POST /api/v1/groups
func (h *GroupHandler) CreateGroup(c echo.Context) error {
	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	creatorID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}

	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description,omitempty"`
		Currency    string  `json:"currency,omitempty"`
	}
	if err := c.Bind(&body); err != nil || body.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}

	// Resolve currency: use provided value, or fall back to creator's preference.
	currency := body.Currency
	if currency == "" {
		creator, err := h.userRepo.FindByID(c.Request().Context(), creatorID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "could not fetch user")
		}
		currency = creator.PreferredCurrency
		if currency == "" {
			currency = "USD"
		}
	}

	if !domain.IsSupported(currency) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported currency: "+currency)
	}

	group := &domain.Group{
		ID:          uuid.New(),
		Name:        body.Name,
		Description: body.Description,
		CreatedBy:   creatorID,
		Currency:    currency,
		CreatedAt:   time.Now(),
	}

	if err := h.groupRepo.Create(c.Request().Context(), group); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create group")
	}

	// Add creator as admin member.
	member := &domain.GroupMember{
		GroupID:  group.ID,
		UserID:   creatorID,
		Role:     domain.RoleAdmin,
		JoinedAt: time.Now(),
	}
	if err := h.groupRepo.AddMember(c.Request().Context(), member); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add creator as member")
	}

	return c.JSON(http.StatusCreated, group)
}

// GET /api/v1/groups
func (h *GroupHandler) ListGroups(c echo.Context) error {
	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}

	groups, err := h.groupRepo.FindByUserID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"groups": groups})
}
