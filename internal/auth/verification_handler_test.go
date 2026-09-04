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
func (s *verificationHandlerUserRepositoryStub) GetByID(ctx context.Context, id uuid.UUID) (*users.User, error) {
	return s.user, s.err
}
func (s *verificationHandlerUserRepositoryStub) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*users.User, error) {
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
	tokenManager, err := NewTokenManager(strings.Repeat("b", 32), time.Hour)
	if err != nil {
		t.Fatalf("create token manager: %v", err)
	}
	handler, err := NewVerificationHandler(userService, verificationFlow, verificationService, tokenManager)
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
		tokens       *TokenManager
		wantErr      string
	}{
		{"nil user service", nil, &VerificationFlow{}, &VerificationService{}, &TokenManager{}, "user service is nil"},
		{"nil flow", userService, nil, &VerificationService{}, &TokenManager{}, "verification flow is nil"},
		{"nil verification service", userService, &VerificationFlow{}, nil, &TokenManager{}, "verification service is nil"},
		{"nil token manager", userService, &VerificationFlow{}, &VerificationService{}, nil, "token manager is nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewVerificationHandler(tt.userService, tt.flow, tt.verification, tt.tokens)
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

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", strings.NewReader(`{"phone_number":"+2348012345678"}`))
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

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", strings.NewReader(`{"phone_number":"+2348012345678"}`))
	recorder := httptest.NewRecorder()
	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeBadRequest(t *testing.T) {
	handler := newVerificationHandler(t, uuid.New(), &verificationRepositoryStub{}, &verificationSenderStub{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"code":`))
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

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"phone_number":"+2348012345678","code":"000000"}`))
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

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"phone_number":"+2348012345678","code":"123456"}`))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
}

func TestVerificationHandlerRequestCodeRequiresPhoneNumber(t *testing.T) {
	handler := newVerificationHandler(t, uuid.New(), &verificationRepositoryStub{}, &verificationSenderStub{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/request", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	handler.RequestCode(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestVerificationHandlerVerifyCodeIssuesToken(t *testing.T) {
	userID := uuid.New()
	phoneNumber := "+2348012345678"
	repo := &verificationRepositoryStub{}
	handler := newVerificationHandler(t, userID, repo, &verificationSenderStub{})

	verificationService := handler.verification
	code := "123456"
	repo.active = &VerificationCode{
		ID:        uuid.New(),
		UserID:    userID,
		CodeHash:  verificationService.hashCode(userID, code),
		ExpiresAt: verificationService.now().Add(time.Minute),
		CreatedAt: verificationService.now(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verification/verify", strings.NewReader(`{"phone_number":"`+phoneNumber+`","code":"123456"}`))
	recorder := httptest.NewRecorder()
	handler.VerifyCode(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"token"`) {
		t.Fatal("expected authentication token in response")
	}
}
