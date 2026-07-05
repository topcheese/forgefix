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

	ledger := NewLedgerEngine()
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
		writeArchiveSpecFile(t, specsDir, entry.specID, entry.status)
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

	archivePath := filepath.Join(specsDir, "archive", archiveName)
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
	if entry := reloaded.GetSpecEntry("SPEC-100"); entry != nil {
		t.Error("SPEC-100 should have been removed from ledger after archiving")
	}
	if entry := reloaded.GetSpecEntry("SPEC-300"); entry != nil {
		t.Error("SPEC-300 should have been removed from ledger after archiving")
	}
	if entry := reloaded.GetSpecEntry("SPEC-200"); entry == nil || entry.Status != "ship" {
		t.Error("SPEC-200 should remain in ledger with status ship (not archived)")
	}
	if entry := reloaded.GetSpecEntry("SPEC-400"); entry == nil || entry.Status != "draft" {
		t.Error("SPEC-400 should remain in ledger with status draft (not archived)")
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
		writeArchiveSpecFile(t, specsDir, entry.specID, entry.status)
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

func TestArchiveResolvedSpecs_ArchivesOrphanedLedgerEntries(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Only create one file on disk; leave the other as an orphaned ledger entry
	writeArchiveSpecFile(t, specsDir, "SPEC-100", "closed")

	ledger := NewLedgerEngine()
	ledger.WorkflowConfig = DefaultWorkflowConfig()
	ledger.SetSpecEntry("SPEC-100", &SpecEntry{SpecID: "SPEC-100", Status: "closed"})
	ledger.SetSpecEntry("SPEC-200", &SpecEntry{
		SpecID:        "SPEC-200",
		Status:        "closed",
		RepoIssueID:   42,
		Type:          "bug",
		LinkedCommits: []string{"abc123"},
	})
	if err := SaveLedger(ledger, configDir); err != nil {
		t.Fatal(err)
	}

	archiveName, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs failed: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archived specs (1 file + 1 orphaned ledger), got %d", count)
	}

	archivePath := filepath.Join(specsDir, "archive", archiveName)
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "SPEC-100") {
		t.Error("archive should contain SPEC-100 from file")
	}
	if !strings.Contains(content, "SPEC-200") {
		t.Error("archive should contain orphaned SPEC-200 from ledger")
	}
	if !strings.Contains(content, "Spec file was missing at time of archive") {
		t.Error("archive should note that SPEC-200 was reconstructed from ledger")
	}

	reloaded, err := LoadLedger(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if entry := reloaded.GetSpecEntry("SPEC-100"); entry != nil {
		t.Error("SPEC-100 should have been removed from ledger after archiving")
	}
	if entry := reloaded.GetSpecEntry("SPEC-200"); entry != nil {
		t.Error("SPEC-200 should have been removed from ledger after archiving")
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
		writeArchiveSpecFile(t, specsDir, entry.specID, entry.status)
	}

	archiveName, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs failed: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 archived specs (resolved + closed), got %d", count)
	}

	archivePath := filepath.Join(specsDir, "archive", archiveName)
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
