package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/auth"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	google *auth.GoogleAuthService
	otp    *auth.OTPService
	jwt    *auth.JWTService
}

func NewAuthHandler(google *auth.GoogleAuthService, otp *auth.OTPService, jwt *auth.JWTService) *AuthHandler {
	return &AuthHandler{google: google, otp: otp, jwt: jwt}
}

// POST /auth/google
func (h *AuthHandler) GoogleLogin(c echo.Context) error {
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := c.Bind(&body); err != nil || body.IDToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id_token required")
	}

	pair, user, err := h.google.LoginWithGoogle(c.Request().Context(), body.IDToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"tokens": pair, "user": user})
}

// POST /auth/otp/send
func (h *AuthHandler) SendOTP(c echo.Context) error {
	var body struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&body); err != nil || body.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email required")
	}

	if err := h.otp.SendEmailOTP(c.Request().Context(), body.Email); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to send otp")
	}

	return c.JSON(http.StatusOK, echo.Map{"message": "OTP sent"})
}

// POST /auth/otp/verify
func (h *AuthHandler) VerifyOTP(c echo.Context) error {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.Bind(&body); err != nil || body.Email == "" || body.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email and code required")
	}

	pair, user, err := h.otp.VerifyEmailOTP(c.Request().Context(), body.Email, body.Code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	return c.JSON(http.StatusOK, echo.Map{"tokens": pair, "user": user})
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&body); err != nil || body.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refresh_token required")
	}

	claims, err := h.jwt.ValidateRefreshToken(body.RefreshToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	id, _ := uuid.Parse(claims.UserID)
	stub := &domain.User{ID: id, Email: claims.Email}

	pair, err := h.jwt.IssueTokenPair(stub)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "token issuance failed")
	}

	return c.JSON(http.StatusOK, pair)
}
