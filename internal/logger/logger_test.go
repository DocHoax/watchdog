package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerText(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, false, true)
	l.Info("testing info message", "key", "val")

	out := buf.String()
	if !strings.Contains(out, "[INFO]") || !strings.Contains(out, "testing info message") || !strings.Contains(out, "key=val") {
		t.Fatalf("Unexpected log output: %s", out)
	}
}

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug, true, true)
	l.Warn("warning json", "module", "storage")

	out := buf.String()
	if !strings.Contains(out, `"level":"WARN"`) || !strings.Contains(out, `"msg":"warning json"`) || !strings.Contains(out, `"module":"storage"`) {
		t.Fatalf("Unexpected json log output: %s", out)
	}
}

func TestLoggerFilter(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelWarn, false, true)
	l.Debug("debug should be skipped")
	l.Info("info should be skipped")
	l.Error("error should be printed")

	out := buf.String()
	if strings.Contains(out, "debug should be skipped") || strings.Contains(out, "info should be skipped") {
		t.Fatalf("Filtered log appeared in output: %s", out)
	}
	if !strings.Contains(out, "error should be printed") {
		t.Fatalf("Error log missing: %s", out)
	}
}
