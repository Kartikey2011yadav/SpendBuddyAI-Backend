package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users
		    (id, google_sub, email, display_name, avatar_url, is_email_verified,
		     preferred_currency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		u.ID, u.GoogleSub, u.Email, u.DisplayName, u.AvatarURL,
		u.IsEmailVerified, u.PreferredCurrency, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.scanOne(ctx, `
		SELECT id, google_sub, email, display_name, avatar_url, is_email_verified,
		       preferred_currency, created_at, updated_at
		FROM users WHERE id = $1`, id)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.scanOne(ctx, `
		SELECT id, google_sub, email, display_name, avatar_url, is_email_verified,
		       preferred_currency, created_at, updated_at
		FROM users WHERE email = $1`, email)
}

func (r *UserRepository) FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	return r.scanOne(ctx, `
		SELECT id, google_sub, email, display_name, avatar_url, is_email_verified,
		       preferred_currency, created_at, updated_at
		FROM users WHERE google_sub = $1`, sub)
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET google_sub=$1, display_name=$2, avatar_url=$3,
		    is_email_verified=$4, updated_at=$5
		WHERE id=$6`,
		u.GoogleSub, u.DisplayName, u.AvatarURL, u.IsEmailVerified, u.UpdatedAt, u.ID,
	)
	return err
}

func (r *UserRepository) UpdatePreferredCurrency(ctx context.Context, userID uuid.UUID, currency string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET preferred_currency=$1, updated_at=NOW() WHERE id=$2`,
		currency, userID,
	)
	return err
}

func (r *UserRepository) scanOne(ctx context.Context, query string, args ...interface{}) (*domain.User, error) {
	row := r.db.QueryRow(ctx, query, args...)
	var u domain.User
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.DisplayName, &u.AvatarURL,
		&u.IsEmailVerified, &u.PreferredCurrency, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}
