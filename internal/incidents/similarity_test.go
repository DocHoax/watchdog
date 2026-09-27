package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestCalculateSimilarity(t *testing.T) {
	now := time.Now().UTC()

	incBase := Incident{
		ID:              "inc-base",
		Severity:        model.SeverityCritical,
		PrimarySymptoms: []string{"High CPU load on cores", "Disk write saturation"},
		AffectedNodes:   []string{"node-1", "node-2"},
		RootSignals: []IncidentSignal{
			{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
			{Source: "disk_util", Type: SignalTypeAnomaly, Severity: model.SeverityCritical},
		},
		Impact: ImpactScope{
			Subsystems: []string{"cpu", "disk"},
		},
		StartTime: now.Add(-1 * time.Hour),
	}

	t.Run("Identical Incidents have 1.0 Similarity", func(t *testing.T) {
		score, breakdown, expl := CalculateSimilarity(incBase, incBase)
		if score != 1.0 {
			t.Errorf("expected similarity score 1.0 for identical incidents, got %.3f", score)
		}
		if breakdown.SymptomSimilarity != 1.0 || breakdown.SubsystemSimilarity != 1.0 ||
			breakdown.NodeSimilarity != 1.0 || breakdown.SeveritySimilarity != 1.0 {
			t.Errorf("expected all breakdown components to be 1.0, got %+v", breakdown)
		}
		if expl == "" {
			t.Errorf("expected non-empty explanation")
		}
	})

	t.Run("Completely Disjoint Incidents have 0.0 or minimal Similarity", func(t *testing.T) {
		incDisjoint := Incident{
			ID:              "inc-disjoint",
			Severity:        model.SeverityInfo,
			PrimarySymptoms: []string{"Certificate expiry approaching"},
			AffectedNodes:   []string{"node-99"},
			RootSignals: []IncidentSignal{
				{Source: "cert_check", Type: SignalTypeDiagnostic, Severity: model.SeverityInfo},
			},
			Impact: ImpactScope{
				Subsystems: []string{"security"},
			},
			StartTime: now.Add(-24 * time.Hour),
		}

		score, breakdown, _ := CalculateSimilarity(incBase, incDisjoint)
		if score != 0.0 {
			t.Errorf("expected similarity score 0.0 for completely disjoint incidents, got %.3f", score)
		}
		if breakdown.SymptomSimilarity != 0.0 || breakdown.SubsystemSimilarity != 0.0 ||
			breakdown.NodeSimilarity != 0.0 || breakdown.SeveritySimilarity != 0.0 {
			t.Errorf("expected all breakdown components to be 0.0, got %+v", breakdown)
		}
	})

	t.Run("Partial Overlap Incident", func(t *testing.T) {
		incPartial := Incident{
			ID:              "inc-partial",
			Severity:        model.SeverityCritical,
			PrimarySymptoms: []string{"High CPU load on cores"}, // 1 shared symptom
			AffectedNodes:   []string{"node-1", "node-3"},      // 1 shared node
			RootSignals: []IncidentSignal{
				{Source: "cpu_usage", Type: SignalTypeAlert, Severity: model.SeverityCritical},
			},
			Impact: ImpactScope{
				Subsystems: []string{"cpu", "memory"}, // 1 shared subsystem
			},
			StartTime: now.Add(-2 * time.Hour),
		}

		score, breakdown, _ := CalculateSimilarity(incBase, incPartial)
		if score <= 0.0 || score >= 1.0 {
			t.Errorf("expected intermediate score (0 < score < 1), got %.3f", score)
		}
		if breakdown.SeveritySimilarity != 1.0 {
			t.Errorf("expected severity similarity 1.0 since both are Critical, got %.2f", breakdown.SeveritySimilarity)
		}
	})
}

func TestFindSimilarIncidents(t *testing.T) {
	now := time.Now().UTC()

	target := Incident{
		ID:              "target-inc",
		Severity:        model.SeverityCritical,
		PrimarySymptoms: []string{"OOM killer invoked"},
		AffectedNodes:   []string{"node-1"},
		RootSignals: []IncidentSignal{
			{Source: "memory_pressure", Type: SignalTypeAlert, Severity: model.SeverityCritical},
		},
		Impact: ImpactScope{
			Subsystems: []string{"memory"},
		},
		StartTime: now,
	}

	candidates := []Incident{
		target, // Should be ignored because ID matches target
		{
			ID:              "cand-high-match",
			Severity:        model.SeverityCritical,
			PrimarySymptoms: []string{"OOM killer invoked"},
			AffectedNodes:   []string{"node-1"},
			RootSignals: []IncidentSignal{
				{Source: "memory_pressure", Type: SignalTypeAlert, Severity: model.SeverityCritical},
			},
			Impact: ImpactScope{
				Subsystems: []string{"memory"},
			},
			StartTime: now.Add(-5 * time.Minute),
		},
		{
			ID:              "cand-low-match",
			Severity:        model.SeverityInfo,
			PrimarySymptoms: []string{"Network interface link flip"},
			AffectedNodes:   []string{"node-5"},
			RootSignals: []IncidentSignal{
				{Source: "net_link", Type: SignalTypeDiagnostic, Severity: model.SeverityInfo},
			},
			Impact: ImpactScope{
				Subsystems: []string{"network"},
			},
			StartTime: now.Add(-10 * time.Minute),
		},
	}

	results := FindSimilarIncidents(target, candidates, 0.50, 10)
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 similar incident above 0.50 threshold, got %d", len(results))
	}
	if results[0].Incident.ID != "cand-high-match" {
		t.Errorf("expected matched incident ID cand-high-match, got %s", results[0].Incident.ID)
	}
	if results[0].SimilarityScore < 0.99 {
		t.Errorf("expected near 1.0 similarity, got %.3f", results[0].SimilarityScore)
	}
}
