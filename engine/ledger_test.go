package engine

import (
	"path/filepath"
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
