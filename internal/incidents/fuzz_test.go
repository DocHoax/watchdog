package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func FuzzCalculateSimilarity(f *testing.F) {
	f.Add("symptom1", "symptom2", "node-1", "node-2", "cpu", "memory")
	f.Add("High CPU", "High CPU", "srv-1", "srv-1", "cpu", "cpu")
	f.Add("", "", "", "", "", "")

	f.Fuzz(func(t *testing.T, s1, s2, n1, n2, sub1, sub2 string) {
		incA := Incident{
			ID:              "inc-a",
			Severity:        model.SeverityCritical,
			PrimarySymptoms: []string{s1},
			AffectedNodes:   []string{n1},
			Impact: ImpactScope{
				Subsystems: []string{sub1},
			},
		}
		incB := Incident{
			ID:              "inc-b",
			Severity:        model.SeverityWarning,
			PrimarySymptoms: []string{s2},
			AffectedNodes:   []string{n2},
			Impact: ImpactScope{
				Subsystems: []string{sub2},
			},
		}

		score, breakdown, expl := CalculateSimilarity(incA, incB)
		if score < 0.0 || score > 1.0 {
			t.Errorf("similarity score out of bounds [0, 1]: %.4f", score)
		}
		if breakdown.SymptomSimilarity < 0.0 || breakdown.SymptomSimilarity > 1.0 {
			t.Errorf("symptom similarity out of bounds: %.4f", breakdown.SymptomSimilarity)
		}
		if breakdown.NodeSimilarity < 0.0 || breakdown.NodeSimilarity > 1.0 {
			t.Errorf("node similarity out of bounds: %.4f", breakdown.NodeSimilarity)
		}
		if breakdown.SubsystemSimilarity < 0.0 || breakdown.SubsystemSimilarity > 1.0 {
			t.Errorf("subsystem similarity out of bounds: %.4f", breakdown.SubsystemSimilarity)
		}
		if breakdown.SeveritySimilarity < 0.0 || breakdown.SeveritySimilarity > 1.0 {
			t.Errorf("severity similarity out of bounds: %.4f", breakdown.SeveritySimilarity)
		}
		if expl == "" {
			t.Errorf("explanation should never be empty")
		}
	})
}

func FuzzTimelineOrdering(f *testing.F) {
	f.Add(int64(100), int64(200), int64(300))
	f.Add(int64(500), int64(500), int64(100))
	f.Add(int64(0), int64(0), int64(0))

	f.Fuzz(func(t *testing.T, offset1, offset2, offset3 int64) {
		base := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
		tb := NewTimelineBuilder("inc-fuzz")

		tb.AddEntry(IncidentTimelineEntry{
			ID:        "entry-1",
			Timestamp: base.Add(time.Duration(offset1) * time.Second),
			Severity:  model.SeverityCritical,
		})
		tb.AddEntry(IncidentTimelineEntry{
			ID:        "entry-2",
			Timestamp: base.Add(time.Duration(offset2) * time.Second),
			Severity:  model.SeverityWarning,
		})
		tb.AddEntry(IncidentTimelineEntry{
			ID:        "entry-3",
			Timestamp: base.Add(time.Duration(offset3) * time.Second),
			Severity:  model.SeverityInfo,
		})

		entries := tb.Build()
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}

		for i := 1; i < len(entries); i++ {
			prev := entries[i-1]
			curr := entries[i]
			if curr.Timestamp.Before(prev.Timestamp) {
				t.Errorf("entries out of chronological order: [%d] %v before [%d] %v", i, curr.Timestamp, i-1, prev.Timestamp)
			}
		}
	})
}
