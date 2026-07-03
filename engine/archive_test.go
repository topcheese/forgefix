package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArchiveSpecFile(t *testing.T, dir, specID, status string) {
	t.Helper()
	content := "---\n" +
		"spec_id: \"" + specID + "\"\n" +
		"status: " + status + "\n" +
		"type: feature\n" +
		"created: 2026-07-03\n" +
		"---\n" +
		"# " + specID + "\n\n" +
		"## Goal\n\nTest spec for " + status + " archiving.\n"
	path := filepath.Join(dir, specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveResolvedSpecs_ArchivesClosedSpecs(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	ledger := NewLedgerEngine(configDir)
	ledger.WorkflowConfig = DefaultWorkflowConfig()

	for _, entry := range []struct {
		specID string
		status string
	}{
		{"SPEC-100", "closed"},
		{"SPEC-200", "ship"},
		{"SPEC-300", "closed"},
		{"SPEC-400", "draft"},
	} {
		writeSpecFile(t, specsDir, entry.specID, entry.status)
		ledger.SetSpecEntry(entry.specID, &SpecEntry{
			SpecID: entry.specID,
			Status: entry.status,
		})
	}
	if err := SaveLedger(ledger, configDir); err != nil {
		t.Fatal(err)
	}

	archiveName, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs failed: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archived specs (closed), got %d", count)
	}

	archivePath := filepath.Join(specsDir, archiveName)
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("archive file was not created")
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "SPEC-100") {
		t.Error("archive should contain SPEC-100")
	}
	if !strings.Contains(content, "SPEC-300") {
		t.Error("archive should contain SPEC-300")
	}
	if strings.Contains(content, "SPEC-200") {
		t.Error("archive should not contain ship status SPEC-200")
	}
	if strings.Contains(content, "SPEC-400") {
		t.Error("archive should not contain draft status SPEC-400")
	}

	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-100.md")); !os.IsNotExist(err) {
		t.Error("SPEC-100.md should have been removed after archiving")
	}
	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-300.md")); !os.IsNotExist(err) {
		t.Error("SPEC-300.md should have been removed after archiving")
	}

	reloaded, err := LoadLedger(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.GetSpecEntry("SPEC-100") != nil {
		t.Error("SPEC-100 should be removed from ledger after archiving")
	}
	if reloaded.GetSpecEntry("SPEC-300") != nil {
		t.Error("SPEC-300 should be removed from ledger after archiving")
	}
	if reloaded.GetSpecEntry("SPEC-200") == nil {
		t.Error("SPEC-200 should remain in ledger (ship, not archived)")
	}
}

func TestArchiveResolvedSpecs_NoClosedSpecs(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, entry := range []struct {
		specID string
		status string
	}{
		{"SPEC-100", "ship"},
		{"SPEC-200", "draft"},
		{"SPEC-300", "review"},
	} {
		writeSpecFile(t, specsDir, entry.specID, entry.status)
	}

	archiveName, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs failed: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0 archived specs, got %d", count)
	}
	if archiveName != "" {
		t.Errorf("expected empty archive name, got %s", archiveName)
	}
}

func TestArchiveResolvedSpecs_MixedResolvedAndClosed(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, entry := range []struct {
		specID string
		status string
	}{
		{"SPEC-100", "resolved"},
		{"SPEC-200", "closed"},
		{"SPEC-300", "ship"},
	} {
		writeSpecFile(t, specsDir, entry.specID, entry.status)
	}

	archiveName, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs failed: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archived specs (resolved + closed), got %d", count)
	}

	archivePath := filepath.Join(specsDir, archiveName)
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "SPEC-100") {
		t.Error("archive should contain resolved SPEC-100")
	}
	if !strings.Contains(content, "SPEC-200") {
		t.Error("archive should contain closed SPEC-200")
	}
}
