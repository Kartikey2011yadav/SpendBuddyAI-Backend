package http

import (
	"net/http"

	"github.com/kartikeyyadav/spendbuddy/internal/delivery/http/handler"
	mw "github.com/kartikeyyadav/spendbuddy/internal/delivery/http/middleware"
	"github.com/kartikeyyadav/spendbuddy/internal/auth"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewRouter(
	jwtSvc *auth.JWTService,
	authH *handler.AuthHandler,
	chatH *handler.ChatHandler,
	expH  *handler.ExpenseHandler,
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

	// Auth (public)
	a := e.Group("/auth")
	a.POST("/google", authH.GoogleLogin)
	a.POST("/otp/send", authH.SendOTP)
	a.POST("/otp/verify", authH.VerifyOTP)
	a.POST("/refresh", authH.Refresh)

	// Authenticated routes
	api := e.Group("/api/v1", mw.JWTMiddleware(jwtSvc))

	// WebSocket — JWT via query param ?token=
	api.GET("/ws/groups/:group_id", chatH.ServeWS)

	// Chat history
	api.GET("/groups/:group_id/messages", chatH.GetHistory)

	// Expenses
	api.POST("/groups/:group_id/expenses", expH.CreateExpense)
	api.GET("/groups/:group_id/balances", expH.GetBalances)
	api.GET("/groups/:group_id/balances/me", expH.GetMyBalance)

	return e
}
