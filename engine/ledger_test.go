package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLedgerSaveAndLoad asserts that the ledger engine can successfully
// marshal entry maps to disk as JSON and read them back cleanly.
func TestLedgerSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerPath := filepath.Join(tmpDir, ".forgefix_ledger.json")

	// 1. Initialize a new ledger engine and inject a pipeline high score
	engine := NewLedgerEngine()
	entry := engine.GetOrCreateEntry("go-main-app")
	entry.HistoricalFloor = 42
	entry.TotalRan = 42
	entry.TotalPassed = 42

	// 2. Persist the state map to your temporary disk directory
	// Note: We use a local helper function if available, or simulate SaveToFile
	err := engine.SaveToFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed to save ledger file to disk: %v", err)
	}

	// 3. Instantiate a second, empty ledger engine and load the saved file
	newEngine := NewLedgerEngine()
	err = newEngine.LoadFromFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed to load ledger file from disk: %v", err)
	}

	// 4. Structural validation: Ensure the historical ceiling is perfectly intact
	loadedEntry := newEngine.GetEntry("go-main-app")
	if loadedEntry == nil {
		t.Fatal("expected to load 'go-main-app' ledger entry, got nil")
	}
	if loadedEntry.HistoricalFloor != 42 {
		t.Errorf("expected historical floor of 42, got %d", loadedEntry.HistoricalFloor)
	}
}

// TestLedgerLoadNullEntries verifies that a ledger with "entries": null (which
// can occur from older versions or manual edits) loads without error and does
// not panic on subsequent map writes like GetOrCreateEntry.
func TestLedgerLoadNullEntries(t *testing.T) {
	nullEntriesJSON := `{
  "version": "0.8.0",
  "entries": null,
  "spec_mappings": {
    "SPEC-TEST": {
      "spec_id": "SPEC-TEST",
      "repo_issue_id": 42,
      "status": "draft",
      "linked_commits": []
    }
  }
}`

	le := NewLedgerEngine()
	if err := le.LoadFromJSON([]byte(nullEntriesJSON)); err != nil {
		t.Fatalf("LoadFromJSON returned error on null entries: %v", err)
	}

	// GetOrCreateEntry must not panic on nil entries map
	entry := le.GetOrCreateEntry("test-pipeline")
	if entry == nil {
		t.Fatal("GetOrCreateEntry returned nil")
	}
	if entry.PipelineID != "test-pipeline" {
		t.Errorf("expected PipelineID 'test-pipeline', got %q", entry.PipelineID)
	}

	// Spec mappings should still load correctly
	spec := le.GetSpecEntry("SPEC-TEST")
	if spec == nil {
		t.Fatal("expected SPEC-TEST spec entry, got nil")
	}
	if spec.RepoIssueID != 42 {
		t.Errorf("expected RepoIssueID 42, got %d", spec.RepoIssueID)
	}

	// Save and reload — should serialize entries as {} not null
	writeDir := t.TempDir()
	if err := SaveLedger(le, writeDir); err != nil {
		t.Fatalf("SaveLedger failed: %v", err)
	}
	reloaded, err := LoadLedger(writeDir)
	if err != nil {
		t.Fatalf("LoadLedger after save failed: %v", err)
	}
	if reloaded.GetEntry("test-pipeline") == nil {
		t.Error("expected test-pipeline entry after reload")
	}
	if reloaded.GetSpecEntry("SPEC-TEST") == nil {
		t.Error("expected SPEC-TEST entry after reload")
	}
}

// TestLedgerResetCurrentRun verifies that ResetCurrentRun zeroes out transient
// per-run metrics while securely preserving the permanent historical floor ceiling.
func TestLedgerResetCurrentRun(t *testing.T) {
	engine := NewLedgerEngine()
	entry := engine.GetOrCreateEntry("flutter-ui")
	entry.HistoricalFloor = 265
	entry.TotalRan = 100
	entry.TotalPassed = 99
	entry.TotalFailed = 1

	// Execute your tactical cleanup method receiver
	engine.ResetCurrentRun()

	// Assert that transient metrics are wiped but the ratchet ceiling is intact
	updatedEntry := engine.GetEntry("flutter-ui")
	if updatedEntry.TotalRan != 0 || updatedEntry.TotalPassed != 0 || updatedEntry.TotalFailed != 0 {
		t.Errorf("expected run metrics to be completely zeroed, got Ran=%d Passed=%d", updatedEntry.TotalRan, updatedEntry.TotalPassed)
	}
	if updatedEntry.HistoricalFloor != 265 {
		t.Errorf("expected historical baseline floor to be preserved at 265, got %d", updatedEntry.HistoricalFloor)
	}
}

