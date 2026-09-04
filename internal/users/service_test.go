package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockRepository struct {
	createFn            func(context.Context, *User) error
	getByIDFn           func(context.Context, uuid.UUID) (*User, error)
	getByPhoneNumberFn  func(context.Context, string) (*User, error)
	createdUser         *User
	requestedID         uuid.UUID
	requestedPhone      string
}

func (m *mockRepository) Create(ctx context.Context, user *User) error {
	m.createdUser = user
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	m.requestedID = id
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, ErrNotFound
}

func (m *mockRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error) {
	m.requestedPhone = phoneNumber
	if m.getByPhoneNumberFn != nil {
		return m.getByPhoneNumberFn(ctx, phoneNumber)
	}
	return nil, ErrNotFound
}

func TestNewService(t *testing.T) {
	t.Run("rejects nil repository", func(t *testing.T) {
		_, err := NewService(nil)
		if err == nil {
			t.Fatal("NewService() error = nil, want error")
		}
	})
}

func TestServiceCreate(t *testing.T) {
	fixedNow := time.Date(2026, 9, 4, 10, 0, 0, 0, time.FixedZone("WAT", 3600))
	repo := &mockRepository{}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	service.now = func() time.Time { return fixedNow }

	t.Run("creates user with normalized phone number and generated ID", func(t *testing.T) {
		repo.createdUser = nil

		got, err := service.Create(context.Background(), "  +2348000000000  ")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got == nil {
			t.Fatal("Create() returned nil user")
		}
		if got.ID == uuid.Nil {
			t.Fatal("Create() generated nil UUID")
		}
		if got.PhoneNumber != "+2348000000000" {
			t.Fatalf("PhoneNumber = %q, want %q", got.PhoneNumber, "+2348000000000")
		}
		if !got.CreatedAt.Equal(fixedNow.UTC()) {
			t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, fixedNow.UTC())
		}
		if repo.createdUser != got {
			t.Fatal("repository did not receive the created user")
		}
	})

	t.Run("rejects empty phone number", func(t *testing.T) {
		repo.createdUser = nil

		_, err := service.Create(context.Background(), "   ")
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
		if repo.createdUser != nil {
			t.Fatal("repository Create() was called for invalid input")
		}
	})

	t.Run("returns repository error", func(t *testing.T) {
		wantErr := errors.New("database failure")
		repo.createFn = func(context.Context, *User) error { return wantErr }
		t.Cleanup(func() { repo.createFn = nil })

		_, err := service.Create(context.Background(), "+2348222222222")
		if !errors.Is(err, wantErr) {
			t.Fatalf("Create() error = %v, want %v", err, wantErr)
		}
	})
}

func TestServiceGetByID(t *testing.T) {
	id := uuid.New()
	want := &User{ID: id, PhoneNumber: "+2348000000000"}
	repo := &mockRepository{
		getByIDFn: func(_ context.Context, gotID uuid.UUID) (*User, error) {
			if gotID != id {
				return nil, errors.New("unexpected user ID")
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
			t.Fatal("repository was called for invalid user ID")
		}
	})
}

func TestServiceGetByPhoneNumber(t *testing.T) {
	want := &User{ID: uuid.New(), PhoneNumber: "+2348111111111"}
	repo := &mockRepository{
		getByPhoneNumberFn: func(_ context.Context, phoneNumber string) (*User, error) {
			if phoneNumber != want.PhoneNumber {
				return nil, errors.New("unexpected phone number")
			}
			return want, nil
		},
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := service.GetByPhoneNumber(context.Background(), "  +2348111111111 ")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if got != want {
		t.Fatalf("GetByPhoneNumber() = %+v, want %+v", got, want)
	}

	t.Run("rejects empty phone number", func(t *testing.T) {
		_, err := service.GetByPhoneNumber(context.Background(), " ")
		if err == nil {
			t.Fatal("GetByPhoneNumber() error = nil, want error")
		}
		if repo.requestedPhone != want.PhoneNumber {
			t.Fatal("repository was called for invalid phone number")
		}
	})
}
