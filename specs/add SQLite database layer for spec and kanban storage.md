---
spec_id: "SPEC-1783712062"
status: ship
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
---
# Add Sqlite Database Layer For Spec And Kanban Storage

## Objective

Add `modernc.org/sqlite` (pure Go, no CGO) as a project dependency and create a
database layer that replaces the JSON-based ledger (`forgefix_ledger.json`) and
provides storage for new features (kanban board, spec metadata, etc.). New code
that needs persistent state uses the DB from the start rather than adding more
JSON files that need migration later.

## Requirements

1. Add `modernc.org/sqlite` dependency via `go get`.
2. Create `engine/db.go` with connection management (open, close, migrate).
3. Schema includes tables for: specs, kanban boards, columns, cards.
4. The database file lives at `.ff/forgefix.db` (alongside the ledger JSON for
   now). Eventually the JSON ledger is replaced entirely.
5. Migration system: a `schema_version` table tracks which migrations have run;
   new migrations are applied automatically on `ff` startup.
6. All new features (kanban, etc.) use the DB exclusively — no new JSON files.
7. Existing JSON ledger code continues to work unchanged until migration.

## Implementation

- `go get modernc.org/sqlite` — pure Go SQLite, no CGO or system SQLite lib.
- New file `engine/db.go`:
  - `type DB struct { conn *sql.DB }` wrapping the connection.
  - `func OpenDB(configDir string) (*DB, error)` creates/opens `.ff/forgefix.db`.
  - `func (db *DB) Migrate() error` runs pending migrations.
  - `func (db *DB) Close() error` closes the connection.
- Migration files in `engine/migrations/` or inline as `[]migration{...}`:
  - `001_initial.sql`: creates `specs`, `kanban_boards`, `kanban_columns`, `kanban_cards` tables.
- The `DB` struct is available via the existing `IssueCoordinator` or a new
  `DBProvider` interface so other packages can access it without circular deps.

## Acceptance Criteria

- `go build ./...` succeeds with `modernc.org/sqlite` included.
- `.ff/forgefix.db` is created on first `ff` command that needs it.
- Schema version table exists and migrations run in order.
- Existing JSON ledger (`forgefix_ledger.json`) is untouched by the DB layer.
- All existing tests pass.

## Verification

- `go test ./... -count=1` — all modules green.
- `ls .ff/forgefix.db` exists after running any ff command that initializes the DB.
- `sqlite3 .ff/forgefix.db ".tables"` shows the expected tables.
- Unit test: `TestDBOpenAndMigrate` creates a temp DB, runs migrations, checks tables.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->