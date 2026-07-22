package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipController_UpdateBacklogVersion(t *testing.T) {
	configDir := t.TempDir()
	specsDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create backlog, draft, and in-progress specs with old version
	oldVersion := "0.9.0"
	newVersion := "1.0.0"

	specs := map[string]string{
		"SPEC-BACKLOG": "backlog",
		"SPEC-DRAFT":   "draft",
		"SPEC-REVIEW":  "review",
		"SPEC-SHIP":    "ship",
	}

	for id, status := range specs {
		content := "---\n" +
			"spec_id: " + id + "\n" +
			"status: " + status + "\n" +
			"type: feature\n" +
			"version: \"" + oldVersion + "\"\n" +
			"---\n" +
			"# " + id + "\n"
		path := filepath.Join(specsDir, id+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Initialize DB and import specs
	db, err := OpenDB(configDir)
	if err != nil {
		t.Fatal(err)
	}
	// Upsert specs into DB
	for id, status := range specs {
		if err := db.UpsertSpec(id, id, status, "feature", oldVersion, 0, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	sc := NewShipController(nil, configDir, false)
	if err := sc.updateBacklogVersion(newVersion); err != nil {
		t.Fatalf("updateBacklogVersion failed: %v", err)
	}

	// Verify versions
	expected := map[string]string{
		"SPEC-BACKLOG": newVersion,
		"SPEC-DRAFT":   newVersion,
		"SPEC-REVIEW":  oldVersion,
		"SPEC-SHIP":    oldVersion,
	}

	for id, wantVer := range expected {
		// Check file
		path := filepath.Join(specsDir, id+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "version: \""+wantVer+"\"") {
			t.Errorf("Spec %s file: expected version %q, but not found in content:\n%s", id, wantVer, string(data))
		}

		// Check DB
		db, err := OpenDB(configDir)
		if err != nil {
			t.Fatal(err)
		}
		var gotVer string
		err = db.Conn().QueryRow("SELECT version FROM specs WHERE spec_id = ?", id).Scan(&gotVer)
		db.Close()
		if err != nil {
			t.Errorf("DB query for %s failed: %v", id, err)
			continue
		}
		if gotVer != wantVer {
			t.Errorf("Spec %s DB: expected version %q, got %q", id, wantVer, gotVer)
		}
	}
}