func TestSpecLifecycleFiltering(t *testing.T) {
	le := NewLedgerEngine()

	le.SetSpecEntry("SPEC-ACTIVE", &SpecEntry{SpecID: "SPEC-ACTIVE", Status: "in-progress"})
	le.SetSpecEntry("SPEC-ARCHIVED", &SpecEntry{SpecID: "SPEC-ARCHIVED", Status: "closed"})

	activeOnly, err := le.ListSpecs(false)
	if err != nil {
		t.Fatalf("ListSpecs returned error: %v", err)
	}
	if len(activeOnly) != 1 || activeOnly[0].SpecID != "SPEC-ACTIVE" {
		t.Errorf("ListSpecs(false) = %d specs, want 1 (SPEC-ACTIVE)", len(activeOnly))
	}

	all, err := le.ListSpecs(true)
	if err != nil {
		t.Fatalf("ListSpecs(true) returned error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListSpecs(true) = %d specs, want 2", len(all))
	}
}

func createTestSpecFile(t *testing.T, specDir, specID string) string {
	t.Helper()
	content := fmt.Sprintf(`---
spec_id: "%s"
status: draft
repo_issue: 42
---
# Test Spec
`, specID)
	dir := filepath.Join(specDir, "specs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating specs dir: %v", err)
	}
	path := filepath.Join(dir, specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing spec file: %v", err)
	}
	return path
}

func TestDeleteSpec_RemovesFileAndLedgerEntry(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-DELETE-TEST", &SpecEntry{
		SpecID:      "SPEC-DELETE-TEST",
		Status:      "in-progress",
		RepoIssueID: 42,
	})

	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	specFile := createTestSpecFile(t, tmpDir, "SPEC-DELETE-TEST")

	if _, err := os.Stat(specFile); os.IsNotExist(err) {
		t.Fatal("spec file should exist before deletion")
	}

	repoID, err := le.DeleteSpec("SPEC-DELETE-TEST", tmpDir)
	if err != nil {
		t.Fatalf("DeleteSpec returned error: %v", err)
	}
	if repoID != 42 {
		t.Errorf("expected repoID 42, got %d", repoID)
	}

	if le.GetSpecEntry("SPEC-DELETE-TEST") != nil {
		t.Error("spec entry should be removed from ledger")
	}

	if _, err := os.Stat(specFile); !os.IsNotExist(err) {
		t.Error("spec file should be removed from filesystem")
	}

	reloaded, err := LoadLedger(tmpDir)
	if err != nil {
		t.Fatalf("reloading ledger: %v", err)
	}
	if reloaded.GetSpecEntry("SPEC-DELETE-TEST") != nil {
		t.Error("spec entry should persist as removed after reload")
	}
}

func TestDeleteSpec_NonExistentSpec(t *testing.T) {
	tmpDir := t.TempDir()
	le := NewLedgerEngine()

	_, err := le.DeleteSpec("SPEC-NONEXISTENT", tmpDir)
	if err == nil {
		t.Fatal("expected error for non-existent spec, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestDeleteSpec_SkipsMissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-NO-FILE", &SpecEntry{
		SpecID:      "SPEC-NO-FILE",
		Status:      "draft",
		RepoIssueID: 0,
	})
	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	repoID, err := le.DeleteSpec("SPEC-NO-FILE", tmpDir)
	if err != nil {
		t.Fatalf("DeleteSpec returned error: %v", err)
	}
	if repoID != 0 {
		t.Errorf("expected repoID 0, got %d", repoID)
	}
	if le.GetSpecEntry("SPEC-NO-FILE") != nil {
		t.Error("spec entry should be removed from ledger")
	}
}

func TestQueueDeleteIssue_PersistsAndLoads(t *testing.T) {
	tmpDir := t.TempDir()

	if err := QueueDeleteIssue(tmpDir, "SPEC-QUEUE-TEST", 99); err != nil {
		t.Fatalf("QueueDeleteIssue returned error: %v", err)
	}

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue returned error: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 sync op, got %d", len(ops))
	}
	if ops[0].Type != SyncOpDeleteIssue {
		t.Errorf("expected type delete_issue, got %s", ops[0].Type)
	}
	if ops[0].SpecID != "SPEC-QUEUE-TEST" {
		t.Errorf("expected spec_id SPEC-QUEUE-TEST, got %s", ops[0].SpecID)
	}
	if ops[0].IssueNum != 99 {
		t.Errorf("expected issue_number 99, got %d", ops[0].IssueNum)
	}
}

