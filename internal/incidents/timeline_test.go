package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestTimelineBuilder_OrderingAndTieBreaking(t *testing.T) {
	baseTime := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	tb := NewTimelineBuilder("inc-timeline-1")

	// Add entries out of chronological order and with duplicate timestamps to test tie-breaking
	tb.AddEntry(IncidentTimelineEntry{
		ID:          "tl-later",
		Timestamp:   baseTime.Add(10 * time.Minute),
		EventType:   TimelineEventAlertResolved,
		Source:      "AlertEngine",
		Severity:    model.SeverityInfo,
		Title:       "Alert resolved",
		Description: "CPU normalized",
	})

	tb.AddEntry(IncidentTimelineEntry{
		ID:          "tl-tie-warning",
		Timestamp:   baseTime,
		EventType:   TimelineEventAnomalyDetected,
		Source:      "AnomalyEngine",
		Severity:    model.SeverityWarning,
		Title:       "Anomaly Warning",
		Description: "Memory deviation",
	})

	tb.AddEntry(IncidentTimelineEntry{
		ID:          "tl-tie-critical",
		Timestamp:   baseTime,
		EventType:   TimelineEventAlertFired,
		Source:      "AlertEngine",
		Severity:    model.SeverityCritical,
		Title:       "Alert Critical",
		Description: "High CPU Alert",
	})

	tb.AddEntry(IncidentTimelineEntry{
		ID:          "tl-earlier",
		Timestamp:   baseTime.Add(-5 * time.Minute),
		EventType:   TimelineEventSignalDetected,
		Source:      "Heartbeat",
		Severity:    model.SeverityInfo,
		Title:       "Signal Detected",
		Description: "Metric drift",
	})

	// Add a signal via AddSignal
	tb.AddSignal(IncidentSignal{
		ID:          "sig-1",
		Type:        SignalTypeDiagnostic,
		Source:      "DiagMem",
		NodeID:      "node-1",
		Severity:    model.SeverityCritical,
		Timestamp:   baseTime.Add(2 * time.Minute),
		Description: "Memory diagnostic failed",
		Value:       92.5,
		Threshold:   85.0,
	})

	entries := tb.Build()

	if len(entries) != 5 {
		t.Fatalf("expected 5 timeline entries, got %d", len(entries))
	}

	// 1. Earliest entry: baseTime - 5m
	if entries[0].ID != "tl-earlier" {
		t.Errorf("expected first entry to be tl-earlier, got %s", entries[0].ID)
	}

	// 2 & 3. Tie-breaking at baseTime: Critical must come before Warning
	if entries[1].ID != "tl-tie-critical" {
		t.Errorf("expected second entry to be tl-tie-critical (due to severity tie-break), got %s", entries[1].ID)
	}
	if entries[2].ID != "tl-tie-warning" {
		t.Errorf("expected third entry to be tl-tie-warning, got %s", entries[2].ID)
	}

	// 4. Signal entry at baseTime + 2m
	if entries[3].EventType != TimelineEventDiagnosticFailed {
		t.Errorf("expected fourth entry event type TimelineEventDiagnosticFailed, got %v", entries[3].EventType)
	}
	if entries[3].Payload["value"] != 92.5 {
		t.Errorf("expected payload value 92.5, got %v", entries[3].Payload["value"])
	}

	// 5. Latest entry: baseTime + 10m
	if entries[4].ID != "tl-later" {
		t.Errorf("expected fifth entry to be tl-later, got %s", entries[4].ID)
	}
}

func TestFilterTimeline(t *testing.T) {
	baseTime := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	entries := []IncidentTimelineEntry{
		{
			ID:        "tl-1",
			NodeID:    "node-1",
			Severity:  model.SeverityInfo,
			Timestamp: baseTime,
		},
		{
			ID:        "tl-2",
			NodeID:    "node-1",
			Severity:  model.SeverityCritical,
			Timestamp: baseTime.Add(1 * time.Minute),
		},
		{
			ID:        "tl-3",
			NodeID:    "node-2",
			Severity:  model.SeverityWarning,
			Timestamp: baseTime.Add(2 * time.Minute),
		},
	}

	t.Run("Filter by NodeID", func(t *testing.T) {
		filtered := FilterTimeline(entries, "node-1", "", time.Time{}, time.Time{})
		if len(filtered) != 2 {
			t.Errorf("expected 2 entries for node-1, got %d", len(filtered))
		}
	})

	t.Run("Filter by MinSeverity Warning", func(t *testing.T) {
		filtered := FilterTimeline(entries, "", model.SeverityWarning, time.Time{}, time.Time{})
		if len(filtered) != 2 { // tl-2 (Critical) and tl-3 (Warning)
			t.Errorf("expected 2 entries with min severity warning, got %d", len(filtered))
		}
	})

	t.Run("Filter by Time Range", func(t *testing.T) {
		filtered := FilterTimeline(entries, "", "", baseTime.Add(30*time.Second), baseTime.Add(90*time.Second))
		if len(filtered) != 1 || filtered[0].ID != "tl-2" {
			t.Errorf("expected tl-2 within time window, got %+v", filtered)
		}
	})
}
