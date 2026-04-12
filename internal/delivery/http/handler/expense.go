package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/kartikeyyadav/spendbuddy/internal/expense"
	"github.com/labstack/echo/v4"
)

type ExpenseHandler struct {
	svc     *expense.Service
	grpRepo domain.GroupRepository
	hub     interface {
		BroadcastBalanceUpdate(groupID uuid.UUID, balances []*domain.UserBalance)
	}
}

func NewExpenseHandler(svc *expense.Service, grpRepo domain.GroupRepository, hub interface {
	BroadcastBalanceUpdate(groupID uuid.UUID, balances []*domain.UserBalance)
}) *ExpenseHandler {
	return &ExpenseHandler{svc: svc, grpRepo: grpRepo, hub: hub}
}

// POST /groups/:group_id/expenses
func (h *ExpenseHandler) CreateExpense(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	payerID, err := uuid.Parse(rawUID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user")
	}

	var body struct {
		Amount      float64                `json:"amount"`
		Description string                 `json:"description"`
		SplitMethod domain.SplitMethod     `json:"split_method"`
		Splits      map[string]float64     `json:"splits,omitempty"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	splits := make(map[uuid.UUID]float64, len(body.Splits))
	for k, v := range body.Splits {
		uid, err := uuid.Parse(k)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user id in splits: "+k)
		}
		splits[uid] = v
	}

	exp, err := h.svc.CreateExpense(c.Request().Context(), expense.CreateExpenseInput{
		GroupID:     groupID,
		PayerID:     payerID,
		Amount:      body.Amount,
		Description: body.Description,
		SplitMethod: body.SplitMethod,
		Splits:      splits,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	// Push updated balances to all connected clients
	go func() {
		balances, _ := h.svc.GetGroupBalances(c.Request().Context(), groupID)
		h.hub.BroadcastBalanceUpdate(groupID, balances)
	}()

	return c.JSON(http.StatusCreated, exp)
}

// GET /groups/:group_id/balances
func (h *ExpenseHandler) GetBalances(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	balances, err := h.svc.GetGroupBalances(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	debts := expense.SimplifyDebts(balances)
	return c.JSON(http.StatusOK, echo.Map{"balances": balances, "simplified_debts": debts})
}

// GET /groups/:group_id/balances/me
func (h *ExpenseHandler) GetMyBalance(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, _ := uuid.Parse(rawUID)

	net, err := h.svc.GetNetBalance(c.Request().Context(), groupID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"net_balance": net})
}
