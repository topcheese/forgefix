package engine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection and provides migration management.
type DB struct {
	conn      *sql.DB
	configDir string
}

// DBPath returns the path to the SQLite database file.
func DBPath(configDir string) string {
	return filepath.Join(configDir, ".ff", "forgefix.db")
}

// OpenDB opens (or creates) the SQLite database and runs pending migrations.
func OpenDB(configDir string) (*DB, error) {
	path := DBPath(configDir)
	// Ensure the .ff directory exists
	ffDir := filepath.Dir(path)
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		return nil, fmt.Errorf("creating .ff directory: %w", err)
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	// Enable WAL mode for concurrent reads
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enabling WAL: %w", err)
	}
	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}
	db := &DB{conn: conn, configDir: configDir}
	if err := db.Migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying database connection for queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// ConfigDir returns the config directory this DB was opened for.
func (db *DB) ConfigDir() string {
	return db.configDir
}

// ---------------------------------------------------------------------------
// Meta helpers (key-value store for project metadata)
// ---------------------------------------------------------------------------

// SetMeta sets a key-value pair in the meta table.
func (db *DB) SetMeta(key, value string) error {
	_, err := db.conn.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// GetMeta retrieves a value by key from the meta table. Returns empty string
// if the key does not exist.
func (db *DB) GetMeta(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// GetAllMeta returns all key-value pairs from the meta table.
func (db *DB) GetAllMeta() (map[string]string, error) {
	rows, err := db.conn.Query("SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// ProjectVersion returns the current project version from meta.
func (db *DB) ProjectVersion() (string, error) {
	return db.GetMeta("project_version")
}

// SetProjectVersion updates the project version in meta.
func (db *DB) SetProjectVersion(version string) error {
	return db.SetMeta("project_version", version)
}

// ---------------------------------------------------------------------------
// Pipeline stats helpers
// ---------------------------------------------------------------------------

// UpsertPipelineStats inserts or updates a pipeline's statistics row.
func (db *DB) UpsertPipelineStats(id string, ran, passed, failed, floor int) error {
	_, err := db.conn.Exec(`
		INSERT INTO pipeline_stats (pipeline_id, total_ran, total_passed, total_failed, historical_floor, last_update)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(pipeline_id) DO UPDATE SET
			total_ran = excluded.total_ran,
			total_passed = excluded.total_passed,
			total_failed = excluded.total_failed,
			historical_floor = excluded.historical_floor,
			last_update = datetime('now')
	`, id, ran, passed, failed, floor)
	return err
}

// GetPipelineStats returns stats for a single pipeline.
func (db *DB) GetPipelineStats(id string) (*PipelineStat, error) {
	s := &PipelineStat{}
	err := db.conn.QueryRow(
		"SELECT pipeline_id, total_ran, total_passed, total_failed, historical_floor, last_update FROM pipeline_stats WHERE pipeline_id = ?", id,
	).Scan(&s.PipelineID, &s.TotalRan, &s.TotalPassed, &s.TotalFailed, &s.HistoricalFloor, &s.LastUpdate)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// GetAllPipelineStats returns all pipeline stats rows.
func (db *DB) GetAllPipelineStats() ([]PipelineStat, error) {
	rows, err := db.conn.Query("SELECT pipeline_id, total_ran, total_passed, total_failed, historical_floor, last_update FROM pipeline_stats")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PipelineStat
	for rows.Next() {
		var s PipelineStat
		if err := rows.Scan(&s.PipelineID, &s.TotalRan, &s.TotalPassed, &s.TotalFailed, &s.HistoricalFloor, &s.LastUpdate); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// Spec helpers
// ---------------------------------------------------------------------------

// UpsertSpec inserts or updates a spec record.
func (db *DB) UpsertSpec(specID, title, status, specType, version string, repoIssueID int, rootCause, resolution, body string) error {
	_, err := db.conn.Exec(`
		INSERT INTO specs (spec_id, title, status, type, version, repo_issue_id, root_cause, resolution, body, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(spec_id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			type = excluded.type,
			version = excluded.version,
			repo_issue_id = excluded.repo_issue_id,
			root_cause = excluded.root_cause,
			resolution = excluded.resolution,
			body = excluded.body,
			updated_at = datetime('now')
	`, specID, title, status, specType, version, repoIssueID, rootCause, resolution, body)
	return err
}

// AddLinkedCommit records a commit hash for a spec.
func (db *DB) AddLinkedCommit(specID, commitHash string) error {
	_, err := db.conn.Exec(
		"INSERT OR IGNORE INTO linked_commits (spec_id, commit_hash) VALUES (?, ?)",
		specID, commitHash,
	)
	return err
}

// GetLinkedCommits returns all commit hashes for a spec.
func (db *DB) GetLinkedCommits(specID string) ([]string, error) {
	rows, err := db.conn.Query("SELECT commit_hash FROM linked_commits WHERE spec_id = ? ORDER BY created_at", specID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// PipelineStat data type
// ---------------------------------------------------------------------------

// PipelineStat represents a single pipeline's statistics row.
type PipelineStat struct {
	PipelineID      string
	TotalRan        int
	TotalPassed     int
	TotalFailed     int
	HistoricalFloor int
	LastUpdate      string
}

// ---------------------------------------------------------------------------
// Migration system
// ---------------------------------------------------------------------------

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{version: 1, sql: initialSchema},
	{version: 2, sql: migration002},
}

const initialSchema = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS specs (
    spec_id       TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'draft',
    type          TEXT NOT NULL DEFAULT 'feature',
    version       TEXT NOT NULL DEFAULT '0.9.0',
    repo_issue_id INTEGER NOT NULL DEFAULT 0,
    root_cause    TEXT NOT NULL DEFAULT '',
    resolution    TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS kanban_boards (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS kanban_columns (
    id        TEXT PRIMARY KEY,
    board_id  TEXT NOT NULL REFERENCES kanban_boards(id) ON DELETE CASCADE,
    title     TEXT NOT NULL,
    position  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS kanban_cards (
    id         TEXT PRIMARY KEY,
    column_id  TEXT NOT NULL REFERENCES kanban_columns(id) ON DELETE CASCADE,
    card_type  TEXT NOT NULL DEFAULT 'spec',
    title      TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'todo',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS linked_commits (
    spec_id    TEXT NOT NULL REFERENCES specs(spec_id) ON DELETE CASCADE,
    commit_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (spec_id, commit_hash)
);
`

const migration002 = `
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pipeline_stats (
    pipeline_id      TEXT PRIMARY KEY,
    total_ran        INTEGER NOT NULL DEFAULT 0,
    total_passed     INTEGER NOT NULL DEFAULT 0,
    total_failed     INTEGER NOT NULL DEFAULT 0,
    historical_floor INTEGER NOT NULL DEFAULT 0,
    last_update      TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO meta (key, value) VALUES ('project_version', '0.9.0');
`

// Migrate applies any pending migrations.
func (db *DB) Migrate() error {
	// Create the schema_version table if it doesn't exist
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	for _, m := range migrations {
		var exists bool
		err := db.conn.QueryRow("SELECT 1 FROM schema_version WHERE version = ?", m.version).Scan(&exists)
		if err == sql.ErrNoRows {
			tx, err := db.conn.Begin()
			if err != nil {
				return fmt.Errorf("beginning transaction for migration %d: %w", m.version, err)
			}
			if _, err := tx.Exec(m.sql); err != nil {
				tx.Rollback()
				return fmt.Errorf("running migration %d: %w", m.version, err)
			}
			if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", m.version); err != nil {
				tx.Rollback()
				return fmt.Errorf("recording migration %d: %w", m.version, err)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing migration %d: %w", m.version, err)
			}
		} else if err != nil {
			return fmt.Errorf("checking migration %d: %w", m.version, err)
		}
	}
	return nil
}
