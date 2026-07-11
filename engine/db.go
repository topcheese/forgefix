package engine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Non-fatal: import existing JSON ledger data into the DB if it hasn't
	// been imported yet. Failures are logged but don't block startup.
	if err := db.ImportLedger(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ledger import: %v\n", err)
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

// ArchiveSpec sets a spec's status to "archived" in the DB and deletes its file.
// Returns an error if the file can't be removed (non-fatal) or the DB update fails.
func (db *DB) ArchiveSpec(specID, title, specType string, repoIssueID int) error {
	specDir := filepath.Join(db.configDir, "specs")
	// Use findSpecFileByID to locate the file by spec_id, not by title —
	// the title in the ledger may not match the actual filename on disk.
	filePath, err := findSpecFileByID(specDir, specID)
	if err == nil {
		if err := os.Remove(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove spec file %s: %v\n", filepath.Base(filePath), err)
		}
	}
	return db.UpsertSpec(specID, title, "archived", specType, "", repoIssueID, "", "", "")
}

// ---------------------------------------------------------------------------
// Kanban helpers
// ---------------------------------------------------------------------------

// KanbanBoard represents a board with its columns and cards.
type KanbanBoard struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Columns   []KanbanColumn `json:"columns"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// KanbanColumn represents a column within a board.
type KanbanColumn struct {
	ID        string       `json:"id"`
	BoardID   string       `json:"board_id"`
	Title     string       `json:"title"`
	Position  int          `json:"position"`
	Cards     []KanbanCard `json:"cards"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
}

// KanbanCard represents a single card in a column.
type KanbanCard struct {
	ID        string `json:"id"`
	ColumnID  string `json:"column_id"`
	CardType  string `json:"card_type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// InitDefaultBoard creates a board with standard columns if none exists.
// Returns the board ID.
func (db *DB) InitDefaultBoard() (string, error) {
	// Check if any board exists
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM kanban_boards").Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		var id string
		if err := db.conn.QueryRow("SELECT id FROM kanban_boards LIMIT 1").Scan(&id); err != nil {
			return "", err
		}
		return id, nil
	}

	boardID := "default"
	if _, err := db.conn.Exec("INSERT INTO kanban_boards (id, name) VALUES (?, ?)", boardID, "Default Board"); err != nil {
		return "", err
	}

	columns := []struct {
		id    string
		title string
		pos   int
	}{
		{"todo", "To Do", 0},
		{"in-progress", "In Progress", 1},
		{"review", "Review", 2},
		{"done", "Done", 3},
	}
	for _, c := range columns {
		if _, err := db.conn.Exec(
			"INSERT INTO kanban_columns (id, board_id, title, position) VALUES (?, ?, ?, ?)",
			boardID+"-"+c.id, boardID, c.title, c.pos,
		); err != nil {
			return "", err
		}
	}
	return boardID, nil
}

// CreateColumn adds a new column to a board.
func (db *DB) CreateColumn(boardID, title string) error {
	var maxPos int
	db.conn.QueryRow("SELECT COALESCE(MAX(position), -1) FROM kanban_columns WHERE board_id = ?", boardID).Scan(&maxPos)
	id := fmt.Sprintf("%s-col-%d", boardID, maxPos+1)
	_, err := db.conn.Exec(
		"INSERT INTO kanban_columns (id, board_id, title, position) VALUES (?, ?, ?, ?)",
		id, boardID, title, maxPos+1,
	)
	return err
}

// CreateCard adds a card to a column. Returns the card ID.
func (db *DB) CreateCard(columnID, cardType, title string) (string, error) {
	id := fmt.Sprintf("card-%d", time.Now().UnixMilli())
	_, err := db.conn.Exec(
		"INSERT INTO kanban_cards (id, column_id, card_type, title) VALUES (?, ?, ?, ?)",
		id, columnID, cardType, title,
	)
	return id, err
}

// ListBoards returns all boards.
func (db *DB) ListBoards() ([]KanbanBoard, error) {
	rows, err := db.conn.Query("SELECT id, name, created_at, updated_at FROM kanban_boards ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var boards []KanbanBoard
	for rows.Next() {
		var b KanbanBoard
		if err := rows.Scan(&b.ID, &b.Name, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

// ListBoard retrieves a board with all its columns and cards.
func (db *DB) ListBoard(boardID string) (*KanbanBoard, error) {
	b := &KanbanBoard{}
	if err := db.conn.QueryRow("SELECT id, name, created_at, updated_at FROM kanban_boards WHERE id = ?", boardID).
		Scan(&b.ID, &b.Name, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}

	colRows, err := db.conn.Query(
		"SELECT id, board_id, title, position, created_at, updated_at FROM kanban_columns WHERE board_id = ? ORDER BY position", boardID)
	if err != nil {
		return nil, err
	}
	defer colRows.Close()
	for colRows.Next() {
		var c KanbanColumn
		if err := colRows.Scan(&c.ID, &c.BoardID, &c.Title, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cardRows, err := db.conn.Query(
			"SELECT id, column_id, card_type, title, status, created_at, updated_at FROM kanban_cards WHERE column_id = ? ORDER BY created_at", c.ID)
		if err != nil {
			return nil, err
		}
		for cardRows.Next() {
			var card KanbanCard
			if err := cardRows.Scan(&card.ID, &card.ColumnID, &card.CardType, &card.Title, &card.Status, &card.CreatedAt, &card.UpdatedAt); err != nil {
				cardRows.Close()
				return nil, err
			}
			c.Cards = append(c.Cards, card)
		}
		cardRows.Close()
		b.Columns = append(b.Columns, c)
	}
	return b, nil
}

// ImportArchiveFiles reads all existing specs/archive/archive_*.md files and
// imports each spec entry into the DB with status "archived". After a successful
// import the archive directory is removed. Idempotent — skips specs already in DB.
func ImportArchiveFiles(configDir string, db *DB) error {
	archiveDir := filepath.Join(configDir, "specs", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading archive directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(archiveDir, entry.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping archive file %s: %v\n", entry.Name(), err)
			continue
		}
		// Each archive file contains multiple specs separated by --- dividers.
		content := string(data)
		blocks := strings.Split(content, "\n\n---\n\n")
		for _, block := range blocks {
			fm, body := parseArchiveBlock(block)
			if fm["spec_id"] == "" {
				continue
			}
			status := fm["status"]
			if status == "" {
				status = "archived"
			}
			_ = db.UpsertSpec(fm["spec_id"], fm["spec_id"], status, fm["type"], "", 0, fm["root_cause"], fm["resolution"], body)
		}
	}

	// Remove the archive directory after successful import.
	if err := os.RemoveAll(archiveDir); err != nil {
		return fmt.Errorf("removing archive directory: %w", err)
	}
	return nil
}

// parseArchiveBlock extracts frontmatter and body from a single archive entry.
func parseArchiveBlock(block string) (map[string]string, string) {
	fm := make(map[string]string)
	if !strings.HasPrefix(block, "---") {
		return fm, block
	}
	parts := strings.SplitN(block, "---", 3)
	if len(parts) < 3 {
		return fm, block
	}
	for _, line := range strings.Split(parts[1], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		val = strings.Trim(val, `"`)
		if idx := strings.Index(val, " "); idx > 0 && key != "resolution" {
			val = val[:idx]
		}
		fm[key] = val
	}
	body := strings.TrimSpace(parts[2])
	return fm, body
}

