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
	allTables := []string{"schema_version", "specs", "kanban_boards", "kanban_columns", "kanban_cards", "linked_commits", "meta", "pipeline_stats"}
	for _, table := range allTables {
		var count int
		err := db.Conn().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist or query failed: %v", table, err)
		}
	}

	// Verify migration 1 was recorded
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

	// Verify migration 2 was recorded
	err = db.Conn().QueryRow("SELECT version, applied_at FROM schema_version WHERE version = 2").Scan(&version, &appliedAt)
	if err != nil {
		t.Fatalf("migration 2 not recorded: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}

	// Verify default meta value was inserted by migration 2
	v, err := db.ProjectVersion()
	if err != nil {
		t.Fatalf("ProjectVersion: %v", err)
	}
	if v != "0.9.0" {
		t.Errorf("expected default project version 0.9.0, got %q", v)
	}

	// Verify re-opening runs migrations again without error
	db.Close()
	db2, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB second time: %v", err)
	}
	defer db2.Close()
}

func TestDBMeta(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := db.SetMeta("test_key", "test_value"); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}
	v, err := db.GetMeta("test_key")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if v != "test_value" {
		t.Errorf("GetMeta = %q, want %q", v, "test_value")
	}

	all, err := db.GetAllMeta()
	if err != nil {
		t.Fatalf("GetAllMeta: %v", err)
	}
	if all["test_key"] != "test_value" {
		t.Errorf("GetAllMeta missing test_key")
	}
	if all["project_version"] == "" {
		t.Error("GetAllMeta missing project_version")
	}

	// Overwrite and verify
	if err := db.SetProjectVersion("0.9.9"); err != nil {
		t.Fatalf("SetProjectVersion: %v", err)
	}
	v, err = db.ProjectVersion()
	if err != nil {
		t.Fatalf("ProjectVersion: %v", err)
	}
	if v != "0.9.9" {
		t.Errorf("ProjectVersion = %q, want %q", v, "0.9.9")
	}
}

func TestDBPipelineStats(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := db.UpsertPipelineStats("test-pipe", 10, 8, 2, 5); err != nil {
		t.Fatalf("UpsertPipelineStats: %v", err)
	}
	s, err := db.GetPipelineStats("test-pipe")
	if err != nil {
		t.Fatalf("GetPipelineStats: %v", err)
	}
	if s.TotalRan != 10 || s.TotalPassed != 8 || s.TotalFailed != 2 || s.HistoricalFloor != 5 {
		t.Errorf("unexpected stats: %+v", s)
	}

	// Upsert again and verify update
	if err := db.UpsertPipelineStats("test-pipe", 20, 18, 2, 5); err != nil {
		t.Fatalf("UpsertPipelineStats: %v", err)
	}
	s, err = db.GetPipelineStats("test-pipe")
	if err != nil {
		t.Fatalf("GetPipelineStats: %v", err)
	}
	if s.TotalRan != 20 {
		t.Errorf("TotalRan = %d, want 20", s.TotalRan)
	}

	all, err := db.GetAllPipelineStats()
	if err != nil {
		t.Fatalf("GetAllPipelineStats: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 pipeline stat, got %d", len(all))
	}
}

func TestDBSpecs(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	if err := db.UpsertSpec("SPEC-100", "Test Spec", "draft", "feature", "0.9.0", 42, "", "", "# Body"); err != nil {
		t.Fatalf("UpsertSpec: %v", err)
	}

	if err := db.AddLinkedCommit("SPEC-100", "abc123"); err != nil {
		t.Fatalf("AddLinkedCommit: %v", err)
	}
	if err := db.AddLinkedCommit("SPEC-100", "def456"); err != nil {
		t.Fatalf("AddLinkedCommit: %v", err)
	}

	commits, err := db.GetLinkedCommits("SPEC-100")
	if err != nil {
		t.Fatalf("GetLinkedCommits: %v", err)
	}
	if len(commits) != 2 {
		t.Errorf("expected 2 linked commits, got %d", len(commits))
	}
	if commits[0] != "abc123" || commits[1] != "def456" {
		t.Errorf("unexpected commits: %v", commits)
	}

	// Verify upsert updates
	if err := db.UpsertSpec("SPEC-100", "Updated Spec", "ship", "feature", "0.9.0", 42, "cause", "resolved", ""); err != nil {
		t.Fatalf("UpsertSpec: %v", err)
	}
}

func TestDBPath(t *testing.T) {
	path := DBPath("/some/project")
	expected := "/some/project/.ff/forgefix.db"
	if path != expected {
		t.Errorf("DBPath(%q) = %q, want %q", "/some/project", path, expected)
	}
}
