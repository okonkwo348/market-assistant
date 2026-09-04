package httpapi

import "net/http"

// RequireUser rejects requests that do not have an authenticated user ID in
// the request context. Authentication middleware is responsible for populating
// that context before this middleware runs.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := UserIDFromContext(r.Context()); err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}
