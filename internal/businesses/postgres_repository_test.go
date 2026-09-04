package businesses

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
	userID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, phone_number)
		VALUES ($1, $2)
	`, userID, "+2348000000000"); err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	t.Run("Create persists business", func(t *testing.T) {
		business := &Business{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "Test Store",
			CreatedAt: time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
		}

		if err := repo.Create(ctx, business); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.GetByID(ctx, userID, business.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if *got != *business {
			t.Fatalf("GetByID() = %+v, want %+v", got, business)
		}
	})

	t.Run("Create rejects nil business", func(t *testing.T) {
		if err := repo.Create(ctx, nil); err == nil {
			t.Fatal("Create() error = nil, want error")
		}
	})

	t.Run("GetByID returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetByID(ctx, userID, uuid.New())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("GetByID prevents cross-user access", func(t *testing.T) {
		business := &Business{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "Private Store",
			CreatedAt: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		}
		if err := repo.Create(ctx, business); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		otherUserID := uuid.New()
		if _, err := pool.Exec(ctx, `
			INSERT INTO users (id, phone_number)
			VALUES ($1, $2)
		`, otherUserID, "+2348222222222"); err != nil {
			t.Fatalf("insert second test user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUserID)
		})

		_, err := repo.GetByID(ctx, otherUserID, business.ID)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetByID() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("ListByUserID returns only matching businesses in order", func(t *testing.T) {
		first := &Business{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "First Store",
			CreatedAt: time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC),
		}
		second := &Business{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      "Second Store",
			CreatedAt: time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC),
		}
		otherUserID := uuid.New()
		other := &Business{
			ID:        uuid.New(),
			UserID:    otherUserID,
			Name:      "Other Store",
			CreatedAt: time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC),
		}

		if _, err := pool.Exec(ctx, `INSERT INTO users (id, phone_number) VALUES ($1, $2)`, otherUserID, "+2348111111111"); err != nil {
			t.Fatalf("insert second test user: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, otherUserID)
		})

		for _, business := range []*Business{first, second, other} {
			if err := repo.Create(ctx, business); err != nil {
				t.Fatalf("Create(%q) error = %v", business.Name, err)
			}
		}

		got, err := repo.ListByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("ListByUserID() error = %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("ListByUserID() returned %d businesses, want 4", len(got))
		}
		if got[0].Name != "First Store" || got[1].Name != "Test Store" || got[2].Name != "Private Store" || got[3].Name != "Second Store" {
			t.Fatalf("ListByUserID() order = [%s, %s, %s, %s], want [First Store, Test Store, Private Store, Second Store]", got[0].Name, got[1].Name, got[2].Name, got[3].Name)
		}
		for _, business := range got {
			if business.UserID != userID {
				t.Fatalf("ListByUserID() returned business for user %s, want %s", business.UserID, userID)
			}
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
