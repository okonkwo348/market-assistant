package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type verificationRepositoryStub struct {
	created       *VerificationCode
	active        *VerificationCode
	createErr     error
	getErr        error
	consumeErr    error
	consumeID     uuid.UUID
	consumeAt     time.Time
	createCalls   int
	getCalls      int
	consumeCalls  int
}

func (r *verificationRepositoryStub) Create(_ context.Context, code *VerificationCode) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	copy := *code
	r.created = &copy
	return nil
}

func (r *verificationRepositoryStub) GetLatestActive(_ context.Context, _ uuid.UUID, _ time.Time) (*VerificationCode, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.active == nil {
		return nil, ErrVerificationCodeNotFound
	}
	copy := *r.active
	return &copy, nil
}

func (r *verificationRepositoryStub) Consume(_ context.Context, id uuid.UUID, now time.Time) error {
	r.consumeCalls++
	r.consumeID = id
	r.consumeAt = now
	return r.consumeErr
}

func (r *verificationRepositoryStub) DeleteExpired(_ context.Context, _ time.Time) error {
	return nil
}

func newVerificationServiceForTest(t *testing.T, repo VerificationRepository) *VerificationService {
	t.Helper()

	svc, err := NewVerificationService(repo, "01234567890123456789012345678901", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewVerificationService() error = %v", err)
	}

	svc.now = func() time.Time {
		return time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	}
	return svc
}

func TestNewVerificationServiceValidation(t *testing.T) {
	validRepo := &verificationRepositoryStub{}

	tests := []struct {
		name   string
		repo   VerificationRepository
		secret string
		ttl    time.Duration
	}{
		{"nil repository", nil, "01234567890123456789012345678901", time.Minute},
		{"short secret", validRepo, "short", time.Minute},
		{"invalid ttl", validRepo, "01234567890123456789012345678901", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewVerificationService(tt.repo, tt.secret, tt.ttl); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestVerificationServiceIssue(t *testing.T) {
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)
	userID := uuid.New()

	code, err := svc.Issue(context.Background(), userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			t.Fatalf("code contains non-digit character %q", char)
		}
	}
	if repo.created == nil {
		t.Fatal("expected verification code to be persisted")
	}
	if repo.created.UserID != userID {
		t.Fatalf("stored user ID = %v, want %v", repo.created.UserID, userID)
	}
	if repo.created.CodeHash == "" {
		t.Fatal("expected code hash to be stored")
	}
	if repo.created.CodeHash == code {
		t.Fatal("verification code must not be stored in plaintext")
	}
	if !repo.created.ExpiresAt.Equal(repo.created.CreatedAt.Add(5 * time.Minute)) {
		t.Fatal("verification code expiry does not match configured TTL")
	}
}

func TestVerificationServiceIssueRepositoryError(t *testing.T) {
	repoErr := errors.New("database unavailable")
	repo := &verificationRepositoryStub{createErr: repoErr}
	svc := newVerificationServiceForTest(t, repo)

	_, err := svc.Issue(context.Background(), uuid.New())
	if !errors.Is(err, repoErr) {
		t.Fatalf("Issue() error = %v, want %v", err, repoErr)
	}
}

func TestVerificationServiceVerifySuccess(t *testing.T) {
	userID := uuid.New()
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)

	code := "123456"
	repo.active = &VerificationCode{
		ID:        uuid.New(),
		UserID:    userID,
		CodeHash:  svc.hashCode(userID, code),
		ExpiresAt: svc.now().Add(time.Minute),
		CreatedAt: svc.now().Add(-time.Minute),
	}

	if err := svc.Verify(context.Background(), userID, code); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if repo.getCalls != 1 {
		t.Fatalf("GetLatestActive calls = %d, want 1", repo.getCalls)
	}
	if repo.consumeCalls != 1 {
		t.Fatalf("Consume calls = %d, want 1", repo.consumeCalls)
	}
	if repo.consumeID != repo.active.ID {
		t.Fatalf("consumed ID = %v, want %v", repo.consumeID, repo.active.ID)
	}
}

func TestVerificationServiceVerifyRejectsInvalidCode(t *testing.T) {
	userID := uuid.New()
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)

	repo.active = &VerificationCode{
		ID:        uuid.New(),
		UserID:    userID,
		CodeHash:  svc.hashCode(userID, "123456"),
		ExpiresAt: svc.now().Add(time.Minute),
		CreatedAt: svc.now(),
	}

	err := svc.Verify(context.Background(), userID, "654321")
	if !errors.Is(err, ErrInvalidVerificationCode) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidVerificationCode)
	}
	if repo.consumeCalls != 0 {
		t.Fatalf("Consume calls = %d, want 0", repo.consumeCalls)
	}
}

func TestVerificationServiceVerifyRejectsMalformedCode(t *testing.T) {
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)

	for _, code := range []string{"", "12345", "1234567", "12a456", "abcdef"} {
		t.Run(code, func(t *testing.T) {
			err := svc.Verify(context.Background(), uuid.New(), code)
			if !errors.Is(err, ErrInvalidVerificationCode) {
				t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidVerificationCode)
			}
			if repo.getCalls != 0 {
				t.Fatalf("GetLatestActive calls = %d, want 0", repo.getCalls)
			}
		})
	}
}

func TestVerificationServiceVerifyNotFound(t *testing.T) {
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)

	err := svc.Verify(context.Background(), uuid.New(), "123456")
	if !errors.Is(err, ErrVerificationCodeNotFound) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrVerificationCodeNotFound)
	}
	if repo.consumeCalls != 0 {
		t.Fatalf("Consume calls = %d, want 0", repo.consumeCalls)
	}
}

func TestVerificationServiceVerifyConsumeError(t *testing.T) {
	userID := uuid.New()
	consumeErr := errors.New("consume failed")
	repo := &verificationRepositoryStub{consumeErr: consumeErr}
	svc := newVerificationServiceForTest(t, repo)
	code := "123456"
	repo.active = &VerificationCode{
		ID:        uuid.New(),
		UserID:    userID,
		CodeHash:  svc.hashCode(userID, code),
		ExpiresAt: svc.now().Add(time.Minute),
		CreatedAt: svc.now(),
	}

	err := svc.Verify(context.Background(), userID, code)
	if !errors.Is(err, consumeErr) {
		t.Fatalf("Verify() error = %v, want %v", err, consumeErr)
	}
}

func TestVerificationServiceRejectsNilUserID(t *testing.T) {
	repo := &verificationRepositoryStub{}
	svc := newVerificationServiceForTest(t, repo)

	if _, err := svc.Issue(context.Background(), uuid.Nil); err == nil {
		t.Fatal("Issue() expected error for nil user ID")
	}
	if err := svc.Verify(context.Background(), uuid.Nil, "123456"); err == nil {
		t.Fatal("Verify() expected error for nil user ID")
	}
}
