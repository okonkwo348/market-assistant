package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"market-assistant/internal/auth"
	"market-assistant/internal/users"
)

type VerificationHandler struct {
	users            *users.Service
	verificationFlow *auth.VerificationFlow
	verification     *auth.VerificationService
}

func NewVerificationHandler(
	userService *users.Service,
	verificationFlow *auth.VerificationFlow,
	verificationService *auth.VerificationService,
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

	return &VerificationHandler{
		users:            userService,
		verificationFlow: verificationFlow,
		verification:     verificationService,
	}, nil
}

type verifyCodeRequest struct {
	Code string `json:"code"`
}

func (h *VerificationHandler) RequestCode(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.verificationFlow.RequestCode(r.Context(), user.ID, user.PhoneNumber); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "verification code sent"})
}

func (h *VerificationHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var request verifyCodeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.verification.Verify(r.Context(), userID, request.Code); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidVerificationCode),
			errors.Is(err, auth.ErrVerificationCodeNotFound),
			errors.Is(err, auth.ErrVerificationCodeExpired):
			writeError(w, http.StatusBadRequest, "invalid or expired verification code")
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}
