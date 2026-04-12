package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/kartikeyyadav/spendbuddy/pkg/config"
	"google.golang.org/api/idtoken"
)

// GoogleAuthService verifies Google ID tokens and upserts users.
type GoogleAuthService struct {
	cfg      config.GoogleConfig
	userRepo domain.UserRepository
	jwt      *JWTService
}

func NewGoogleAuthService(
	cfg config.GoogleConfig,
	userRepo domain.UserRepository,
	jwt *JWTService,
) *GoogleAuthService {
	return &GoogleAuthService{cfg: cfg, userRepo: userRepo, jwt: jwt}
}

// LoginWithGoogle validates a Google ID token, upserts the user, and returns a token pair.
func (s *GoogleAuthService) LoginWithGoogle(ctx context.Context, idToken string) (*domain.TokenPair, *domain.User, error) {
	payload, err := idtoken.Validate(ctx, idToken, s.cfg.ClientID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid google id token: %w", err)
	}

	claims := payload.Claims
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	picture, _ := claims["picture"].(string)
	emailVerified, _ := claims["email_verified"].(bool)

	if sub == "" || email == "" {
		return nil, nil, fmt.Errorf("google token missing required claims")
	}

	// Try to find existing user by Google sub
	user, err := s.userRepo.FindByGoogleSub(ctx, sub)
	if err != nil {
		// User not found — try by email (may have registered with OTP before)
		user, err = s.userRepo.FindByEmail(ctx, email)
		if err != nil {
			// New user entirely — create them
			user = &domain.User{
				ID:              uuid.New(),
				GoogleSub:       &sub,
				Email:           email,
				DisplayName:     name,
				IsEmailVerified: emailVerified,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			if picture != "" {
				user.AvatarURL = &picture
			}
			if createErr := s.userRepo.Create(ctx, user); createErr != nil {
				return nil, nil, fmt.Errorf("create user: %w", createErr)
			}
		} else {
			// Existing email user — link Google sub
			user.GoogleSub = &sub
			user.IsEmailVerified = emailVerified
			if picture != "" {
				user.AvatarURL = &picture
			}
			user.UpdatedAt = time.Now()
			if updateErr := s.userRepo.Update(ctx, user); updateErr != nil {
				return nil, nil, fmt.Errorf("update user: %w", updateErr)
			}
		}
	}

	pair, err := s.jwt.IssueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return pair, user, nil
}
