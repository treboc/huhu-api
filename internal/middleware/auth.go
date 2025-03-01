package middleware

import (
	"net/http"
)

var Unauthorized = "Unauthorized"

func AdminAuth(apiKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Admin-API-Key")
			if key == "" {
				http.Error(w, Unauthorized, http.StatusUnauthorized)
				return
			}

			if key != apiKey {
				http.Error(w, Unauthorized, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
