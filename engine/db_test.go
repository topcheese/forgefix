package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDBOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".ff")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	db, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Verify the database file was created
	if _, err := os.Stat(DBPath(dir)); os.IsNotExist(err) {
		t.Error("database file was not created")
	}

	// Verify tables exist
	tables := []string{"schema_version", "specs", "kanban_boards", "kanban_columns", "kanban_cards"}
	for _, table := range tables {
		var count int
		err := db.Conn().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist or query failed: %v", table, err)
		}
	}

	// Verify migration was recorded
	var version int
	var appliedAt string
	err = db.Conn().QueryRow("SELECT version, applied_at FROM schema_version WHERE version = 1").Scan(&version, &appliedAt)
	if err != nil {
		t.Fatalf("migration 1 not recorded: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
	if appliedAt == "" {
		t.Error("applied_at should not be empty")
	}

	// Verify re-opening runs migrations again without error
	db.Close()
	db2, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB second time: %v", err)
	}
	defer db2.Close()
}

func TestDBPath(t *testing.T) {
	path := DBPath("/some/project")
	expected := "/some/project/.ff/forgefix.db"
	if path != expected {
		t.Errorf("DBPath(%q) = %q, want %q", "/some/project", path, expected)
	}
}
