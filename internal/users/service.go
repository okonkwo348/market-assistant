package users

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
		return nil, errors.New("user repository is nil")
	}

	return &Service{
		repo: repo,
		now:  time.Now,
	}, nil
}

func (s *Service) Create(ctx context.Context, phoneNumber string) (*User, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if phoneNumber == "" {
		return nil, errors.New("phone number is required")
	}

	user := &User{
		ID:          uuid.New(),
		PhoneNumber: phoneNumber,
		CreatedAt:   s.now().UTC(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if id == uuid.Nil {
		return nil, errors.New("user id is required")
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*User, error) {
	phoneNumber = strings.TrimSpace(phoneNumber)
	if phoneNumber == "" {
		return nil, errors.New("phone number is required")
	}

	return s.repo.GetByPhoneNumber(ctx, phoneNumber)
}
