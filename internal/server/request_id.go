package server

import (
	"context"
	"net/http"
	"regexp"

	"github.com/DocHoax/watchdog/internal/audit"
)

type contextKey string

const contextKeyRequestID contextKey = "watchdog_request_id"

var validRequestIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// RequestIDMiddleware extracts or generates a unique correlation request ID for each HTTP request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" || !validRequestIDPattern.MatchString(reqID) {
			reqID = audit.GenerateEventID()
		}

		ctx := context.WithValue(r.Context(), contextKeyRequestID, reqID)
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context, or returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return val
	}
	return ""
}
