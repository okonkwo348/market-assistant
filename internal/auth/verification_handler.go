package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"market-assistant/internal/httpapi"
	"market-assistant/internal/users"
)

type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
}

type VerificationHandler struct {
	users            *users.Service
	verificationFlow *VerificationFlow
	verification     *VerificationService
	tokens           TokenIssuer
}

func NewVerificationHandler(
	userService *users.Service,
	verificationFlow *VerificationFlow,
	verificationService *VerificationService,
	tokenIssuer TokenIssuer,
) (*VerificationHandler, error) {
	if userService == nil {
		return nil, errors.New("user service is nil")
	}
	if verificationFlow == nil {
		return nil, errors.New("verification flow is nil")
	}
	if verificationService == nil {
		return nil, errors.New("verification service is nil")
	}
	if tokenIssuer == nil {
		return nil, errors.New("token issuer is nil")
	}

	return &VerificationHandler{
		users:            userService,
		verificationFlow: verificationFlow,
		verification:     verificationService,
		tokens:           tokenIssuer,
	}, nil
}

type verificationRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type verifyCodeRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func (h *VerificationHandler) RequestCode(w http.ResponseWriter, r *http.Request) {
	var request verificationRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	phoneNumber := strings.TrimSpace(request.PhoneNumber)
	if phoneNumber == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "phone number is required")
		return
	}

	user, err := h.users.GetByPhoneNumber(r.Context(), phoneNumber)
	if errors.Is(err, users.ErrNotFound) {
		user, err = h.users.Create(r.Context(), phoneNumber)
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to prepare verification")
		return
	}

	if err := h.verificationFlow.RequestCode(r.Context(), user.ID, user.PhoneNumber); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	httpapi.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "verification code sent"})
}

func (h *VerificationHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var request verifyCodeRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	phoneNumber := strings.TrimSpace(request.PhoneNumber)
	if phoneNumber == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "phone number is required")
		return
	}

	user, err := h.users.GetByPhoneNumber(r.Context(), phoneNumber)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid or expired verification code")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.verification.Verify(r.Context(), user.ID, request.Code); err != nil {
		switch {
		case errors.Is(err, ErrInvalidVerificationCode),
			errors.Is(err, ErrVerificationCodeNotFound),
			errors.Is(err, ErrVerificationCodeExpired):
			httpapi.WriteError(w, http.StatusBadRequest, "invalid or expired verification code")
		default:
			httpapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	token, err := h.tokens.Issue(user.ID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to issue authentication token")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{
		"status": "verified",
		"token":  token,
	})
}
