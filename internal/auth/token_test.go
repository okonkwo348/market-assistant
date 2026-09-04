package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"market-assistant/internal/httpapi"
)

const testAuthSecret = "01234567890123456789012345678901"

func newTokenManagerForTest(t *testing.T) *TokenManager {
	t.Helper()

	manager, err := NewTokenManager(testAuthSecret, time.Hour)
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	manager.now = func() time.Time {
		return time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	}
	return manager
}

func TestNewTokenManagerValidation(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		ttl    time.Duration
	}{
		{"short secret", "short", time.Hour},
		{"invalid ttl", testAuthSecret, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewTokenManager(tt.secret, tt.ttl); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestTokenManagerIssueAndVerify(t *testing.T) {
	manager := newTokenManagerForTest(t)
	userID := uuid.New()

	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	got, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got != userID {
		t.Fatalf("Verify() user ID = %v, want %v", got, userID)
	}
}

func TestTokenManagerIssueRejectsNilUserID(t *testing.T) {
	manager := newTokenManagerForTest(t)

	if _, err := manager.Issue(uuid.Nil); err == nil {
		t.Fatal("Issue() expected error for nil user ID")
	}
}

func TestTokenManagerVerifyRejectsMalformedToken(t *testing.T) {
	manager := newTokenManagerForTest(t)

	for _, token := range []string{"", "abc", "abc.def.ghi", ".signature", "payload."} {
		t.Run(token, func(t *testing.T) {
			_, err := manager.Verify(token)
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidToken)
			}
		})
	}
}

func TestTokenManagerVerifyRejectsTamperedToken(t *testing.T) {
	manager := newTokenManagerForTest(t)
	token, err := manager.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	parts := strings.Split(token, ".")
	parts[0] = parts[0] + "x"

	if _, err := manager.Verify(strings.Join(parts, ".")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestTokenManagerVerifyRejectsExpiredToken(t *testing.T) {
	manager := newTokenManagerForTest(t)
	userID := uuid.New()

	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	manager.now = func() time.Time {
		return time.Date(2026, time.September, 4, 13, 0, 0, 0, time.UTC)
	}

	if _, err := manager.Verify(token); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("Verify() error = %v, want %v", err, ErrTokenExpired)
	}
}

func TestTokenManagerMiddlewareRequiresBearerToken(t *testing.T) {
	manager := newTokenManagerForTest(t)

	tests := []struct {
		name   string
		header string
	}{
		{"missing header", ""},
		{"missing scheme", "token"},
		{"wrong scheme", "Basic token"},
		{"missing token", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			})

			manager.Middleware(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if called {
				t.Fatal("next handler was called")
			}
		})
	}
}

func TestTokenManagerMiddlewareSetsUserID(t *testing.T) {
	manager := newTokenManagerForTest(t)
	userID := uuid.New()
	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got, err := httpapi.UserIDFromContext(r.Context())
		if err != nil {
			t.Errorf("UserIDFromContext() error = %v", err)
			return
		}
		if got != userID {
			t.Errorf("context user ID = %v, want %v", got, userID)
		}
	})

	manager.Middleware(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestTokenManagerMiddlewareRejectsExpiredToken(t *testing.T) {
	manager := newTokenManagerForTest(t)
	token, err := manager.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	manager.now = func() time.Time {
		return time.Date(2026, time.September, 4, 13, 0, 0, 0, time.UTC)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	manager.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler was called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTokenManagerMiddlewareAcceptsBearerCaseInsensitively(t *testing.T) {
	manager := newTokenManagerForTest(t)
	token, err := manager.Issue(uuid.New())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bEaReR "+token)
	rec := httptest.NewRecorder()

	called := false
	manager.Middleware(http.HandlerFunc(func(http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestTokenManagerMiddlewarePreservesRequestContext(t *testing.T) {
	manager := newTokenManagerForTest(t)
	userID := uuid.New()
	token, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	originalValue := "kept"
	type contextKey string
	key := contextKey("test")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.WithValue(context.Background(), key, originalValue))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	manager.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(key); got != originalValue {
			t.Errorf("original context value = %v, want %v", got, originalValue)
		}
	})).ServeHTTP(rec, req)
}
