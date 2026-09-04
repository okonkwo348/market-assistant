package users

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"market-assistant/internal/db"
)

func TestNewPostgresRepository(t *testing.T) {
	t.Run("rejects nil pool", func(t *testing.T) {
		_, err := NewPostgresRepository(nil)
		if err == nil {
			t.Fatal("NewPostgresRepository() error = nil, want error")
		}
	})
}

func TestPostgresRepository(t *testing.T) {
	pool := newTestPool(t)
	repo, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}

	ctx := context.Background()

	t.Run("Create persists user", func(t *testing.T) {
		user := &User{
			ID:          uuid.New(),
			PhoneNumber: "+2348000000000",
			CreatedAt:   time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
		})

		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if *got != *user {
			t.Fatalf("GetByID() = %+v, want %+v", got, user)
		}
	})

	t.Run("Create rejects nil user", func(t *testing.T) {
		if err := repo.Create(ctx, nil); err == nil {
			t.Fatal("Create() error = nil, want error")
		}
	})

	t.Run("GetByID returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("GetByPhoneNumber returns user", func(t *testing.T) {
		user := &User{
			ID:          uuid.New(),
			PhoneNumber: "+2348111111111",
			CreatedAt:   time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC),
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
		})

		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByPhoneNumber(ctx, user.PhoneNumber)
		if err != nil {
			t.Fatalf("GetByPhoneNumber() error = %v", err)
		}
		if *got != *user {
			t.Fatalf("GetByPhoneNumber() = %+v, want %+v", got, user)
		}
	})

	t.Run("GetByPhoneNumber returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetByPhoneNumber(ctx, "+2348999999999")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByPhoneNumber() error = %v, want ErrNotFound", err)
		}
	})
}

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create test pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.Ping(ctx, pool); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return pool
}
