package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/kartikeyyadav/spendbuddy/pkg/config"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type JWTService struct {
	cfg config.JWTConfig
}

func NewJWTService(cfg config.JWTConfig) *JWTService {
	return &JWTService{cfg: cfg}
}

func (s *JWTService) IssueTokenPair(user *domain.User) (*domain.TokenPair, error) {
	access, err := s.issue(user, s.cfg.AccessSecret, s.cfg.AccessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refresh, err := s.issue(user, s.cfg.RefreshSecret, s.cfg.RefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (s *JWTService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return s.validate(tokenStr, s.cfg.AccessSecret)
}

func (s *JWTService) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return s.validate(tokenStr, s.cfg.RefreshSecret)
}

func (s *JWTService) issue(user *domain.User, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: user.ID.String(),
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func (s *JWTService) validate(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}
