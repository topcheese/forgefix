package engine

import (
	"os"
	"testing"
)

func TestLoadConfigFromPathValidYAML(t *testing.T) {
	yamlContent := `
project: "Test Project"
global_timeout_seconds: 120
exclude_dirs:
  - "vendor"
  - ".git"
languages:
  go_stack:
    root_anchor: "go.mod"
    test_command: "go test -json ./..."
    token_patterns:
      token_run: "Action.*run"
      token_pass: "Action.*pass"
      token_fail: "Action.*fail"
  flutter_stack:
    root_anchor: "pubspec.yaml"
    test_command: "flutter test --machine"
    token_patterns:
      token_run: "testStart"
      token_pass: "testDone"
      token_fail: "error"
pipelines:
  - id: "go-unit"
    name: "Go Unit Tests"
    type: "go_stack"
    panel_color: "blue"
    ledger_floor: 10
`

	tmpDir, err := os.MkdirTemp("", "forgefix-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := tmpDir + "/forgefix.yaml"
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	loaded, err := loadConfigFromPath(configPath)
	if err != nil {
		t.Fatalf("loadConfigFromPath failed: %v", err)
	}

	if loaded.Config.GlobalTimeoutSeconds != 120 {
		t.Errorf("expected GlobalTimeoutSeconds=120, got %d", loaded.Config.GlobalTimeoutSeconds)
	}
	if len(loaded.Config.ExcludeDirs) != 2 {
		t.Errorf("expected 2 exclude dirs, got %d", len(loaded.Config.ExcludeDirs))
	}
	if loaded.Config.ExcludeDirs[0] != "vendor" || loaded.Config.ExcludeDirs[1] != ".git" {
		t.Errorf("exclude dirs mismatch: %v", loaded.Config.ExcludeDirs)
	}
	if len(loaded.Config.Pipelines) != 1 {
		t.Errorf("expected 1 pipeline, got %d", len(loaded.Config.Pipelines))
	}
	if loaded.Config.Pipelines[0].LedgerFloor != 10 {
		t.Errorf("expected pipeline floor=10, got %d", loaded.Config.Pipelines[0].LedgerFloor)
	}
}

func TestDashboardResetTrackersClearsGhostData(t *testing.T) {
	pipelines := []PipelineConfig{
		{
			ID:   "test-pipeline",
			Name: "Test Pipeline",
			Type: "go_stack",
			TokenPatterns: TokenPatterns{
				TokenRun:  "Action.*run",
				TokenPass: "Action.*pass",
				TokenFail: "Action.*fail",
			},
		},
	}
	dashboard := NewDashboard(pipelines)

	tracker := dashboard.GetTracker("test-pipeline")
	tracker.History = []string{"✓ OldTestOne", "✗ OldTestTwo"}
	tracker.ActiveTests["ghost-test"] = &TestInfo{ID: "ghost-test", Name: "Ghost Test", State: StateRunning}
	tracker.Completed["ghost-test"] = &TestInfo{ID: "ghost-test", Name: "Ghost Test", State: StateCompleted}
	tracker.CompletedIDs["ghost-test"] = true

	dashboard.ResetTrackers()

	tracker = dashboard.GetTracker("test-pipeline")
	if len(tracker.History) != 0 {
		t.Errorf("History not cleared: %v", tracker.History)
	}
	if len(tracker.ActiveTests) != 0 {
		t.Errorf("ActiveTests not cleared: %v", tracker.ActiveTests)
	}
	if len(tracker.Completed) != 0 {
		t.Errorf("Completed not cleared: %v", tracker.Completed)
	}
	if len(tracker.CompletedIDs) != 0 {
		t.Errorf("CompletedIDs not cleared: %v", tracker.CompletedIDs)
	}
}
