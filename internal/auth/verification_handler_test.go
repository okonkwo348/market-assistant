package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"market-assistant/internal/httpapi"
	"market-assistant/internal/users"

	"github.com/google/uuid"
)

type verificationHandlerUserRepositoryStub struct {
	user *users.User
	err  error
}

func (s *verificationHandlerUserRepositoryStub) Create(context.Context, *users.User) error {
	return nil
}

func (s *verificationHandlerUserRepositoryStub) GetByID(context.Context, uuid.UUID) (*users.User, error) {
	return s.user, s.err
}

func (s *verificationHandlerUserRepositoryStub) GetByPhoneNumber(context.Context, string) (*users.User, error) {
	return s.user, s.err
}

type verificationHandlerVerificationFlowStub struct {
	called bool
	err    error
}

func (s *verificationHandlerVerificationFlowStub) RequestCode(context.Context, uuid.UUID, string) error {
	s.called = true
	return s.err
}

type verificationHandlerVerificationStub struct {
	called bool
	err    error
}

func (s *verificationHandlerVerificationStub) Verify(context.Context, uuid.UUID, string) error {
	s.called = true
	return s.err
}

func TestNewVerificationHandlerValidation(t *testing.T) {
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}

	verificationFlow := &VerificationFlow{}
	verificationService := &VerificationService{}

	tests := []struct {
		name          string
		userService   *users.Service
		flow          *VerificationFlow
		verification  *VerificationService
		wantErr       string
	}{
		{"nil user service", nil, verificationFlow, verificationService, "user service is nil"},
		{"nil flow", userService, nil, verificationService, "verification flow is nil"},
		{"nil verification service", userService, verificationFlow, nil, "verification service is nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVerificationHandler(tt.userService, tt.flow, tt.verification)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestVerificationHandlerRequestCode(t *testing.T) {
	userID := uuid.New()
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{
		user: &users.User{ID: userID, PhoneNumber: "+2348012345678"},
	})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}

	flow := &verificationHandlerVerificationFlowStub{}
	verification := &verificationHandlerVerificationStub{}

	// The handler depends on concrete flow/service types, so this test verifies
	// the HTTP behavior through the real collaborators and a stub repository.
	_ = flow
	_ = verification

	verificationRepo := &verificationRepositoryStub{}
	verificationService, err := NewVerificationService(verificationRepo, strings.Repeat("a", 32), 5*60*1e9)
	if err != nil {
		t.Fatalf("create verification service: %v", err)
	}
	verificationSender := &verificationSenderStub{}
	verificationFlow, err := NewVerificationFlow(verificationService, verificationSender)
	if err != nil {
		t.Fatalf("create verification flow: %v", err)
	}

	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", nil)
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()

	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if !verificationSender.called {
		t.Fatal("expected verification sender to be called")
	}
}

func TestVerificationHandlerRequestCodeUnauthorized(t *testing.T) {
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}
	verificationRepo := &verificationRepositoryStub{}
	verificationService, err := NewVerificationService(verificationRepo, strings.Repeat("a", 32), 5*60*1e9)
	if err != nil {
		t.Fatalf("create verification service: %v", err)
	}
	verificationFlow, err := NewVerificationFlow(verificationService, &verificationSenderStub{})
	if err != nil {
		t.Fatalf("create verification flow: %v", err)
	}
	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", nil)
	recorder := httptest.NewRecorder()
	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeBadRequest(t *testing.T) {
	userID := uuid.New()
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{
		user: &users.User{ID: userID, PhoneNumber: "+2348012345678"},
	})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}
	verificationRepo := &verificationRepositoryStub{}
	verificationService, err := NewVerificationService(verificationRepo, strings.Repeat("a", 32), 5*60*1e9)
	if err != nil {
		t.Fatalf("create verification service: %v", err)
	}
	verificationFlow, err := NewVerificationFlow(verificationService, &verificationSenderStub{})
	if err != nil {
		t.Fatalf("create verification flow: %v", err)
	}
	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"code":`))
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeInvalid(t *testing.T) {
	userID := uuid.New()
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{
		user: &users.User{ID: userID, PhoneNumber: "+2348012345678"},
	})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}

	repo := &verificationRepositoryStub{}
	verificationService, err := NewVerificationService(repo, strings.Repeat("a", 32), 5*60*1e9)
	if err != nil {
		t.Fatalf("create verification service: %v", err)
	}
	verificationFlow, err := NewVerificationFlow(verificationService, &verificationSenderStub{})
	if err != nil {
		t.Fatalf("create verification flow: %v", err)
	}
	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"code":"000000"}`))
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if !errors.Is(repo.err, nil) {
		t.Fatalf("unexpected repository error: %v", repo.err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}
