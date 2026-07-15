package engine

import (
	"os"
	"path/filepath"
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
	path := filepath.Join(dir, "specs", specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func addToLedger(t *testing.T, configDir, specID, status string) {
	t.Helper()
	ledger, err := LoadLedger(configDir)
	if err != nil {
		t.Fatal(err)
	}
	ledger.SetSpecEntry(specID, &SpecEntry{
		SpecID:      specID,
		Status:      status,
		RepoIssueID: 0,
	})
	if err := SaveLedger(ledger, configDir); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveResolvedSpecs_ArchivesClosedSpecs(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeArchiveSpecFile(t, configDir, "SPEC-ARCHIVE-CLOSED", "closed")
	addToLedger(t, configDir, "SPEC-ARCHIVE-CLOSED", "closed")

	_, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 archived spec, got %d", count)
	}

	// Spec file should be deleted
	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-ARCHIVE-CLOSED.md")); !os.IsNotExist(err) {
		t.Error("spec file should have been deleted")
	}

	// Ledger entry should be removed
	ledger, err := LoadLedger(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if entry := ledger.GetSpecEntry("SPEC-ARCHIVE-CLOSED"); entry != nil {
		t.Error("ledger entry should have been removed")
	}

	// DB should have the spec with status archived
	db, err := OpenDB(configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status string
	err = db.Conn().QueryRow("SELECT status FROM specs WHERE spec_id = ?", "SPEC-ARCHIVE-CLOSED").Scan(&status)
	if err != nil {
		t.Fatalf("querying archived spec: %v", err)
	}
	if status != "archived" {
		t.Errorf("expected status 'archived', got %q", status)
	}
}

func TestArchiveResolvedSpecs_DoesNotArchiveDraft(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeArchiveSpecFile(t, configDir, "SPEC-ARCHIVE-DRAFT", "draft")
	addToLedger(t, configDir, "SPEC-ARCHIVE-DRAFT", "draft")

	_, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 archived specs for draft, got %d", count)
	}

	// File should still exist
	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-ARCHIVE-DRAFT.md")); os.IsNotExist(err) {
		t.Error("draft spec file should not have been deleted")
	}
}

func TestArchiveResolvedSpecs_ArchivesOrphanedLedgerEntries(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Ledger has a closed spec with no file
	addToLedger(t, configDir, "SPEC-ORPHAN", "closed")

	_, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 archived orphan, got %d", count)
	}

	// Ledger entry should be removed
	ledger, err := LoadLedger(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if entry := ledger.GetSpecEntry("SPEC-ORPHAN"); entry != nil {
		t.Error("orphan ledger entry should have been removed")
	}

	// DB should have it
	db, err := OpenDB(configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status string
	err = db.Conn().QueryRow("SELECT status FROM specs WHERE spec_id = ?", "SPEC-ORPHAN").Scan(&status)
	if err != nil {
		t.Fatalf("querying archived orphan: %v", err)
	}
	if status != "archived" {
		t.Errorf("expected status 'archived', got %q", status)
	}
}

func TestArchiveResolvedSpecs_MixedResolvedAndClosed(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeArchiveSpecFile(t, configDir, "SPEC-MIX-A", "closed")
	writeArchiveSpecFile(t, configDir, "SPEC-MIX-B", "resolved")
	writeArchiveSpecFile(t, configDir, "SPEC-MIX-C", "draft")
	addToLedger(t, configDir, "SPEC-MIX-A", "closed")
	addToLedger(t, configDir, "SPEC-MIX-B", "resolved")
	addToLedger(t, configDir, "SPEC-MIX-C", "draft")

	_, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 archived specs (closed + resolved), got %d", count)
	}

	// Draft file should still exist
	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-MIX-C.md")); os.IsNotExist(err) {
		t.Error("draft spec file should not have been deleted")
	}

	// DB should have 2 archived specs
	db, err := OpenDB(configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var archivedCount int
	err = db.Conn().QueryRow("SELECT COUNT(*) FROM specs WHERE status = 'archived'").Scan(&archivedCount)
	if err != nil {
		t.Fatalf("querying archived count: %v", err)
	}
	if archivedCount != 2 {
		t.Errorf("expected 2 archived specs in DB, got %d", archivedCount)
	}
}

func TestArchiveResolvedSpecs_QuarantinesMalformedFiles(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a malformed spec file (empty spec_id)
	malformedContent := "---\n" +
		"spec_id: \"\"\n" +
		"status: closed\n" +
		"type: feature\n" +
		"created: 2026-07-03\n" +
		"---\n" +
		"# Malformed Spec\n\n" +
		"## Goal\n\nTest spec with empty spec_id.\n"
	malformedPath := filepath.Join(specsDir, "malformed.md")
	if err := os.WriteFile(malformedPath, []byte(malformedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a valid closed spec
	writeArchiveSpecFile(t, configDir, "SPEC-VALID", "closed")
	addToLedger(t, configDir, "SPEC-VALID", "closed")

	_, count, err := ArchiveResolvedSpecs(configDir)
	if err != nil {
		t.Fatalf("ArchiveResolvedSpecs: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 archived spec, got %d", count)
	}

	// Valid spec file should be deleted
	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-VALID.md")); !os.IsNotExist(err) {
		t.Error("valid spec file should have been deleted")
	}

	// Malformed file should be quarantined
	quarantineDir := filepath.Join(specsDir, "_quarantine")
	quarantinedPath := filepath.Join(quarantineDir, "malformed.md")
	if _, err := os.Stat(quarantinedPath); os.IsNotExist(err) {
		t.Errorf("malformed file should have been quarantined to %s", quarantinedPath)
	}

	// DB should have the valid spec archived
	db, err := OpenDB(configDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var status string
	err = db.Conn().QueryRow("SELECT status FROM specs WHERE spec_id = ?", "SPEC-VALID").Scan(&status)
	if err != nil {
		t.Fatalf("querying archived spec: %v", err)
	}
	if status != "archived" {
		t.Errorf("expected status 'archived', got %q", status)
	}
}
