package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestIncidentClusterer_ClusterFleetSignals(t *testing.T) {
	clusterer := NewIncidentClusterer(15 * time.Minute)
	now := time.Now().UTC()

	bundles := []NodeSignalsBundle{
		{
			NodeID:   "node-1",
			Hostname: "srv-01",
			Tags:     map[string]string{"env": "prod"},
			Alerts: []model.AlertEvent{
				{
					ID:       "alt-1",
					RuleName: "HighCPU",
					Severity: model.SeverityCritical,
					FiredAt:  now.Add(-2 * time.Minute),
					IsActive: true,
					Message:  "CPU > 90%",
				},
				{
					ID:       "alt-isolated-1",
					RuleName: "IsolatedDiskWarn",
					Severity: model.SeverityWarning,
					FiredAt:  now.Add(-1 * time.Minute),
					IsActive: true,
					Message:  "Disk > 80%",
				},
			},
		},
		{
			NodeID:   "node-2",
			Hostname: "srv-02",
			Tags:     map[string]string{"env": "prod"},
			Alerts: []model.AlertEvent{
				{
					ID:       "alt-2",
					RuleName: "HighCPU", // Matches node-1 HighCPU -> will form multi-node cluster
					Severity: model.SeverityCritical,
					FiredAt:  now.Add(-3 * time.Minute),
					IsActive: true,
					Message:  "CPU > 95%",
				},
			},
		},
		{
			NodeID:   "node-3",
			Hostname: "srv-03",
			Tags:     map[string]string{"env": "prod"},
			Alerts: []model.AlertEvent{
				{
					ID:       "alt-3",
					RuleName: "HighCPU", // Matches node-1, node-2 HighCPU -> 3 nodes (3/3 = 100% of fleet -> Fleet Scope)
					Severity: model.SeverityCritical,
					FiredAt:  now.Add(-4 * time.Minute),
					IsActive: true,
					Message:  "CPU > 92%",
				},
			},
		},
	}

	incidents := clusterer.ClusterFleetSignals(bundles, 3)

	if len(incidents) < 1 {
		t.Fatalf("expected at least 1 incident, got %d", len(incidents))
	}

	// First incident should be the multi-node/fleet HighCPU incident
	foundFleetInc := false
	foundIsolatedInc := false

	for _, inc := range incidents {
		if inc.Scope == IncidentScopeFleet || inc.Scope == IncidentScopeMultiNode {
			foundFleetInc = true
			if len(inc.AffectedNodes) != 3 {
				t.Errorf("expected 3 affected nodes in fleet incident, got %d", len(inc.AffectedNodes))
			}
			if inc.Severity != model.SeverityCritical {
				t.Errorf("expected Critical severity, got %v", inc.Severity)
			}
			if len(inc.Timeline) != 3 {
				t.Errorf("expected 3 timeline entries, got %d", len(inc.Timeline))
			}
		} else if inc.Scope == IncidentScopeNode {
			foundIsolatedInc = true
			if len(inc.AffectedNodes) != 1 || inc.AffectedNodes[0] != "node-1" {
				t.Errorf("expected single-node incident for node-1, got %+v", inc.AffectedNodes)
			}
		}
	}

	if !foundFleetInc {
		t.Errorf("expected fleet/multi-node incident to be formed")
	}
	if !foundIsolatedInc {
		t.Errorf("expected isolated single-node incident for node-1 to be formed")
	}
}
