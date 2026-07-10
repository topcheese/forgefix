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
// Migration system
// ---------------------------------------------------------------------------

type migration struct {
	version int
	sql     string
}

var migrations = []migration{
	{version: 1, sql: initialSchema},
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
