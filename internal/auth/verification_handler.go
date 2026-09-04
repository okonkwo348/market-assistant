package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"market-assistant/internal/httpapi"
	"market-assistant/internal/users"
)

type VerificationHandler struct {
	users            *users.Service
	verificationFlow *VerificationFlow
	verification     *VerificationService
}

func NewVerificationHandler(
	userService *users.Service,
	verificationFlow *VerificationFlow,
	verificationService *VerificationService,
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
	userID, err := httpapi.UserIDFromContext(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpapi.WriteError(w, http.StatusUnauthorized, "user not found")
			return
		}
		httpapi.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.verificationFlow.RequestCode(r.Context(), user.ID, user.PhoneNumber); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	httpapi.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "verification code sent"})
}

func (h *VerificationHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	userID, err := httpapi.UserIDFromContext(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var request verifyCodeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.verification.Verify(r.Context(), userID, request.Code); err != nil {
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

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}
