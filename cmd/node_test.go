package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DocHoax/watchdog/internal/config"
	"github.com/DocHoax/watchdog/pkg/model"
)

func captureStdout(f func() error) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	err := f()
	_ = w.Close()
	os.Stdout = oldStdout
	output := <-outChan
	return output, err
}

func TestCmd_Node_HumanReadable(t *testing.T) {
	nodeJSON = false
	nodeShort = false
	nodeIDOverride = "test-node-hr"
	nodeTags = []string{"env=test", "role=ci"}
	globalCfg = config.DefaultConfig()

	out, err := captureStdout(func() error {
		return runNode(nodeCmd, nil)
	})
	if err != nil {
		t.Fatalf("runNode returned error: %v", err)
	}

	if !strings.Contains(out, "Watchdog Node Identity") {
		t.Errorf("Expected output to contain header, got: %s", out)
	}
	if !strings.Contains(out, "test-node-hr") {
		t.Errorf("Expected output to contain node ID, got: %s", out)
	}
	if !strings.Contains(out, "env=test") || !strings.Contains(out, "role=ci") {
		t.Errorf("Expected tags in output, got: %s", out)
	}
}

func TestCmd_Node_Short(t *testing.T) {
	nodeJSON = false
	nodeShort = true
	nodeIDOverride = "short-node-id-xyz"
	nodeTags = nil
	globalCfg = config.DefaultConfig()

	out, err := captureStdout(func() error {
		return runNode(nodeCmd, nil)
	})
	if err != nil {
		t.Fatalf("runNode short returned error: %v", err)
	}

	trimmed := strings.TrimSpace(out)
	if trimmed != "short-node-id-xyz" {
		t.Errorf("Expected 'short-node-id-xyz', got %q", trimmed)
	}
}

func TestCmd_Node_JSON(t *testing.T) {
	nodeJSON = true
	nodeShort = false
	nodeIDOverride = "json-node-id-123"
	nodeTags = []string{"tier=api", "zone=us-west"}
	globalCfg = config.DefaultConfig()

	out, err := captureStdout(func() error {
		return runNode(nodeCmd, nil)
	})
	if err != nil {
		t.Fatalf("runNode json returned error: %v", err)
	}

	var identity model.NodeIdentity
	if err := json.Unmarshal([]byte(out), &identity); err != nil {
		t.Fatalf("Failed to parse json output: %v, raw: %s", err, out)
	}

	if identity.NodeID != "json-node-id-123" {
		t.Errorf("Expected node ID json-node-id-123, got %s", identity.NodeID)
	}
	if identity.Tags["tier"] != "api" || identity.Tags["zone"] != "us-west" {
		t.Errorf("Expected tags tier=api and zone=us-west, got %v", identity.Tags)
	}
	if identity.OS == "" || identity.Hostname == "" {
		t.Errorf("Expected OS and Hostname to be populated, got %+v", identity)
	}
}

func TestCmd_Node_PersistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	idFile := filepath.Join(tmpDir, "node_id")

	nodeJSON = false
	nodeShort = true
	nodeIDOverride = ""
	nodeIDFile = idFile
	nodeTags = nil
	globalCfg = config.DefaultConfig()

	out1, err := captureStdout(func() error {
		return runNode(nodeCmd, nil)
	})
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}
	id1 := strings.TrimSpace(out1)
	if len(id1) < 10 {
		t.Fatalf("Invalid generated node ID: %s", id1)
	}

	// Second run should reuse same ID from file
	out2, err := captureStdout(func() error {
		return runNode(nodeCmd, nil)
	})
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}
	id2 := strings.TrimSpace(out2)
	if id1 != id2 {
		t.Errorf("Expected node IDs to match from persistent file: %s vs %s", id1, id2)
	}
}
