package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidVerificationCode = errors.New("invalid verification code")
	ErrVerificationCodeExpired = errors.New("verification code expired")
)

type VerificationService struct {
	repo   VerificationRepository
	secret []byte
	ttl    time.Duration
	now    func() time.Time
	rand   func([]byte) (int, error)
}

func NewVerificationService(repo VerificationRepository, secret string, ttl time.Duration) (*VerificationService, error) {
	if repo == nil {
		return nil, errors.New("verification repository is nil")
	}
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, errors.New("verification secret must be at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("verification code TTL must be greater than zero")
	}

	return &VerificationService{
		repo:   repo,
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
		rand:   rand.Read,
	}, nil
}

func (s *VerificationService) Issue(ctx context.Context, userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("user id is required")
	}

	code, err := s.generateCode()
	if err != nil {
		return "", err
	}

	now := s.now().UTC()
	verification := &VerificationCode{
		ID:        uuid.New(),
		UserID:    userID,
		CodeHash:  s.hashCode(userID, code),
		ExpiresAt: now.Add(s.ttl),
		CreatedAt: now,
	}

	if err := s.repo.Create(ctx, verification); err != nil {
		return "", err
	}

	return code, nil
}

func (s *VerificationService) Verify(ctx context.Context, userID uuid.UUID, code string) error {
	if userID == uuid.Nil {
		return errors.New("user id is required")
	}
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return ErrInvalidVerificationCode
	}
	if _, err := strconv.Atoi(code); err != nil {
		return ErrInvalidVerificationCode
	}

	verification, err := s.repo.GetLatestActive(ctx, userID, s.now().UTC())
	if err != nil {
		return err
	}

	expectedHash := s.hashCode(userID, code)
	if subtle.ConstantTimeCompare([]byte(verification.CodeHash), []byte(expectedHash)) != 1 {
		return ErrInvalidVerificationCode
	}

	if err := s.repo.Consume(ctx, verification.ID, s.now().UTC()); err != nil {
		return err
	}

	return nil
}

func (s *VerificationService) generateCode() (string, error) {
	bytes := make([]byte, 4)
	if _, err := s.rand(bytes); err != nil {
		return "", fmt.Errorf("generate verification code: %w", err)
	}

	value := uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func (s *VerificationService) hashCode(userID uuid.UUID, code string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(userID.String()))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
