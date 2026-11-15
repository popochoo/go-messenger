package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/popochoo/go-messenger/internal/domain"
)

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

func (r *MessageRepository) Create(ctx context.Context, chatID, senderID int64, content string) (*domain.Message, error) {
	query := `
		INSERT INTO messages (chat_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, chat_id, sender_id, content, reply_to_id, is_edited, is_deleted, created_at;
	`

	var message domain.Message
	err := r.pool.QueryRow(ctx, query, chatID, senderID, content).Scan(
		&message.ID,
		&message.ChatID,
		&message.SenderID,
		&message.Content,
		&message.ReplyToID,
		&message.IsEdited,
		&message.IsDeleted,
		&message.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	return &message, nil
}

func (r *MessageRepository) GetChatMemberIDs(ctx context.Context, chatID int64) ([]int64, error) {
	query := `
		SELECT user_id FROM chat_members
		WHERE chat_id = $1;
	`
	rows, err := r.pool.Query(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("get chat member ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id error: %w", err)
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
}

func (r *MessageRepository) GetHistory(ctx context.Context, chatID, cursor int64, limit int) (*domain.MessagesPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	var query string
	var args []any

	if cursor > 0 {
		query = `
			SELECT id, chat_id, sender_id, content, reply_to_id, is_edited, is_deleted, created_at
			FROM messages
			WHERE chat_id = $1 AND id < $2 AND is_deleted = FALSE
			ORDER BY id DESC
			LIMIT $3;
		`
		args = []any{chatID, cursor, limit + 1}
	} else {
		query = `
			SELECT id, chat_id, sender_id, content, reply_to_id, is_edited, is_deleted, created_at
			FROM messages
			WHERE chat_id = $1 AND is_deleted = FALSE
			ORDER BY id DESC
			LIMIT $2;
		`
		args = []any{chatID, limit + 1}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get message history: %w", err)
	}
	defer rows.Close()

	messages := make([]domain.Message, 0, limit)
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.SenderID,
			&message.Content,
			&message.ReplyToID,
			&message.IsEdited,
			&message.IsDeleted,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message error: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows message error: %w", err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	var nextCursor int64
	if len(messages) > 0 {
		nextCursor = messages[len(messages)-1].ID
	}

	return &domain.MessagesPage{
		Messages:   messages,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (r *MessageRepository) Search(ctx context.Context, chatID int64, searchQuery string, limit int) ([]domain.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	query := `
		SELECT id, chat_id, sender_id, content, reply_to_id, is_edited, is_deleted, created_at
		FROM messages
		WHERE chat_id = $1 AND is_deleted = FALSE AND content ILIKE '%' || $2 || '%'
		ORDER BY id DESC
		LIMIT $3;
	`
	rows, err := r.pool.Query(ctx, query, chatID, searchQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	messages := make([]domain.Message, 0)
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(
			&message.ID,
			&message.ChatID,
			&message.SenderID,
			&message.Content,
			&message.ReplyToID,
			&message.IsEdited,
			&message.IsDeleted,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message error: %w", err)
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}
