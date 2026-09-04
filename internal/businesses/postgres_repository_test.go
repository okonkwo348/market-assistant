package businesses

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestNewPostgresRepository(t *testing.T) {
	t.Run("rejects nil pool", func(t *testing.T) {
		_, err := NewPostgresRepository(nil)
		if err == nil {
			t.Fatal("NewPostgresRepository() error = nil, want error")
		}
	})
}

func TestPostgresRepositoryCreate(t *testing.T) {
	repo := &PostgresRepository{}

	t.Run("rejects nil business", func(t *testing.T) {
		err := repo.Create(context.Background(), nil)
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
	})
}

func TestPostgresRepositoryGetByID(t *testing.T) {
	repo := &PostgresRepository{}

	_, err := repo.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("GetByID() error = nil, want error")
	}
}

func TestPostgresRepositoryListByUserID(t *testing.T) {
	repo := &PostgresRepository{}

	_, err := repo.ListByUserID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("ListByUserID() error = nil, want error")
	}
}
