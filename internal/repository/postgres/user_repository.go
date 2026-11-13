package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/popochoo/go-messenger/internal/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (*domain.User, error) {
	query := `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, password_hash, created_at;
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, username, passwordHash).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrUserAlreadyExists
		}

		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1;
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}

		return nil, fmt.Errorf("get user by username %w", err)
	}

	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE id = $1;
	`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}

		return nil, fmt.Errorf("get user by id %w", err)
	}

	return &user, nil
}
