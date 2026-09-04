package httpapi

import (
	"errors"
	"net/http"

	"market-assistant/internal/businesses"
)

type Handler struct {
	businesses *businesses.Service
}

func NewHandler(businessService *businesses.Service) (*Handler, error) {
	if businessService == nil {
		return nil, errors.New("business service is nil")
	}
	return &Handler{businesses: businessService}, nil
}

func (h *Handler) GetBusiness(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	businessID, err := parseUUIDPathValue(r, "businessID")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid business id")
		return
	}

	business, err := h.businesses.GetByID(r.Context(), userID, businessID)
	if err != nil {
		if errors.Is(err, businesses.ErrNotFound) {
			writeError(w, http.StatusNotFound, "business not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, business)
}

func (h *Handler) ListBusinesses(w http.ResponseWriter, r *http.Request) {
	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	businessesList, err := h.businesses.ListByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, businessesList)
}
