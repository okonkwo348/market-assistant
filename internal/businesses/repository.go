package businesses

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, business *Business) error
	GetByID(ctx context.Context, userID, businessID uuid.UUID) (*Business, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]Business, error)
}
