package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/DocHoax/watchdog/internal/audit"
	"github.com/DocHoax/watchdog/internal/logger"
	"github.com/DocHoax/watchdog/pkg/model"
)

const (
	// DefaultMaxRequestBodySize is 1MB (1,048,576 bytes).
	DefaultMaxRequestBodySize int64 = 1 << 20
)

// PanicRecoveryMiddleware intercepts runtime panics in downstream HTTP handlers,
// safely logs the stack trace, optionally records a critical audit event,
// and returns a structured JSON 500 Internal Server Error response without crashing the daemon.
func PanicRecoveryMiddleware(next http.Handler, auditLog audit.AuditLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				reqID := GetRequestID(r.Context())

				logger.Errorf("[PANIC RECOVERED] Request %s %s (Request-ID: %s): %v\nStack trace:\n%s",
					r.Method, r.URL.Path, reqID, rec, stack)

				if auditLog != nil {
					_ = auditLog.Record(r.Context(), model.AuditEvent{
						ID:        audit.GenerateEventID(),
						Timestamp: time.Now().UTC(),
						EventType: model.EventServerStartFailure,
						Severity:  model.AuditSeverityCritical,
						Outcome:   model.AuditOutcomeFailure,
						Actor: model.AuditActor{
							Type:     model.ActorTypeSystem,
							Identity: "watchdog-daemon",
						},
						Source: model.AuditSource{
							Address:   r.RemoteAddr,
							Endpoint:  r.URL.Path,
							Method:    r.Method,
							RequestID: reqID,
							UserAgent: r.UserAgent(),
						},
						Message: fmt.Sprintf("Internal HTTP server panic recovered: %v", rec),
					})
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Internal server error",
				})
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// MaxBodySizeMiddleware limits the maximum readable size of incoming HTTP request bodies
// to prevent memory exhaustion and Denial-of-Service attacks.
func MaxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxRequestBodySize
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLoggerMiddleware logs incoming HTTP requests with latency, status, and Request ID.
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := GetRequestID(r.Context())

		// Wrap ResponseWriter to capture status code
		wrapped := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		logger.Debugf("[%s] %s %s -> %d (%s)", reqID, r.Method, r.URL.Path, wrapped.statusCode, duration)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.written {
		r.statusCode = statusCode
		r.written = true
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}
