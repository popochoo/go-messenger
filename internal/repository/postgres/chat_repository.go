package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/popochoo/go-messenger/internal/domain"
)

type ChatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{pool: pool}
}

func (r *ChatRepository) GetOrCreateDirect(ctx context.Context, user1ID, user2ID int64) (*domain.Chat, bool, error) {
	minID, maxID := user1ID, user2ID
	if minID > maxID {
		minID, maxID = maxID, minID
	}

	directKey := fmt.Sprintf("%d_%d", minID, maxID)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var chat domain.Chat
	created := false

	insertQuery := `
		INSERT INTO chats (type, direct_key)
		VALUES ('direct', $1)
		ON CONFLICT (direct_key) DO NOTHING
		RETURNING id, type, title, created_at;
	`

	err = tx.QueryRow(ctx, insertQuery, directKey).Scan(
		&chat.ID,
		&chat.Type,
		&chat.Title,
		&chat.CreatedAt,
	)
	if err == nil {
		created = true

		memberQuery := `
			INSERT INTO chat_members (chat_id, user_id, role)
			VALUES ($1, $2, 'member'), ($1, $3, 'member');
		`
		if _, err := tx.Exec(ctx, memberQuery, chat.ID, minID, maxID); err != nil {
			return nil, false, fmt.Errorf("insert direct members: %w", err)
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		selectQuery := `
			SELECT id, type, title, created_at
			FROM chats
			WHERE direct_key = $1;
		`
		err = tx.QueryRow(ctx, selectQuery, directKey).Scan(
			&chat.ID,
			&chat.Type,
			&chat.Title,
			&chat.CreatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("select existing direct chat: %w", err)
		}
	} else {
		return nil, false, fmt.Errorf("insert direct chat: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit tx: %w", err)
	}

	chat.Role = domain.RoleMember

	return &chat, created, nil
}

func (r *ChatRepository) CreateGroup(ctx context.Context, creatorID int64, title string, memberIDs []int64) (*domain.Chat, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var chat domain.Chat
	chatQuery := `
		INSERT INTO chats (type, title)
		VALUES ('group', $1)
		RETURNING id, type, title, created_at;
	`
	err = tx.QueryRow(ctx, chatQuery, title).Scan(
		&chat.ID,
		&chat.Type,
		&chat.Title,
		&chat.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert group chat: %w", err)
	}

	insertAdminQuery := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, 'admin');
		`
	if _, err := tx.Exec(ctx, insertAdminQuery, chat.ID, creatorID); err != nil {
		return nil, fmt.Errorf("insert group admin: %w", err)
	}

	for _, memberID := range memberIDs {
		if memberID == creatorID {
			continue
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO chat_members (chat_id, user_id, role)
			VALUES ($1, $2, 'member')
			ON CONFLICT (chat_id, user_id) DO NOTHING;
		`, chat.ID, memberID)
		if err != nil {
			return nil, fmt.Errorf("insert group member %d:%w", memberID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	chat.Role = domain.RoleAdmin

	return &chat, nil
}

func (r *ChatRepository) GetUsersChats(ctx context.Context, userID int64) ([]domain.Chat, error) {
	query := `
		SELECT c.id, c.type, c.title, cm.role, c.created_at
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		WHERE cm.user_id = $1
		ORDER BY c.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user chats: %w", err)
	}
	defer rows.Close()

	chats := make([]domain.Chat, 0)
	for rows.Next() {
		var chat domain.Chat
		err = rows.Scan(
			&chat.ID,
			&chat.Type,
			&chat.Title,
			&chat.Role,
			&chat.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}

		chats = append(chats, chat)
	}

	return chats, rows.Err()
}

func (r *ChatRepository) GetChatAndMemberRole(ctx context.Context, chatID, userID int64) (domain.ChatType, domain.MemberRole, error) {
	query := `
		SELECT c.type, cm.role
		FROM chats c
		LEFT JOIN chat_members cm ON c.id = cm.chat_id AND cm.user_id = $2
		WHERE c.id = $1;
	`

	var chatType domain.ChatType
	var role *domain.MemberRole

	err := r.pool.QueryRow(ctx, query, chatID, userID).Scan(
		&chatType,
		&role,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", domain.ErrChatNotFound
		}

		return "", "", fmt.Errorf("get chat and role: %w", err)
	}

	if role == nil {
		return chatType, "", domain.ErrChatNotFound
	}

	return chatType, *role, nil
}

func (r *ChatRepository) AddMember(ctx context.Context, chatID, userID int64) error {
	query := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, 'member')
		ON CONFLICT (chat_id, user_id) DO NOTHING;
	`
	_, err := r.pool.Exec(ctx, query, chatID, userID)

	return err
}

func (r *ChatRepository) RemoveMember(ctx context.Context, chatID, userID int64) error {
	query := `
		DELETE FROM chat_members
		WHERE chat_id = $1 AND user_id = $2;
	`
	_, err := r.pool.Exec(ctx, query, chatID, userID)

	return err
}
