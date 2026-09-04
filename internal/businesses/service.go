package businesses

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) (*Service, error) {
	if repo == nil {
		return nil, errors.New("business repository is nil")
	}

	return &Service{repo: repo, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, name string) (*Business, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("business name is required")
	}

	business := &Business{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		CreatedAt: s.now().UTC(),
	}

	if err := s.repo.Create(ctx, business); err != nil {
		return nil, err
	}

	return business, nil
}

func (s *Service) GetByID(ctx context.Context, userID, businessID uuid.UUID) (*Business, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}
	if businessID == uuid.Nil {
		return nil, errors.New("business id is required")
	}

	return s.repo.GetByID(ctx, userID, businessID)
}

func (s *Service) ListByUserID(ctx context.Context, userID uuid.UUID) ([]Business, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	return s.repo.ListByUserID(ctx, userID)
}
