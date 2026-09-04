package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"market-assistant/internal/httpapi"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid authentication token")
	ErrTokenExpired = errors.New("authentication token expired")
)

type TokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret string, ttl time.Duration) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, errors.New("authentication secret must be at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("authentication token TTL must be greater than zero")
	}

	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

func (m *TokenManager) Issue(userID uuid.UUID) (string, error) {
	if userID == uuid.Nil {
		return "", errors.New("user id is required")
	}

	expiresAt := m.now().UTC().Add(m.ttl).Unix()
	payload := userID.String() + "." + strconv.FormatInt(expiresAt, 10)
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := m.sign(encodedPayload)

	return encodedPayload + "." + signature, nil
}

func (m *TokenManager) Verify(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return uuid.Nil, ErrInvalidToken
	}

	expectedSignature := m.sign(parts[0])
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expectedSignature)) != 1 {
		return uuid.Nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	fields := strings.Split(string(payload), ".")
	if len(fields) != 2 {
		return uuid.Nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(fields[0])
	if err != nil || userID == uuid.Nil {
		return uuid.Nil, ErrInvalidToken
	}

	expiresAt, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	if m.now().UTC().Unix() >= expiresAt {
		return uuid.Nil, ErrTokenExpired
	}

	return userID, nil
}

func (m *TokenManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if header == "" {
			writeAuthError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		userID, err := m.Verify(parts[1])
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		next.ServeHTTP(w, r.WithContext(httpapi.WithUserID(r.Context(), userID)))
	})
}

func (m *TokenManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, message)
}
