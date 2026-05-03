package handler

import (
	"math"
	"net/http"
	"time"

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

// expenseResp is the JSON response for a created expense.
// Amounts are in human units (e.g. 15.00 for USD, 1500 for JPY).
type expenseResp struct {
	ID          uuid.UUID          `json:"id"`
	GroupID     uuid.UUID          `json:"group_id"`
	PayerID     uuid.UUID          `json:"payer_id"`
	Amount      float64            `json:"amount"`
	Currency    string             `json:"currency"`
	Description string             `json:"description"`
	SplitMethod domain.SplitMethod `json:"split_method"`
	CreatedAt   time.Time          `json:"created_at"`
}

type balanceResp struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	NetBalance  float64   `json:"net_balance"`
	Currency    string    `json:"currency"`
}

type debtResp struct {
	FromUserID   uuid.UUID `json:"from_user_id"`
	FromUserName string    `json:"from_user_name"`
	ToUserID     uuid.UUID `json:"to_user_id"`
	ToUserName   string    `json:"to_user_name"`
	Amount       float64   `json:"amount"`
	Currency     string    `json:"currency"`
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

	currency, err := h.grpRepo.GetCurrency(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}
	factor := domain.MinorUnitFactor(currency)

	var body struct {
		Amount      float64            `json:"amount"`
		Description string             `json:"description"`
		SplitMethod domain.SplitMethod `json:"split_method"`
		Splits      map[string]float64 `json:"splits,omitempty"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Convert human-readable units → minor units using the group's currency factor.
	amountMinor := int64(math.Round(body.Amount * float64(factor)))

	splits := make(map[uuid.UUID]int64, len(body.Splits))
	for k, v := range body.Splits {
		uid, err := uuid.Parse(k)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user id in splits: "+k)
		}
		splits[uid] = int64(math.Round(v * float64(factor)))
	}

	exp, err := h.svc.CreateExpense(c.Request().Context(), expense.CreateExpenseInput{
		GroupID:     groupID,
		PayerID:     payerID,
		Amount:      amountMinor,
		Description: body.Description,
		SplitMethod: body.SplitMethod,
		Splits:      splits,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	// Push updated balances to all connected clients.
	go func() {
		balances, _ := h.svc.GetGroupBalances(c.Request().Context(), groupID)
		h.hub.BroadcastBalanceUpdate(groupID, balances)
	}()

	return c.JSON(http.StatusCreated, expenseResp{
		ID:          exp.ID,
		GroupID:     exp.GroupID,
		PayerID:     exp.PayerID,
		Amount:      float64(exp.Amount) / float64(factor),
		Currency:    currency,
		Description: exp.Description,
		SplitMethod: exp.SplitMethod,
		CreatedAt:   exp.CreatedAt,
	})
}

// GET /groups/:group_id/balances
func (h *ExpenseHandler) GetBalances(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	currency, err := h.grpRepo.GetCurrency(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}
	factor := domain.MinorUnitFactor(currency)

	balances, err := h.svc.GetGroupBalances(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	debts := expense.SimplifyDebts(balances)

	bResp := make([]balanceResp, len(balances))
	for i, b := range balances {
		bResp[i] = balanceResp{
			UserID:      b.UserID,
			DisplayName: b.DisplayName,
			NetBalance:  float64(b.NetBalance) / float64(factor),
			Currency:    currency,
		}
	}
	dResp := make([]debtResp, len(debts))
	for i, d := range debts {
		dResp[i] = debtResp{
			FromUserID:   d.FromUserID,
			FromUserName: d.FromUserName,
			ToUserID:     d.ToUserID,
			ToUserName:   d.ToUserName,
			Amount:       float64(d.Amount) / float64(factor),
			Currency:     currency,
		}
	}

	return c.JSON(http.StatusOK, echo.Map{"balances": bResp, "simplified_debts": dResp})
}

// GET /groups/:group_id/balances/me
func (h *ExpenseHandler) GetMyBalance(c echo.Context) error {
	groupID, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}

	currency, err := h.grpRepo.GetCurrency(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "group not found")
	}
	factor := domain.MinorUnitFactor(currency)

	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, _ := uuid.Parse(rawUID)

	netMinor, err := h.svc.GetNetBalance(c.Request().Context(), groupID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{
		"net_balance": float64(netMinor) / float64(factor),
		"currency":    currency,
	})
}
