package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userRepo domain.UserRepository
}

func NewUserHandler(userRepo domain.UserRepository) *UserHandler {
	return &UserHandler{userRepo: userRepo}
}

// GET /api/v1/users/me
func (h *UserHandler) GetMe(c echo.Context) error {
	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}

	user, err := h.userRepo.FindByID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.JSON(http.StatusOK, user)
}

// PUT /api/v1/users/me/currency
func (h *UserHandler) UpdateCurrency(c echo.Context) error {
	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}

	var body struct {
		Currency string `json:"currency"`
	}
	if err := c.Bind(&body); err != nil || body.Currency == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "currency required")
	}

	if !domain.IsSupported(body.Currency) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported currency: "+body.Currency)
	}

	if err := h.userRepo.UpdatePreferredCurrency(c.Request().Context(), userID, body.Currency); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update currency")
	}

	return c.JSON(http.StatusOK, echo.Map{"preferred_currency": body.Currency})
}
