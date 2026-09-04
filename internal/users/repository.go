package users

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error)
}
