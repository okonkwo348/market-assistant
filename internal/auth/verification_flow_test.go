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

	if repo.createCalls != 1 {
		t.Fatalf("Create calls = %d, want 1", repo.createCalls)
	}
	if sender.calls != 1 {
		t.Fatalf("Send calls = %d, want 1", sender.calls)
	}
	if sender.phone != phoneNumber {
		t.Fatalf("sent phone number = %q, want %q", sender.phone, phoneNumber)
	}
	if len(sender.code) != 6 {
		t.Fatalf("sent code length = %d, want 6", len(sender.code))
	}
	if repo.created == nil {
		t.Fatal("expected verification code to be persisted")
	}
	if repo.created.CodeHash != verification.hashCode(userID, sender.code) {
		t.Fatal("persisted code hash does not match delivered code")
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
		}
	)}

	if repo.createCalls != 0 {
		t.Fatalf("Create calls = %d, want 0", repo.createCalls)
	}
	if sender.calls != 0 {
		t.Fatalf("Send calls = %d, want 0", sender.calls)
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

func TestVerificationFlowRequestCodeDeliveryError(t *testing.T) {
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
	if repo.createCalls != 1 {
		t.Fatalf("Create calls = %d, want 1", repo.createCalls)
	}
	if sender.calls != 1 {
		t.Fatalf("Send calls = %d, want 1", sender.calls)
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