func TestQueueDeleteIssue_ZeroIssueNumberSkips(t *testing.T) {
	tmpDir := t.TempDir()

	if err := QueueDeleteIssue(tmpDir, "SPEC-SKIP", 0); err != nil {
		t.Fatalf("QueueDeleteIssue returned error: %v", err)
	}

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue returned error: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 sync op, got %d", len(ops))
	}
	if ops[0].IssueNum != 0 {
		t.Errorf("expected issue_number 0, got %d", ops[0].IssueNum)
	}
}

func TestDeleteSpec_ArchiveFirst_RemoteClosed_Succeeds(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-ARCHIVE-TEST", &SpecEntry{
		SpecID:      "SPEC-ARCHIVE-TEST",
		Status:      "closed",
		RepoIssueID: 100,
	})
	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	specFile := createTestSpecFile(t, tmpDir, "SPEC-ARCHIVE-TEST")

	coord, transport := newMockCoordinator()

	// Mock: remote issue is already closed
	issue := GitHubIssue{
		ID: 100, Number: 100, Title: "Test Spec", State: "closed",
	}
	issueData, _ := json.Marshal(issue)
	transport.setResponse("GET",
		testBaseURL+"/repos/test-owner/test-repo/issues/100",
		200, issueData)

	repoID, err := le.DeleteSpecWithArchive("SPEC-ARCHIVE-TEST", tmpDir, coord)
	if err != nil {
		t.Fatalf("DeleteSpecWithArchive returned error: %v", err)
	}
	if repoID != 100 {
		t.Errorf("expected repoID 100, got %d", repoID)
	}

	// Verify spec file removed
	if _, err := os.Stat(specFile); !os.IsNotExist(err) {
		t.Error("spec file should be removed from filesystem")
	}

	// Verify ledger entry removed
	if le.GetSpecEntry("SPEC-ARCHIVE-TEST") != nil {
		t.Error("spec entry should be removed from ledger")
	}
}

func TestDeleteSpec_ArchiveFirst_RemoteOpen_Aborts(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-ARCHIVE-FAIL", &SpecEntry{
		SpecID:      "SPEC-ARCHIVE-FAIL",
		Status:      "in-progress",
		RepoIssueID: 200,
	})
	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	specFile := createTestSpecFile(t, tmpDir, "SPEC-ARCHIVE-FAIL")

	coord, transport := newMockCoordinator()

	// Mock: remote issue is still OPEN
	issue := GitHubIssue{
		ID: 200, Number: 200, Title: "Test Spec", State: "open",
	}
	issueData, _ := json.Marshal(issue)
	transport.setResponse("GET",
		testBaseURL+"/repos/test-owner/test-repo/issues/200",
		200, issueData)

	repoID, err := le.DeleteSpecWithArchive("SPEC-ARCHIVE-FAIL", tmpDir, coord)
	if err == nil {
		t.Fatal("expected error for open remote issue, got nil")
	}
	if repoID != 200 {
		t.Errorf("expected repoID 200, got %d", repoID)
	}

	// Verify spec file NOT removed (archive-first aborts)
	if _, err := os.Stat(specFile); os.IsNotExist(err) {
		t.Error("spec file should NOT be removed when remote is open")
	}

	// Verify ledger entry NOT removed
	if le.GetSpecEntry("SPEC-ARCHIVE-FAIL") == nil {
		t.Error("spec entry should NOT be removed when remote is open")
	}
}

