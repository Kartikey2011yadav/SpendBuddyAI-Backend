package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
	"github.com/kartikeyyadav/spendbuddy/pkg/config"
	"github.com/redis/go-redis/v9"
)

const otpKeyPrefix = "otp:"

// OTPService handles email OTP generation, storage (Redis), and verification.
type OTPService struct {
	cfg      config.SMTPConfig
	rdb      *redis.Client
	userRepo domain.UserRepository
	jwt      *JWTService
	mailer   Mailer
}

// Mailer is the interface for sending emails (swap with real SMTP impl).
type Mailer interface {
	SendOTP(ctx context.Context, to, code string) error
}

func NewOTPService(
	cfg config.SMTPConfig,
	rdb *redis.Client,
	userRepo domain.UserRepository,
	jwt *JWTService,
	mailer Mailer,
) *OTPService {
	return &OTPService{cfg: cfg, rdb: rdb, userRepo: userRepo, jwt: jwt, mailer: mailer}
}

// SendEmailOTP generates a 6-digit OTP, stores it in Redis, and emails it.
func (s *OTPService) SendEmailOTP(ctx context.Context, email string) error {
	code, err := generateOTP(6)
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}

	key := otpKeyPrefix + email
	if err := s.rdb.Set(ctx, key, code, s.cfg.OTPTTL).Err(); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}

	return s.mailer.SendOTP(ctx, email, code)
}

// VerifyEmailOTP checks the OTP, upserts the user, and returns a token pair.
func (s *OTPService) VerifyEmailOTP(ctx context.Context, email, code string) (*domain.TokenPair, *domain.User, error) {
	key := otpKeyPrefix + email
	stored, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil, fmt.Errorf("otp expired or not found")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("redis get: %w", err)
	}

	if stored != code {
		return nil, nil, fmt.Errorf("invalid otp")
	}

	// OTP is single-use — delete immediately
	s.rdb.Del(ctx, key)

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// New user via OTP — create them
		user = &domain.User{
			ID:              uuid.New(),
			Email:           email,
			DisplayName:     email, // user can update later
			IsEmailVerified: true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if createErr := s.userRepo.Create(ctx, user); createErr != nil {
			return nil, nil, fmt.Errorf("create user: %w", createErr)
		}
	} else {
		user.IsEmailVerified = true
		user.UpdatedAt = time.Now()
		if updateErr := s.userRepo.Update(ctx, user); updateErr != nil {
			return nil, nil, fmt.Errorf("update user: %w", updateErr)
		}
	}

	pair, err := s.jwt.IssueTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return pair, user, nil
}

func generateOTP(digits int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, n), nil
}
