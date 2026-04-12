package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kartikeyyadav/spendbuddy/internal/domain"
)

type MessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Save(ctx context.Context, msg *domain.Message) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO messages (id, group_id, user_id, content, type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		msg.ID, msg.GroupID, msg.UserID, msg.Content, msg.Type, msg.CreatedAt,
	)
	return err
}

func (r *MessageRepository) ListByGroup(ctx context.Context, groupID uuid.UUID, limit, offset int) ([]*domain.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.id, m.group_id, m.user_id, m.content, m.type, m.created_at,
		       u.display_name, u.avatar_url
		FROM messages m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`,
		groupID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Message
	for rows.Next() {
		var msg domain.Message
		if err := rows.Scan(
			&msg.ID, &msg.GroupID, &msg.UserID, &msg.Content, &msg.Type, &msg.CreatedAt,
			&msg.SenderName, &msg.SenderAvatar,
		); err != nil {
			return nil, err
		}
		out = append(out, &msg)
	}
	return out, rows.Err()
}
