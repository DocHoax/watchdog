package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// FuzzParseTimeOrDuration fuzzes timestamp and duration string parsing in the server package.
func FuzzParseTimeOrDuration(f *testing.F) {
	seeds := []string{
		"2026-09-27T12:00:00Z",
		"2026-09-27T12:00:00.123456789Z",
		"2026-09-27",
		"15m",
		"1h",
		"24h",
		"7d",
		"30D",
		"0s",
		"-5m",
		"invalid",
		"999999999999999h",
		"",
		"10000000d",
		"0d",
		"100000d",
		"100001d",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		gotTime, err := parseTimeOrDuration(input)
		if err == nil {
			// If parsing succeeded, ensure the parsed time is not zero
			if gotTime.IsZero() {
				t.Errorf("parseTimeOrDuration returned zero time with nil error for %q", input)
			}
		}
	})
}

// FuzzServerRequestIDValidation fuzzes X-Request-ID validation against malicious or malformed header values.
func FuzzServerRequestIDValidation(f *testing.F) {
	seeds := []string{
		"req-123456",
		"550e8400-e29b-41d4-a716-446655440000",
		"",
		"   ",
		"header\r\ninjection: true",
		"req\x00nullbyte",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 129 chars
		"<script>alert(1)</script>",
		"req; drop table audit_events;",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("X-Request-ID", input)
		w := httptest.NewRecorder()

		handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := GetRequestID(r.Context())
			if reqID == "" {
				t.Errorf("expected non-empty request ID in context")
			}
			if len(reqID) > 128 {
				t.Errorf("request ID length %d exceeds max 128 chars", len(reqID))
			}
			w.WriteHeader(http.StatusOK)
		}))

		// Must never panic
		handler.ServeHTTP(w, req)
		respReqID := w.Header().Get("X-Request-ID")
		if respReqID == "" {
			t.Errorf("expected non-empty X-Request-ID header in response")
		}
	})
}
