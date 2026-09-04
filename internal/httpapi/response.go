package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

func parseUUIDPathValue(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue(name))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	writeJSON(w, status, value)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func WriteError(w http.ResponseWriter, status int, message string) {
	writeError(w, status, message)
}
