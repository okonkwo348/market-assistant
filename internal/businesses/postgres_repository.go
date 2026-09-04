package businesses

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

func (r *PostgresRepository) Create(ctx context.Context, business *Business) error {
	if business == nil {
		return errors.New("business is nil")
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO businesses (id, user_id, name, created_at)
		VALUES ($1, $2, $3, $4)
	`, business.ID, business.UserID, business.Name, business.CreatedAt)
	if err != nil {
		return fmt.Errorf("create business: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, userID, businessID uuid.UUID) (*Business, error) {
	business := new(Business)

	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, created_at
		FROM businesses
		WHERE id = $1 AND user_id = $2
	`, businessID, userID).Scan(
		&business.ID,
		&business.UserID,
		&business.Name,
		&business.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get business by id: %w", err)
	}

	return business, nil
}

func (r *PostgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]Business, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, name, created_at
		FROM businesses
		WHERE user_id = $1
		ORDER BY created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list businesses by user id: %w", err)
	}
	defer rows.Close()

	businesses := make([]Business, 0)
	for rows.Next() {
		var business Business
		if err := rows.Scan(
			&business.ID,
			&business.UserID,
			&business.Name,
			&business.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan business: %w", err)
		}
		businesses = append(businesses, business)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate businesses: %w", err)
	}

	return businesses, nil
}
