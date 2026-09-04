package businesses

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRepository struct {
	createFn          func(context.Context, *Business) error
	getByIDFn         func(context.Context, uuid.UUID) (*Business, error)
	listByUserIDFn    func(context.Context, uuid.UUID) ([]Business, error)
	createdBusiness   *Business
	requestedID       uuid.UUID
	requestedUserID   uuid.UUID
}

func (m *mockRepository) Create(ctx context.Context, business *Business) error {
	m.createdBusiness = business
	if m.createFn != nil {
		return m.createFn(ctx, business)
	}
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (*Business, error) {
	m.requestedID = id
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (m *mockRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]Business, error) {
	m.requestedUserID = userID
	if m.listByUserIDFn != nil {
		return m.listByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func TestNewService(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Fatal("NewService() error = nil, want error")
	}
}

func TestServiceCreate(t *testing.T) {
	userID := uuid.New()
	fixedNow := time.Date(2026, 9, 4, 10, 0, 0, 0, time.FixedZone("WAT", 3600))
	repo := &mockRepository{}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return fixedNow }

	t.Run("creates business with normalized name and generated ID", func(t *testing.T) {
		repo.createdBusiness = nil
		got, err := service.Create(context.Background(), userID, "  My Store  ")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got == nil || got.ID == uuid.Nil {
			t.Fatal("Create() did not generate a business ID")
		}
		if got.UserID != userID {
			t.Fatalf("UserID = %v, want %v", got.UserID, userID)
		}
		if got.Name != "My Store" {
			t.Fatalf("Name = %q, want %q", got.Name, "My Store")
		}
		if !got.CreatedAt.Equal(fixedNow.UTC()) {
			t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, fixedNow.UTC())
		}
		if repo.createdBusiness != got {
			t.Fatal("repository did not receive the created business")
		}
	})

	t.Run("rejects nil user ID", func(t *testing.T) {
		repo.createdBusiness = nil
		_, err := service.Create(context.Background(), uuid.Nil, "Store")
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
		if repo.createdBusiness != nil {
			t.Fatal("repository was called for invalid user ID")
		}
	})

	t.Run("rejects empty business name", func(t *testing.T) {
		repo.createdBusiness = nil
		_, err := service.Create(context.Background(), userID, "   ")
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
		if repo.createdBusiness != nil {
			t.Fatal("repository was called for invalid business name")
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		wantErr := errors.New("database failure")
		repo.createFn = func(context.Context, *Business) error { return wantErr }
		t.Cleanup(func() { repo.createFn = nil })

		_, err := service.Create(context.Background(), userID, "Store")
		if !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
	})
}

func TestServiceGetByID(t *testing.T) {
	id := uuid.New()
	want := &Business{ID: id, UserID: uuid.New(), Name: "Store"}
	repo := &mockRepository{
		getByIDFn: func(_ context.Context, gotID uuid.UUID) (*Business, error) {
			if gotID != id {
				return nil, errors.New("unexpected business ID")
			}
			return want, nil
		},
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := service.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got != want {
		t.Fatalf("GetByID() = %+v, want %+v", got, want)
	}

	t.Run("rejects nil UUID", func(t *testing.T) {
		_, err := service.GetByID(context.Background(), uuid.Nil)
		if err == nil {
			t.Fatal("GetByID() error = nil, want error")
		}
		if repo.requestedID != id {
			t.Fatal("repository was called for invalid business ID")
		}
	})
}

func TestServiceListByUserID(t *testing.T) {
	userID := uuid.New()
	want := []Business{{ID: uuid.New(), UserID: userID, Name: "Store"}}
	repo := &mockRepository{
		listByUserIDFn: func(_ context.Context, gotUserID uuid.UUID) ([]Business, error) {
			if gotUserID != userID {
				return nil, errors.New("unexpected user ID")
			}
			return want, nil
		},
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := service.ListByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("ListByUserID() = %+v, want %+v", got, want)
	}

	t.Run("rejects nil UUID", func(t *testing.T) {
		_, err := service.ListByUserID(context.Background(), uuid.Nil)
		if err == nil {
			t.Fatal("ListByUserID() error = nil, want error")
		}
		if repo.requestedUserID != userID {
			t.Fatal("repository was called for invalid user ID")
		}
	})
}
