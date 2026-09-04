package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type verificationCodeSenderStub struct {
	phone string
	code  string
	err   error
	calls int
}

func (s *verificationCodeSenderStub) Send(_ context.Context, phoneNumber, code string) error {
	s.calls++
	s.phone = phoneNumber
	s.code = code
	return s.err
}

func TestNewVerificationFlowValidation(t *testing.T) {
	repo := &verificationRepositoryStub{}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{}

	tests := []struct {
		name         string
		verification *VerificationService
		sender       VerificationCodeSender
	}{
		{"nil verification service", nil, sender},
		{"nil sender", verification, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewVerificationFlow(tt.verification, tt.sender); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestVerificationFlowRequestCode(t *testing.T) {
	repo := &verificationRepositoryStub{}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	userID := uuid.New()
	phoneNumber := "+2348012345678"
	if err := flow.RequestCode(context.Background(), userID, phoneNumber); err != nil {
		t.Fatalf("RequestCode() error = %v", err)
	}

	if repo.createCalls != 1 || sender.calls != 1 {
		t.Fatalf("expected one persistence and delivery call, got create=%d send=%d", repo.createCalls, sender.calls)
	}
	if sender.phone != phoneNumber {
		t.Fatalf("sent phone number = %q, want %q", sender.phone, phoneNumber)
	}
	if len(sender.code) != 6 {
		t.Fatalf("sent code length = %d, want 6", len(sender.code))
	}
	if repo.created == nil || repo.created.CodeHash != verification.hashCode(userID, sender.code) {
		t.Fatal("persisted code hash does not match delivered code")
	}
	if repo.invalidateCalls != 0 {
		t.Fatalf("Invalidate calls = %d, want 0", repo.invalidateCalls)
	}
}

func TestVerificationFlowRequestCodeValidation(t *testing.T) {
	repo := &verificationRepositoryStub{}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	tests := []struct {
		name        string
		userID      uuid.UUID
		phoneNumber string
	}{
		{"nil user ID", uuid.Nil, "+2348012345678"},
		{"empty phone number", uuid.New(), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := flow.RequestCode(context.Background(), tt.userID, tt.phoneNumber); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if repo.createCalls != 0 || sender.calls != 0 {
		t.Fatalf("unexpected calls: create=%d send=%d", repo.createCalls, sender.calls)
	}
}

func TestVerificationFlowRequestCodeRepositoryError(t *testing.T) {
	repoErr := errors.New("database unavailable")
	repo := &verificationRepositoryStub{createErr: repoErr}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	err = flow.RequestCode(context.Background(), uuid.New(), "+2348012345678")
	if !errors.Is(err, repoErr) {
		t.Fatalf("RequestCode() error = %v, want %v", err, repoErr)
	}
	if sender.calls != 0 {
		t.Fatalf("Send calls = %d, want 0", sender.calls)
	}
}

func TestVerificationFlowRequestCodeDeliveryErrorInvalidatesCode(t *testing.T) {
	deliveryErr := errors.New("delivery failed")
	repo := &verificationRepositoryStub{}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{err: deliveryErr}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	err = flow.RequestCode(context.Background(), uuid.New(), "+2348012345678")
	if !errors.Is(err, deliveryErr) {
		t.Fatalf("RequestCode() error = %v, want %v", err, deliveryErr)
	}
	if repo.createCalls != 1 || sender.calls != 1 {
		t.Fatalf("unexpected calls: create=%d send=%d", repo.createCalls, sender.calls)
	}
	if repo.invalidateCalls != 1 {
		t.Fatalf("Invalidate calls = %d, want 1", repo.invalidateCalls)
	}
	if repo.invalidatedID == uuid.Nil || repo.created == nil || repo.invalidatedID != repo.created.ID {
		t.Fatalf("invalidated ID = %v, want created ID %v", repo.invalidatedID, repo.created.ID)
	}
}

func TestVerificationFlowRequestCodeDeliveryAndInvalidationErrors(t *testing.T) {
	deliveryErr := errors.New("delivery failed")
	invalidateErr := errors.New("invalidate failed")
	repo := &verificationRepositoryStub{invalidateErr: invalidateErr}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{err: deliveryErr}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	err = flow.RequestCode(context.Background(), uuid.New(), "+2348012345678")
	if !errors.Is(err, deliveryErr) || !errors.Is(err, invalidateErr) {
		t.Fatalf("RequestCode() error = %v, want delivery and invalidation errors", err)
	}
}

func TestVerificationFlowRequestCodeUsesVerificationTTL(t *testing.T) {
	repo := &verificationRepositoryStub{}
	verification := newVerificationServiceForTest(t, repo)
	sender := &verificationCodeSenderStub{}
	flow, err := NewVerificationFlow(verification, sender)
	if err != nil {
		t.Fatalf("NewVerificationFlow() error = %v", err)
	}

	if err := flow.RequestCode(context.Background(), uuid.New(), "+2348012345678"); err != nil {
		t.Fatalf("RequestCode() error = %v", err)
	}
	if repo.created == nil {
		t.Fatal("expected verification code to be persisted")
	}

	wantExpiry := time.Date(2026, time.September, 4, 12, 5, 0, 0, time.UTC)
	if !repo.created.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiry = %v, want %v", repo.created.ExpiresAt, wantExpiry)
	}
}
