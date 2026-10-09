package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func Auth(jwtSecret string, userRepo repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			userID, err := service.ValidateJWTToken(strings.TrimPrefix(header, "Bearer "), jwtSecret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			if _, err := userRepo.GetByID(r.Context(), userID); err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
