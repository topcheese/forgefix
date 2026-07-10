---
spec_id: "SPEC-1783715766"
status: review
repo_issue: ""
type: refactor
version: "0.9.0"
root_cause: "Database schema was created as a starting point but hasn't been reviewed or finalized. Tables need to be validated against actual usage before we start migrating features off the JSON ledger."
resolution: ""
---
# Finalize SQLite Schemas And Plan DB Integration

## Objective

Review and finalize all SQLite database tables and schemas so we have a clear
plan for migrating features from the JSON ledger to the DB. The current schema
(created in `001_initial.sql`) was a first pass — it needs to be validated
against actual query patterns and extended with tables for version tracking
and pipeline stats.

## Requirements

1. Review existing tables for correctness and completeness:
   - `specs` — does it cover all fields from `SpecEntry`?
   - `kanban_boards`, `kanban_columns`, `kanban_cards` — ready for the kanban feature?
2. Add `meta` table for key-value project metadata (version, last_update, etc.).
3. Add `pipeline_stats` table matching the `entries` in the JSON ledger.
4. Define primary query patterns for each table (CRUD operations).
5. Plan the migration order: which data moves to DB first?
6. Document the final schema as the source of truth going forward.

## Existing Tables

### `schema_version`
Tracks which migrations have been applied. Already in use.

### `specs`
```sql
CREATE TABLE specs (
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
```
Questions: should `linked_commits` be a separate table? Does `body` duplicate
the spec file on disk?

### `kanban_*` tables
Created for the kanban feature (not yet used).

## Proposed Additions

### `meta` table
```sql
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```
Stores `project_version`, `last_sync`, `last_update`, etc. Replaces the
`version` field in the JSON ledger.

### `pipeline_stats` table
```sql
CREATE TABLE pipeline_stats (
    pipeline_id      TEXT PRIMARY KEY,
    total_ran        INTEGER NOT NULL DEFAULT 0,
    total_passed     INTEGER NOT NULL DEFAULT 0,
    total_failed     INTEGER NOT NULL DEFAULT 0,
    historical_floor INTEGER NOT NULL DEFAULT 0,
    last_update      TEXT NOT NULL DEFAULT (datetime('now'))
);
```
Matches the `entries` map in the JSON ledger exactly.

## Implementation

- Add migration `002` with `meta` and `pipeline_stats` tables.
- Review and adjust `specs` table columns if needed.
- Add helper methods on `DB` for common operations:
  - `SetMeta(key, value)`, `GetMeta(key)`, `GetAllMeta()`
  - `UpsertPipelineStats`, `GetPipelineStats`, `GetAllPipelineStats`
- Document the final schema in `docs/schema.md` or inline in `db.go`.
- Plan migration order: `meta` and `pipeline_stats` migrate first (simple
  key-value), then `specs` (replaces ledger spec_mappings), then kanban.

## Acceptance Criteria

- Migration `002` runs without errors and creates the new tables.
- `meta` table stores and retrieves project version.
- `pipeline_stats` table matches the JSON ledger entries.
- All existing tests pass.

## Verification

- `go test ./engine -run TestDB -v` passes.
- `sqlite3 .ff/forgefix.db ".tables"` shows all expected tables.
- `sqlite3 .ff/forgefix.db ".schema meta"` shows correct columns.
- Full suite: `go test ./... -count=1` green.
