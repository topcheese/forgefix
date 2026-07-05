package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandDispatcher_StatusRoutesCorrectly(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ForgeFix Status") {
		t.Errorf("expected status output to contain 'ForgeFix Status', got:\n%s", stdout.String())
	}
}

func TestCommandDispatcher_StatusWithBlockingSpecs(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()

	// Create the .ff directory and minimal ledger
	ffDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create specs dir with a blocking spec
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a backlog spec that blocks ship
	backlogSpec := `---
spec_id: "SPEC-TEST-001"
status: backlog
repo_issue: ""
type: feature
version: ""
---
# Test Backlog Spec

## Objective

Test

## Requirements

- None
`
	if err := os.WriteFile(filepath.Join(specsDir, "test-backlog.md"), []byte(backlogSpec), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "⛔ Blocked") {
		t.Errorf("expected output to show blocked ship gate, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "SPEC-TEST-001") {
		t.Errorf("expected output to mention blocking spec, got:\n%s", stdout.String())
	}
}

func TestCommandDispatcher_StatusWithSyncFailures(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()

	// Create .ff dir
	ffDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a sync failure
	failures := `[{"timestamp":"2026-07-05T00:00:00Z","spec_id":"SPEC-FAIL","error":"connection refused"}]`
	if err := os.WriteFile(filepath.Join(ffDir, ".sync_failures.log"), []byte(failures), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "⚠") {
		t.Errorf("expected output to show sync failure warning, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "SPEC-FAIL") {
		t.Errorf("expected output to mention failing spec, got:\n%s", stdout.String())
	}
}

func TestCommandDispatcher_StatusHealthy(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()

	// Create .ff dir
	ffDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		t.Fatal(err)
	}
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a ship-status spec (not blocking)
	shipSpec := `---
spec_id: "SPEC-TEST-SHIP"
status: ship
repo_issue: "1"
type: feature
version: "0.8.2"
---
# Test Ship Spec

## Objective

Test

## Requirements

- Done
`
	if err := os.WriteFile(filepath.Join(specsDir, "test-ship.md"), []byte(shipSpec), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ Clear") {
		t.Errorf("expected output to show clear ship gate, got:\n%s", stdout.String())
	}
}

func TestCommandDispatcher_StatusDisplaysSpecCounts(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()

	ffDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		t.Fatal(err)
	}
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write two specs: one ship, one backlog
	specs := map[string]string{
		"ship-one.md": `---
spec_id: "SPEC-SHIP-1"
status: ship
repo_issue: "1"
type: feature
version: ""
---
# Ship One
`,
		"backlog-one.md": `---
spec_id: "SPEC-BL-1"
status: backlog
repo_issue: ""
type: feature
version: ""
---
# Backlog One
`,
	}

	for name, content := range specs {
		if err := os.WriteFile(filepath.Join(specsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("status", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "2 specs") && !strings.Contains(output, "total") {
		t.Errorf("expected output to show spec totals, got:\n%s", output)
	}
}
