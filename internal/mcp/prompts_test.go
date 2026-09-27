package mcp

import (
	"strings"
	"testing"
)

func TestPromptRegistry(t *testing.T) {
	registry := NewPromptRegistry()

	t.Run("ListPrompts", func(t *testing.T) {
		prompts := registry.ListPrompts()
		if len(prompts) != 7 {
			t.Fatalf("expected 7 prompts, got %d", len(prompts))
		}

		names := make(map[string]bool)
		for _, p := range prompts {
			names[p.Name] = true
		}

		expected := []string{
			"system_health_audit",
			"diagnose_node",
			"incident_triage",
			"fleet_status_report",
			"analyze_fleet_health",
			"investigate_incident",
			"triage_node_degradation",
		}
		for _, name := range expected {
			if !names[name] {
				t.Errorf("expected prompt %s to be in ListPrompts()", name)
			}
		}
	})

	t.Run("GetPrompt system_health_audit", func(t *testing.T) {
		res, err := registry.GetPrompt("system_health_audit", map[string]string{"severity": "critical"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(res.Messages))
		}
		if !strings.Contains(res.Messages[0].Content.Text, `severity="CRITICAL"`) {
			t.Errorf("expected message to contain CRITICAL severity filter, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt diagnose_node", func(t *testing.T) {
		// Missing node_id
		_, err := registry.GetPrompt("diagnose_node", map[string]string{})
		if err == nil {
			t.Fatalf("expected error when node_id missing, got nil")
		}

		// Invalid node_id
		_, err = registry.GetPrompt("diagnose_node", map[string]string{"node_id": "node/../invalid"})
		if err == nil {
			t.Fatalf("expected error when node_id is invalid format, got nil")
		}

		// Valid node_id
		res, err := registry.GetPrompt("diagnose_node", map[string]string{"node_id": "worker-01"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `worker-01`) {
			t.Errorf("expected message to contain node_id, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt incident_triage", func(t *testing.T) {
		res, err := registry.GetPrompt("incident_triage", map[string]string{"time_window": "24h"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `since="24h"`) {
			t.Errorf("expected message to contain time_window, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt fleet_status_report", func(t *testing.T) {
		res, err := registry.GetPrompt("fleet_status_report", map[string]string{"tag": "env=production"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `filtering by tag "env=production"`) {
			t.Errorf("expected message to contain tag filter, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt analyze_fleet_health", func(t *testing.T) {
		res, err := registry.GetPrompt("analyze_fleet_health", map[string]string{"focus": "incidents"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `focusing on "incidents"`) {
			t.Errorf("expected message to contain focus, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt investigate_incident", func(t *testing.T) {
		// Missing incident_id
		_, err := registry.GetPrompt("investigate_incident", map[string]string{})
		if err == nil {
			t.Fatalf("expected error when incident_id is missing")
		}

		// Valid incident_id
		res, err := registry.GetPrompt("investigate_incident", map[string]string{"incident_id": "inc-100"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `inc-100`) {
			t.Errorf("expected message to contain incident_id, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt triage_node_degradation", func(t *testing.T) {
		// Missing node_id
		_, err := registry.GetPrompt("triage_node_degradation", map[string]string{})
		if err == nil {
			t.Fatalf("expected error when node_id is missing")
		}

		// Valid node_id
		res, err := registry.GetPrompt("triage_node_degradation", map[string]string{"node_id": "worker-02", "metric_window": "6h"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Messages[0].Content.Text, `worker-02`) || !strings.Contains(res.Messages[0].Content.Text, `window 6h`) {
			t.Errorf("expected message to contain node_id and window, got: %s", res.Messages[0].Content.Text)
		}
	})

	t.Run("GetPrompt unknown prompt", func(t *testing.T) {
		_, err := registry.GetPrompt("nonexistent_prompt", nil)
		if err == nil {
			t.Fatalf("expected error for unknown prompt")
		}
		if err.Code != CodeInvalidParams {
			t.Errorf("expected CodeInvalidParams (-32602), got %d", err.Code)
		}
	})
}
