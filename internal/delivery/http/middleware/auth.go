package middleware

import (
	"net/http"
	"strings"

	"github.com/kartikeyyadav/spendbuddy/internal/auth"
	"github.com/labstack/echo/v4"
)

const UserIDKey = "user_id"
const UserEmailKey = "user_email"

// JWTMiddleware validates the Bearer token and injects claims into context.
func JWTMiddleware(jwtSvc *auth.JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing bearer token")
			}

			token := strings.TrimPrefix(header, "Bearer ")
			claims, err := jwtSvc.ValidateAccessToken(token)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			c.Set(UserIDKey, claims.UserID)
			c.Set(UserEmailKey, claims.Email)
			return next(c)
		}
	}
}
