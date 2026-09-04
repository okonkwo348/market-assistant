package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"market-assistant/internal/httpapi"
	"market-assistant/internal/users"

	"github.com/google/uuid"
)

type verificationHandlerUserRepositoryStub struct {
	user *users.User
	err  error
}

func (s *verificationHandlerUserRepositoryStub) Create(context.Context, *users.User) error { return nil }
func (s *verificationHandlerUserRepositoryStub) GetByID(context.Context, uuid.UUID) (*users.User, error) {
	return s.user, s.err
}
func (s *verificationHandlerUserRepositoryStub) GetByPhoneNumber(context.Context, string) (*users.User, error) {
	return s.user, s.err
}

func newVerificationHandler(t *testing.T, userID uuid.UUID, repo *verificationRepositoryStub, sender VerificationCodeSender) *VerificationHandler {
	t.Helper()

	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{
		user: &users.User{ID: userID, PhoneNumber: "+2348012345678"},
	})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}

	verificationService, err := NewVerificationService(repo, strings.Repeat("a", 32), 5*time.Minute)
	if err != nil {
		t.Fatalf("create verification service: %v", err)
	}
	verificationFlow, err := NewVerificationFlow(verificationService, sender)
	if err != nil {
		t.Fatalf("create verification flow: %v", err)
	}
	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService)
	if err != nil {
		t.Fatalf("create verification handler: %v", err)
	}
	return handler
}

func TestNewVerificationHandlerValidation(t *testing.T) {
	userService, err := users.NewService(&verificationHandlerUserRepositoryStub{})
	if err != nil {
		t.Fatalf("create user service: %v", err)
	}

	tests := []struct {
		name         string
		userService  *users.Service
		flow         *VerificationFlow
		verification *VerificationService
		wantErr      string
	}{
		{"nil user service", nil, &VerificationFlow{}, &VerificationService{}, "user service is nil"},
		{"nil flow", userService, nil, &VerificationService{}, "verification flow is nil"},
		{"nil verification service", userService, &VerificationFlow{}, nil, "verification service is nil"},
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
	repo := &verificationRepositoryStub{}
	sender := &verificationSenderStub{}
	handler := newVerificationHandler(t, userID, repo, sender)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", nil)
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()

	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, recorder.Code)
	}
	if !sender.called {
		t.Fatal("expected verification sender to be called")
	}
}

func TestVerificationHandlerRequestCodeUnauthorized(t *testing.T) {
	handler := newVerificationHandler(t, uuid.New(), &verificationRepositoryStub{}, &verificationSenderStub{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", nil)
	recorder := httptest.NewRecorder()
	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeBadRequest(t *testing.T) {
	userID := uuid.New()
	handler := newVerificationHandler(t, userID, &verificationRepositoryStub{}, &verificationSenderStub{})

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
	repo := &verificationRepositoryStub{}
	handler := newVerificationHandler(t, userID, repo, &verificationSenderStub{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"code":"000000"}`))
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeRepositoryError(t *testing.T) {
	userID := uuid.New()
	repo := &verificationRepositoryStub{err: errors.New("database unavailable")}
	handler := newVerificationHandler(t, userID, repo, &verificationSenderStub{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"code":"123456"}`))
	req = req.WithContext(httpapi.WithUserID(req.Context(), userID))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}
