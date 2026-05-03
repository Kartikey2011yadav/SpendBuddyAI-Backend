package http

import (
	"net/http"

	"github.com/kartikeyyadav/spendbuddy/internal/auth"
	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/handler"
	mw "github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewRouter(
	jwtSvc *auth.JWTService,
	authH  *handler.AuthHandler,
	chatH  *handler.ChatHandler,
	expH   *handler.ExpenseHandler,
	userH  *handler.UserHandler,
	groupH *handler.GroupHandler,
) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})

	// Public: supported currencies list
	e.GET("/api/v1/currencies", func(c echo.Context) error {
		list := make([]domain.Currency, 0, len(domain.Currencies))
		for _, cur := range domain.Currencies {
			list = append(list, cur)
		}
		return c.JSON(http.StatusOK, echo.Map{"currencies": list})
	})

	// Auth (public)
	a := e.Group("/auth")
	a.POST("/google", authH.GoogleLogin)
	a.POST("/otp/send", authH.SendOTP)
	a.POST("/otp/verify", authH.VerifyOTP)
	a.POST("/refresh", authH.Refresh)

	// Authenticated routes
	api := e.Group("/api/v1", mw.JWTMiddleware(jwtSvc))

	// User profile & preferences
	api.GET("/users/me", userH.GetMe)
	api.PUT("/users/me/currency", userH.UpdateCurrency)

	// Groups CRUD
	api.POST("/groups", groupH.CreateGroup)
	api.GET("/groups", groupH.ListGroups)
	api.GET("/groups/:group_id", groupH.GetGroup)
	api.PATCH("/groups/:group_id", groupH.UpdateGroup)
	api.DELETE("/groups/:group_id", groupH.DeleteGroup)

	// Group members
	api.GET("/groups/:group_id/members", groupH.ListMembers)
	api.POST("/groups/:group_id/members", groupH.AddMember)
	api.DELETE("/groups/:group_id/members/me", groupH.LeaveGroup)
	api.DELETE("/groups/:group_id/members/:user_id", groupH.RemoveMember)
	api.PATCH("/groups/:group_id/members/:user_id", groupH.UpdateMemberRole)

	// WebSocket — JWT via query param ?token=
	api.GET("/ws/groups/:group_id", chatH.ServeWS)

	// Chat history
	api.GET("/groups/:group_id/messages", chatH.GetHistory)

	// Expenses & balances
	api.POST("/groups/:group_id/expenses", expH.CreateExpense)
	api.GET("/groups/:group_id/balances", expH.GetBalances)
	api.GET("/groups/:group_id/balances/me", expH.GetMyBalance)

	return e
}
