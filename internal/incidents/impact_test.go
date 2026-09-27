package incidents

import (
	"testing"
	"time"

	"github.com/DocHoax/watchdog/pkg/model"
)

func TestCalculateImpact(t *testing.T) {
	now := time.Now().UTC()

	nodeHostnames := map[string]string{
		"node-1": "srv-prod-01",
		"node-2": "srv-prod-02",
		"node-3": "srv-stage-01",
		"node-4": "srv-prod-03",
	}

	nodeTags := map[string]map[string]string{
		"node-1": {"env": "prod", "region": "us-east"},
		"node-2": {"env": "prod", "region": "us-east"},
		"node-3": {"env": "stage", "region": "us-west"},
		"node-4": {"env": "prod", "region": "us-east"},
	}

	signals := []IncidentSignal{
		{
			ID:          "sig-1",
			Type:        SignalTypeAlert,
			Source:      "CPUThrottling",
			NodeID:      "node-1",
			Severity:    model.SeverityCritical,
			Timestamp:   now,
			Description: "High CPU usage across logical cores",
		},
		{
			ID:          "sig-2",
			Type:        SignalTypeDiagnostic,
			Source:      "MemoryPressure",
			NodeID:      "node-2",
			Severity:    model.SeverityWarning,
			Timestamp:   now,
			Description: "Available memory below 10%",
		},
		{
			ID:          "sig-3",
			Type:        SignalTypeAnomaly,
			Source:      "DiskSpace",
			NodeID:      "node-1",
			Severity:    model.SeverityCritical,
			Timestamp:   now,
			Description: "Root filesystem filling up rapidly",
		},
		{
			ID:          "sig-4",
			Type:        SignalTypeAlert,
			Source:      "NetworkPacketLoss",
			NodeID:      "node-2",
			Severity:    model.SeverityWarning,
			Timestamp:   now,
			Description: "High TX drop rate observed",
		},
	}

	t.Run("MultiNode Tag Cluster Impact", func(t *testing.T) {
		impact := CalculateImpact(
			"inc-123",
			[]string{"node-1", "node-2"},
			nodeHostnames,
			nodeTags,
			signals,
			10, // 2/10 = 20% (<30% fleet wide)
		)

		if impact.IncidentID != "inc-123" {
			t.Errorf("expected incident ID inc-123, got %s", impact.IncidentID)
		}
		if impact.Scope != IncidentScopeMultiNode {
			t.Errorf("expected IncidentScopeMultiNode, got %v", impact.Scope)
		}
		if impact.EstimatedBlastRadius != "tag_cluster" {
			t.Errorf("expected tag_cluster blast radius, got %s", impact.EstimatedBlastRadius)
		}
		if impact.Impact.FleetPercentage != 20.0 {
			t.Errorf("expected fleet percentage 20.0, got %.2f", impact.Impact.FleetPercentage)
		}
		if len(impact.Impact.AffectedNodeIDs) != 2 {
			t.Errorf("expected 2 affected nodes, got %d", len(impact.Impact.AffectedNodeIDs))
		}

		// Verify subsystems extracted
		expectedSubsystems := map[string]bool{
			"cpu":     true,
			"memory":  true,
			"disk":    true,
			"network": true,
		}
		for _, sub := range impact.Impact.Subsystems {
			if !expectedSubsystems[sub] {
				t.Errorf("unexpected subsystem: %s", sub)
			}
		}

		// Common tags between node-1 and node-2 should be env=prod, region=us-east
		if impact.Impact.CommonTags["env"] != "prod" || impact.Impact.CommonTags["region"] != "us-east" {
			t.Errorf("unexpected common tags: %+v", impact.Impact.CommonTags)
		}
	})

	t.Run("Single Isolated Node Impact", func(t *testing.T) {
		singleSig := []IncidentSignal{
			{
				ID:          "sig-1",
				Type:        SignalTypeAlert,
				Source:      "ProcessCrash",
				NodeID:      "node-3",
				Severity:    model.SeverityCritical,
				Timestamp:   now,
				Description: "High process fd count",
			},
		}

		impact := CalculateImpact(
			"inc-456",
			[]string{"node-3"},
			nodeHostnames,
			nodeTags,
			singleSig,
			10,
		)

		if impact.Scope != IncidentScopeNode {
			t.Errorf("expected IncidentScopeNode, got %v", impact.Scope)
		}
		if impact.EstimatedBlastRadius != "isolated_node" {
			t.Errorf("expected isolated_node, got %s", impact.EstimatedBlastRadius)
		}
		if impact.Impact.FleetPercentage != 10.0 {
			t.Errorf("expected 10.0%% fleet percentage, got %.2f", impact.Impact.FleetPercentage)
		}
		if len(impact.Impact.Subsystems) != 1 || impact.Impact.Subsystems[0] != "process" {
			t.Errorf("expected subsystem [process], got %+v", impact.Impact.Subsystems)
		}
	})

	t.Run("Fleet Wide Blast Radius", func(t *testing.T) {
		impact := CalculateImpact(
			"inc-789",
			[]string{"node-1", "node-2", "node-3", "node-4"},
			nodeHostnames,
			nodeTags,
			signals,
			4, // 4/4 = 100%
		)

		if impact.Scope != IncidentScopeFleet {
			t.Errorf("expected IncidentScopeFleet, got %v", impact.Scope)
		}
		if impact.EstimatedBlastRadius != "fleet_wide" {
			t.Errorf("expected fleet_wide, got %s", impact.EstimatedBlastRadius)
		}
		if impact.Impact.FleetPercentage != 100.0 {
			t.Errorf("expected 100.0%% fleet percentage, got %.2f", impact.Impact.FleetPercentage)
		}
	})
}