// CountArchivedSpecs returns the number of specs in the DB that have been
// archived (status = 'archived', 'closed', or 'resolved'). These are specs
// that were imported from old archive files OR archived by `ff archive`.
func (db *DB) CountArchivedSpecs() (int, error) {
	var count int
	err := db.Conn().QueryRow("SELECT COUNT(*) FROM specs WHERE status IN ('archived', 'closed', 'resolved')").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ImportLedger reads the current JSON ledger and writes all spec mappings and
// pipeline entries into the DB. Idempotent — existing rows are overwritten.
func (db *DB) ImportLedger() error {
	ledger, err := LoadLedger(db.configDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}

	// Import project version — skip if still at the engine default (no real ledger loaded).
	if ledger.Version != "" && ledger.Version != "0.0.0" {
		if err := db.SetProjectVersion(ledger.Version); err != nil {
			return fmt.Errorf("setting project version: %w", err)
		}
	}

	// Import pipeline stats
	for id, entry := range ledger.GetAllEntries() {
		if err := db.UpsertPipelineStats(id, entry.TotalRan, entry.TotalPassed, entry.TotalFailed, entry.HistoricalFloor); err != nil {
			return fmt.Errorf("importing pipeline %s: %w", id, err)
		}
	}

	// Import spec mappings
	for specID, entry := range ledger.GetAllSpecEntries() {
		if err := db.UpsertSpec(specID, entry.SpecID, entry.Status, entry.Type, "", entry.RepoIssueID, "", "", ""); err != nil {
			return fmt.Errorf("importing spec %s: %w", specID, err)
		}
		for _, hash := range entry.LinkedCommits {
			if err := db.AddLinkedCommit(specID, hash); err != nil {
				return fmt.Errorf("importing linked commit %s for %s: %w", hash, specID, err)
			}
		}
	}

	return nil
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
	{version: 3, sql: migration003},
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

const migration003 = `
CREATE INDEX IF NOT EXISTS idx_specs_status ON specs(status);
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
