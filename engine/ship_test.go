package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeShipSpecFile(t *testing.T, dir, specID, status string) {
	t.Helper()
	content := "---\n" +
		"spec_id: " + specID + "\n" +
		"status: " + status + "\n" +
		"type: feature\n" +
		"---\n"
	path := filepath.Join(dir, specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func setupSpecsDir(t *testing.T, specs map[string]string) string {
	t.Helper()
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for specID, status := range specs {
		writeShipSpecFile(t, specsDir, specID, status)
	}
	return configDir
}

func TestCheckShipGateSpecStatuses_AllShip(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "ship",
		"SPEC-200": "ship",
	})

	shipSpecs, err := checkShipGateSpecStatuses(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shipSpecs) != 2 {
		t.Fatalf("expected 2 ship specs, got %d: %v", len(shipSpecs), shipSpecs)
	}
}

func TestCheckShipGateSpecStatuses_BacklogBlocks(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "ship",
		"SPEC-200": "backlog",
		"SPEC-300": "ship",
	})

	_, err := checkShipGateSpecStatuses(configDir)
	if err == nil {
		t.Fatal("expected error for backlog spec, got nil")
	}
	if !strings.Contains(err.Error(), "SPEC-200") {
		t.Errorf("error should mention SPEC-200, got: %v", err)
	}
	if !strings.Contains(err.Error(), "backlog") {
		t.Errorf("error should mention 'backlog', got: %v", err)
	}
}

func TestCheckShipGateSpecStatuses_InProgressBlocks(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "in-progress",
	})

	_, err := checkShipGateSpecStatuses(configDir)
	if err == nil {
		t.Fatal("expected error for in-progress spec, got nil")
	}
	if !strings.Contains(err.Error(), "in-progress") {
		t.Errorf("error should mention 'in-progress', got: %v", err)
	}
}

func TestCheckShipGateSpecStatuses_ReviewBlocks(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "review",
	})

	_, err := checkShipGateSpecStatuses(configDir)
	if err == nil {
		t.Fatal("expected error for review spec, got nil")
	}
	if !strings.Contains(err.Error(), "review") {
		t.Errorf("error should mention 'review', got: %v", err)
	}
}

func TestCheckShipGateSpecStatuses_NoSpecsDir(t *testing.T) {
	configDir := t.TempDir()

	shipSpecs, err := checkShipGateSpecStatuses(configDir)
	if err != nil {
		t.Fatalf("unexpected error for missing specs dir: %v", err)
	}
	if len(shipSpecs) != 0 {
		t.Fatalf("expected no ship specs for missing dir, got %d", len(shipSpecs))
	}
}

func TestCheckShipGateSpecStatuses_DraftPasses(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "ship",
		"SPEC-200": "draft",
		"SPEC-300": "ready",
	})

	shipSpecs, err := checkShipGateSpecStatuses(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shipSpecs) != 1 {
		t.Fatalf("expected 1 ship spec, got %d: %v", len(shipSpecs), shipSpecs)
	}
	if shipSpecs[0] != "SPEC-100" {
		t.Errorf("expected SPEC-100, got %s", shipSpecs[0])
	}
}

func TestCheckShipGateSpecStatuses_MultipleBlocking(t *testing.T) {
	configDir := setupSpecsDir(t, map[string]string{
		"SPEC-100": "backlog",
		"SPEC-200": "in-progress",
		"SPEC-300": "review",
	})

	_, err := checkShipGateSpecStatuses(configDir)
	if err == nil {
		t.Fatal("expected error for blocking only, got nil")
	}
	for _, id := range []string{"SPEC-100", "SPEC-200", "SPEC-300"} {
		if !strings.Contains(err.Error(), id) {
			t.Errorf("error should mention %s, got: %v", id, err)
		}
	}
}
