package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestVersionCommand_Default(t *testing.T) {
	versionShort = false
	versionJSON = false

	out := captureOutput(func() {
		versionCmd.Run(versionCmd, []string{})
	})

	if !strings.Contains(out, "Watchdog") {
		t.Errorf("expected version output to contain 'Watchdog', got: %s", out)
	}
	if !strings.Contains(out, "Build Date") {
		t.Errorf("expected version output to contain 'Build Date', got: %s", out)
	}
	if !strings.Contains(out, "Go Version") {
		t.Errorf("expected version output to contain 'Go Version', got: %s", out)
	}
	if !strings.Contains(out, "Platform") {
		t.Errorf("expected version output to contain 'Platform', got: %s", out)
	}
}

func TestVersionCommand_Short(t *testing.T) {
	versionShort = true
	versionJSON = false
	defer func() { versionShort = false }()

	out := captureOutput(func() {
		versionCmd.Run(versionCmd, []string{})
	})

	trimmed := strings.TrimSpace(out)
	if trimmed != Version {
		t.Errorf("expected short output '%s', got '%s'", Version, trimmed)
	}
}

func TestVersionCommand_JSON(t *testing.T) {
	versionShort = false
	versionJSON = true
	defer func() { versionJSON = false }()

	out := captureOutput(func() {
		versionCmd.Run(versionCmd, []string{})
	})

	var info VersionInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("failed to unmarshal version JSON: %v, raw output: %s", err, out)
	}

	if info.Version != Version {
		t.Errorf("expected JSON version '%s', got '%s'", Version, info.Version)
	}
	if info.BuildDate != BuildDate {
		t.Errorf("expected JSON build date '%s', got '%s'", BuildDate, info.BuildDate)
	}
	if info.GoVersion == "" {
		t.Error("expected non-empty go_version in JSON")
	}
	if info.Platform == "" {
		t.Error("expected non-empty platform in JSON")
	}
	if info.Compiler == "" {
		t.Error("expected non-empty compiler in JSON")
	}
}

func TestVersionCommand_CommitFallback(t *testing.T) {
	origGitCommit := GitCommit
	origCommit := Commit
	defer func() {
		GitCommit = origGitCommit
		Commit = origCommit
	}()

	GitCommit = "dev"
	Commit = "legacy-commit-sha"

	versionShort = false
	versionJSON = true
	defer func() { versionJSON = false }()

	out := captureOutput(func() {
		versionCmd.Run(versionCmd, []string{})
	})

	var info VersionInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if info.Commit != "legacy-commit-sha" {
		t.Errorf("expected commit fallback to 'legacy-commit-sha', got '%s'", info.Commit)
	}
}

func TestVersionCommand_BuiltBy(t *testing.T) {
	origBuiltBy := BuiltBy
	defer func() { BuiltBy = origBuiltBy }()

	BuiltBy = "goreleaser"
	versionShort = false
	versionJSON = false

	out := captureOutput(func() {
		versionCmd.Run(versionCmd, []string{})
	})

	if !strings.Contains(out, "Built By   : goreleaser") {
		t.Errorf("expected output to contain 'Built By   : goreleaser', got: %s", out)
	}
}
