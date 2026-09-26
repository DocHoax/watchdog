package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildReproducibility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping reproducibility test in short mode")
	}

	tempDir := t.TempDir()
	bin1 := filepath.Join(tempDir, "watchdog-1")
	bin2 := filepath.Join(tempDir, "watchdog-2")
	if runtime.GOOS == "windows" {
		bin1 += ".exe"
		bin2 += ".exe"
	}

	fixedLdflags := "-s -w -X github.com/DocHoax/watchdog/cmd.Version=1.0.0-repro -X github.com/DocHoax/watchdog/cmd.GitCommit=0000000000000000000000000000000000000000 -X github.com/DocHoax/watchdog/cmd.BuildDate=2026-09-26T00:00:00Z -X github.com/DocHoax/watchdog/cmd.BuiltBy=repro-test"

	// Build 1
	cmd1 := exec.Command("go", "build", "-trimpath", "-ldflags="+fixedLdflags, "-o", bin1, "..")
	cmd1.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd1.CombinedOutput(); err != nil {
		t.Fatalf("build 1 failed: %v\nOutput: %s", err, string(out))
	}

	// Build 2
	cmd2 := exec.Command("go", "build", "-trimpath", "-ldflags="+fixedLdflags, "-o", bin2, "..")
	cmd2.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("build 2 failed: %v\nOutput: %s", err, string(out))
	}

	hash1 := hashFile(t, bin1)
	hash2 := hashFile(t, bin2)

	if hash1 != hash2 {
		t.Fatalf("deterministic build failed: build 1 hash (%s) != build 2 hash (%s)", hash1, hash2)
	}
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read binary file %s: %v", path, err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
