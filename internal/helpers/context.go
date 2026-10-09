package helpers

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/middleware"
)

func UserIDFromContext(r *http.Request) string {
	return r.Context().Value(middleware.UserIDKey).(string)
}
