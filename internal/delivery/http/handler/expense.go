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

// expenseResp is the JSON response for a created expense (amounts in dollars).
type expenseResp struct {
	ID          uuid.UUID          `json:"id"`
	GroupID     uuid.UUID          `json:"group_id"`
	PayerID     uuid.UUID          `json:"payer_id"`
	Amount      float64            `json:"amount"`
	Description string             `json:"description"`
	SplitMethod domain.SplitMethod `json:"split_method"`
	CreatedAt   time.Time          `json:"created_at"`
}

type balanceResp struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	NetBalance  float64   `json:"net_balance"` // dollars
}

type debtResp struct {
	FromUserID   uuid.UUID `json:"from_user_id"`
	FromUserName string    `json:"from_user_name"`
	ToUserID     uuid.UUID `json:"to_user_id"`
	ToUserName   string    `json:"to_user_name"`
	Amount       float64   `json:"amount"` // dollars
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
		Amount      float64            `json:"amount"`
		Description string             `json:"description"`
		SplitMethod domain.SplitMethod `json:"split_method"`
		Splits      map[string]float64 `json:"splits,omitempty"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Convert dollars → cents at the handler boundary.
	amountCents := int64(math.Round(body.Amount * 100))

	splits := make(map[uuid.UUID]int64, len(body.Splits))
	for k, v := range body.Splits {
		uid, err := uuid.Parse(k)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid user id in splits: "+k)
		}
		splits[uid] = int64(math.Round(v * 100))
	}

	exp, err := h.svc.CreateExpense(c.Request().Context(), expense.CreateExpenseInput{
		GroupID:     groupID,
		PayerID:     payerID,
		Amount:      amountCents,
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
		Amount:      float64(exp.Amount) / 100.0,
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

	balances, err := h.svc.GetGroupBalances(c.Request().Context(), groupID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	debts := expense.SimplifyDebts(balances)

	// Convert cents → dollars for the response.
	bResp := make([]balanceResp, len(balances))
	for i, b := range balances {
		bResp[i] = balanceResp{
			UserID:      b.UserID,
			DisplayName: b.DisplayName,
			NetBalance:  float64(b.NetBalance) / 100.0,
		}
	}
	dResp := make([]debtResp, len(debts))
	for i, d := range debts {
		dResp[i] = debtResp{
			FromUserID:   d.FromUserID,
			FromUserName: d.FromUserName,
			ToUserID:     d.ToUserID,
			ToUserName:   d.ToUserName,
			Amount:       float64(d.Amount) / 100.0,
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

	rawUID, _ := c.Get(middleware.UserIDKey).(string)
	userID, _ := uuid.Parse(rawUID)

	netCents, err := h.svc.GetNetBalance(c.Request().Context(), groupID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"net_balance": float64(netCents) / 100.0})
}
