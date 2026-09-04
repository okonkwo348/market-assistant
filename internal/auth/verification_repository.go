package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerificationCode struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type VerificationRepository interface {
	Create(ctx context.Context, code *VerificationCode) error
	GetLatestActive(ctx context.Context, userID uuid.UUID, now time.Time) (*VerificationCode, error)
	Consume(ctx context.Context, id uuid.UUID, now time.Time) error
	Invalidate(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context, now time.Time) error
}

type PostgresVerificationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresVerificationRepository(pool *pgxpool.Pool) (*PostgresVerificationRepository, error) {
	if pool == nil {
		return nil, errors.New("database pool is nil")
	}
	return &PostgresVerificationRepository{pool: pool}, nil
}

func (r *PostgresVerificationRepository) Create(ctx context.Context, code *VerificationCode) error {
	if code == nil {
		return errors.New("verification code is nil")
	}
	if code.ID == uuid.Nil {
		return errors.New("verification code id is required")
	}
	if code.UserID == uuid.Nil {
		return errors.New("user id is required")
	}
	if code.CodeHash == "" {
		return errors.New("verification code hash is required")
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO verification_codes (id, user_id, code_hash, expires_at, consumed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, code.ID, code.UserID, code.CodeHash, code.ExpiresAt, code.ConsumedAt, code.CreatedAt)
	if err != nil {
		return fmt.Errorf("create verification code: %w", err)
	}
	return nil
}

func (r *PostgresVerificationRepository) GetLatestActive(ctx context.Context, userID uuid.UUID, now time.Time) (*VerificationCode, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	code := new(VerificationCode)
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, code_hash, expires_at, consumed_at, created_at
		FROM verification_codes
		WHERE user_id = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, now).Scan(
		&code.ID,
		&code.UserID,
		&code.CodeHash,
		&code.ExpiresAt,
		&code.ConsumedAt,
		&code.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationCodeNotFound
		}
		return nil, fmt.Errorf("get latest active verification code: %w", err)
	}
	return code, nil
}

func (r *PostgresVerificationRepository) Consume(ctx context.Context, id uuid.UUID, now time.Time) error {
	if id == uuid.Nil {
		return errors.New("verification code id is required")
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE verification_codes
		SET consumed_at = $2
		WHERE id = $1
	  AND consumed_at IS NULL
	  AND expires_at > $2
	`, id, now)
	if err != nil {
		return fmt.Errorf("consume verification code: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrVerificationCodeNotFound
	}
	return nil
}

func (r *PostgresVerificationRepository) Invalidate(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return errors.New("verification code id is required")
	}

	_, err := r.pool.Exec(ctx, `
		UPDATE verification_codes
		SET consumed_at = COALESCE(consumed_at, CURRENT_TIMESTAMP)
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("invalidate verification code: %w", err)
	}
	return nil
}

func (r *PostgresVerificationRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM verification_codes
		WHERE expires_at <= $1
	`, now)
	if err != nil {
		return fmt.Errorf("delete expired verification codes: %w", err)
	}
	return nil
}
