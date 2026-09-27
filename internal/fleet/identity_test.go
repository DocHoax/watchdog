package fleet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestGetOrGenerateNodeID(t *testing.T) {
	tempDir := t.TempDir()
	idFile := filepath.Join(tempDir, "node_id")

	// 1. Generate new ID
	id1, err := GetOrGenerateNodeID(idFile)
	if err != nil {
		t.Fatalf("Failed to generate node ID: %v", err)
	}
	if id1 == "" {
		t.Fatalf("Generated node ID is empty")
	}
	if _, err := uuid.Parse(id1); err != nil {
		t.Errorf("Generated node ID is not a valid UUID: %s", id1)
	}

	// 2. Read existing ID from file
	id2, err := GetOrGenerateNodeID(idFile)
	if err != nil {
		t.Fatalf("Failed to read existing node ID: %v", err)
	}
	if id1 != id2 {
		t.Errorf("Expected same ID %s, got %s", id1, id2)
	}

	// 3. Environment variable override
	t.Setenv("WATCHDOG_NODE_ID", "custom-env-node-id")
	id3, err := GetOrGenerateNodeID(idFile)
	if err != nil {
		t.Fatalf("Failed with env var: %v", err)
	}
	if id3 != "custom-env-node-id" {
		t.Errorf("Expected custom-env-node-id, got %s", id3)
	}
}

func TestSaveNodeID(t *testing.T) {
	tempDir := t.TempDir()
	idFile := filepath.Join(tempDir, "subdir", "node_id")
	testID := "test-node-id-12345"

	if err := SaveNodeID(idFile, testID); err != nil {
		t.Fatalf("Failed to save node ID: %v", err)
	}

	data, err := os.ReadFile(idFile)
	if err != nil {
		t.Fatalf("Failed to read saved node ID: %v", err)
	}
	if string(data) != testID+"\n" {
		t.Errorf("Expected content %q, got %q", testID+"\n", string(data))
	}
}

func TestDiscoverNodeIdentity(t *testing.T) {
	ctx := context.Background()
	tags := map[string]string{"env": "test-env", "region": "us-east-1"}

	ident, err := DiscoverNodeIdentity(ctx, "node-test-01", "v1.0.0", tags)
	if err != nil {
		t.Fatalf("Failed to discover node identity: %v", err)
	}

	if ident.NodeID != "node-test-01" {
		t.Errorf("Expected node_id node-test-01, got %s", ident.NodeID)
	}
	if ident.Hostname == "" {
		t.Errorf("Expected non-empty hostname")
	}
	if ident.OS == "" {
		t.Errorf("Expected non-empty OS")
	}
	if ident.Arch == "" {
		t.Errorf("Expected non-empty Arch")
	}
	if ident.CPUCores <= 0 {
		t.Errorf("Expected CPU cores > 0, got %d", ident.CPUCores)
	}
	if ident.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", ident.Version)
	}
	if ident.Tags["env"] != "test-env" {
		t.Errorf("Expected tag env=test-env, got %v", ident.Tags)
	}
}
