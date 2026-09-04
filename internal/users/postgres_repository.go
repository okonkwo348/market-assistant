package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("database pool is nil")
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Create(ctx context.Context, user *User) error {
	if user == nil {
		return errors.New("user is nil")
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, phone_number, created_at)
		VALUES ($1, $2, $3)
	`, user.ID, user.PhoneNumber, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := new(User)

	err := r.pool.QueryRow(ctx, `
		SELECT id, phone_number, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID,
		&user.PhoneNumber,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}

func (r *PostgresRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error) {
	user := new(User)

	err := r.pool.QueryRow(ctx, `
		SELECT id, phone_number, created_at
		FROM users
		WHERE phone_number = $1
	`, phoneNumber).Scan(
		&user.ID,
		&user.PhoneNumber,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by phone number: %w", err)
	}

	return user, nil
}
