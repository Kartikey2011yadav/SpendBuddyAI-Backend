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
		INSERT INTO users (id, google_sub, email, display_name, avatar_url, is_email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, u.GoogleSub, u.Email, u.DisplayName, u.AvatarURL, u.IsEmailVerified, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE id = $1`, id)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE email = $1`, email)
}

func (r *UserRepository) FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	return r.scanOne(ctx, `SELECT * FROM users WHERE google_sub = $1`, sub)
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET google_sub=$1, display_name=$2, avatar_url=$3,
		                 is_email_verified=$4, updated_at=$5
		WHERE id=$6`,
		u.GoogleSub, u.DisplayName, u.AvatarURL, u.IsEmailVerified, u.UpdatedAt, u.ID,
	)
	return err
}

func (r *UserRepository) scanOne(ctx context.Context, query string, args ...interface{}) (*domain.User, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[domain.User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return u, nil
}
