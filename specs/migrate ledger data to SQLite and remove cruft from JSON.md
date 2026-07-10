---
spec_id: "SPEC-1783714300"
status: ship
repo_issue: 518
type: refactor
version: "0.9.0"
root_cause: "forgefix_ledger.json stores version, pipeline stats, and spec_mappings in a single JSON file — hard to query, no referential integrity, migrations are manual"
resolution: ""
---
# Migrate Ledger Data To SQLite And Remove Cruft From JSON

## Objective

Move `spec_mappings`, `version` tracking, and pipeline `entries` from
`forgefix_ledger.json` into the SQLite database. Keep the JSON ledger for
backward compatibility during the transition, but eventually remove cruft
fields like `version` and `entries` from it. The DB becomes the source of
truth for spec metadata, version tracking, and pipeline statistics.

## Requirements

1. Add migration `002` to the SQLite schema:
   - `meta` table: key-value store for project metadata (version, last_update).
   - `pipeline_stats` table: pipeline_id, total_ran, total_passed,
     total_failed, historical_floor, last_update.
   - Bulk-import existing ledger data into these tables on migration.
2. All spec reads/writes that currently go through `LedgerEngine` should
   eventually use the `specs` table (already created in migration `001`).
3. The JSON ledger continues to be written for now (dual-write) so that
   rollback is possible. A later step removes JSON writing entirely.
4. `version` field is removed from the JSON ledger — tracked only in the DB
   `meta` table and the `schema_version` table.
5. Pipeline stats (`entries` in the JSON) move to `pipeline_stats` table.

## Implementation

- Add migration `002` in `engine/db.go` with the new tables and import logic.
- Add helper methods on `DB`: `SetMeta(key, value)`, `GetMeta(key)`,
  `UpsertPipelineStats`, `GetPipelineStats`.
- Modify `LoadLedger` and `SaveLedger` to optionally read/write from the DB
  (dual-write: writes go to both JSON and DB; reads prefer DB when available).
- Remove the `version` field from `SaveLedger` writes (it's in the DB now).
- The `LedgerEngine` struct remains but its `Version` field becomes a DB read.

## Acceptance Criteria

- `ff sync --ai` writes spec data to both JSON and DB (dual-write).
- `ff specs` reads from DB when available, falls back to JSON.
- `version` no longer appears in `forgefix_ledger.json`.
- Pipeline stats are queryable from the `pipeline_stats` table.
- All existing tests pass without changes (JSON path still works).

## Verification

- Run `ff sync --ai`, then verify DB has matching data: `sqlite3 .ff/forgefix.db "SELECT COUNT(*) FROM specs"`.
- `grep version .ff/forgefix_ledger.json` returns nothing (field removed).
- `go test ./... -count=1` green.