func TestDeleteSpec_ArchiveFirst_RemoteNotFound_Aborts(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-ARCHIVE-404", &SpecEntry{
		SpecID:      "SPEC-ARCHIVE-404",
		Status:      "closed",
		RepoIssueID: 300,
	})
	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	specFile := createTestSpecFile(t, tmpDir, "SPEC-ARCHIVE-404")

	coord, transport := newMockCoordinator()

	// Mock: remote issue not found (404)
	transport.setResponse("GET",
		testBaseURL+"/repos/test-owner/test-repo/issues/300",
		404, map[string]string{"message": "not found"})

	repoID, err := le.DeleteSpecWithArchive("SPEC-ARCHIVE-404", tmpDir, coord)
	if err == nil {
		t.Fatal("expected error for 404 remote issue, got nil")
	}
	if repoID != 300 {
		t.Errorf("expected repoID 300, got %d", repoID)
	}

	// Verify spec file NOT removed
	if _, err := os.Stat(specFile); os.IsNotExist(err) {
		t.Error("spec file should NOT be removed when remote not found")
	}
}

func TestDeleteSpec_FullIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	le := NewLedgerEngine()
	le.SetSpecEntry("SPEC-INTEGRATION", &SpecEntry{
		SpecID:      "SPEC-INTEGRATION",
		Status:      "in-progress",
		RepoIssueID: 55,
		Type:        "refactor",
	})
	if err := SaveLedger(le, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	createTestSpecFile(t, tmpDir, "SPEC-INTEGRATION")

	reloaded, err := LoadLedger(tmpDir)
	if err != nil {
		t.Fatalf("loading ledger: %v", err)
	}

	repoID, err := reloaded.DeleteSpec("SPEC-INTEGRATION", tmpDir)
	if err != nil {
		t.Fatalf("DeleteSpec returned error: %v", err)
	}
	if repoID != 55 {
		t.Errorf("expected repoID 55, got %d", repoID)
	}

	if reloaded.GetSpecEntry("SPEC-INTEGRATION") != nil {
		t.Error("spec entry should be removed from loaded ledger")
	}

	specDir := filepath.Join(tmpDir, "specs")
	entries, _ := os.ReadDir(specDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "SPEC-INTEGRATION") {
			t.Error("spec file should not exist on disk")
			break
		}
	}

	// Confirm deletion persists on a fresh load
	freshLedger, err := LoadLedger(tmpDir)
	if err != nil {
		t.Fatalf("loading fresh ledger: %v", err)
	}
	if freshLedger.GetSpecEntry("SPEC-INTEGRATION") != nil {
		t.Error("spec entry should remain removed after fresh reload from disk")
	}
}

// TestLedgerPreservesProjectVersion verifies that the project release version
// (written by `ff ship`) is not overwritten by the application Version constant
// when the ledger is saved via SaveToFile.
func TestLedgerPreservesProjectVersion(t *testing.T) {
	tmpDir := t.TempDir()
	ledgerFile := filepath.Join(tmpDir, ".forgefix_ledger.json")

	engine := NewLedgerEngine()
	engine.Version = "0.9.5"
	if err := engine.SaveToFile(ledgerFile); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	loaded := NewLedgerEngine()
	if err := loaded.LoadFromFile(ledgerFile); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loaded.Version != "0.9.5" {
		t.Errorf("expected project version 0.9.5 after save/load, got %q", loaded.Version)
	}

	loaded.Version = "1.0.0"
	if err := loaded.SaveToFile(ledgerFile); err != nil {
		t.Fatalf("second SaveToFile: %v", err)
	}
	reloaded := NewLedgerEngine()
	if err := reloaded.LoadFromFile(ledgerFile); err != nil {
		t.Fatalf("second LoadFromFile: %v", err)
	}
	if reloaded.Version != "1.0.0" {
		t.Errorf("expected updated project version 1.0.0, got %q", reloaded.Version)
	}
	if reloaded.Version == Version {
		t.Errorf("project version must not be overwritten by app constant %q", Version)
	}
}
