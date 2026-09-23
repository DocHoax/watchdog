package util

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.00 KiB"},
		{1024 * 1024, "1.00 MiB"},
		{1024 * 1024 * 1024 * 4, "4.00 GiB"},
		{1024 * 1024 * 1024 * 1024 * 2, "2.00 TiB"},
	}

	for _, tc := range tests {
		res := FormatBytes(tc.input)
		if res != tc.expected {
			t.Errorf("FormatBytes(%d) = %s, expected %s", tc.input, res, tc.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	d := 25*time.Hour + 30*time.Minute + 15*time.Second
	formatted := FormatDuration(d)
	if !strings.Contains(formatted, "1d") || !strings.Contains(formatted, "1h") || !strings.Contains(formatted, "30m") {
		t.Errorf("Unexpected FormatDuration: %s", formatted)
	}
}

func TestRenderProgressBar(t *testing.T) {
	bar := RenderProgressBar(50.0, 10)
	if !strings.Contains(bar, "50.0%") {
		t.Errorf("Expected 50.0%% in progress bar: %s", bar)
	}
}

func TestTableWriter(t *testing.T) {
	var buf bytes.Buffer
	table := NewTableWriter("NAME", "VALUE")
	table.Append("CPU", "12.5%")
	table.Append("Memory", "64.0%")
	table.Render(&buf)

	output := buf.String()
	if !strings.Contains(output, "NAME") || !strings.Contains(output, "CPU") || !strings.Contains(output, "12.5%") {
		t.Errorf("Unexpected table output: %s", output)
	}
}
