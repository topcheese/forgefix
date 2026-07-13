---
spec_id: "SPEC-1783929475"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: ["c166a4b"]
---
Now that forgefix.db has specs, linked_commits, and meta tables, the old forgefix_ledger.json is redundant. This spec migrates all remaining consumers to forgefix.db and removes the JSON ledger entirely.

## Objective

Remove `forgefix_ledger.json` as the active data store and migrate all remaining consumers to `forgefix.db`. The JSON ledger is currently still written to on every mutation (alongside the DB write), and ~82 `LoadLedger`/`SaveLedger` references exist. After this migration, the JSON file is no longer maintained and can be deleted.

## Requirements

- SC-001: All `LoadLedger` callers must read from `forgefix.db` instead
- SC-002: All `SaveLedger` callers must write to `forgefix.db` instead
- SC-003: The `forgefix.db` schema must cover all data that `forgefix_ledger.json` currently holds (specs, linked commits, status, metadata)
- SC-004: No data loss — migration path must copy existing JSON data to SQLite
- SC-005: No functional regression — all existing tests and `ff` commands must work after removing the JSON ledger
- SC-006: `forgefix_ledger.json` is no longer created, written, or read after migration

## Implementation

### Phase 1 (this commit) — Rewrite LoadLedger/SaveLedger to use SQLite

1. ✅ Rewrite `LoadLedger` to read from SQLite (pipeline_stats, specs, linked_commits, meta tables) with JSON fallback
2. ✅ Rewrite `SaveLedger` to write to SQLite via transaction (delete non-archived + re-insert)
3. ✅ Break circular dependency: `ImportLedger` now uses standalone `loadLedgerFromJSONFile()` instead of `LoadLedger()`
4. ✅ `ImportLedger` skips if DB already has data (one-time migration)
5. ✅ `CurrentVersion` in `version_manager.go` uses `LoadLedger` instead of direct file read
6. ✅ `SyncFromSpecsDir` always runs after loading to catch uncommitted specs
7. ✅ All engine tests pass (376 tests, 0 failures)

### Phase 2 — Clean up JSON artifacts

1. Remove `SaveToFile`/`LoadFromFile`/`LoadFromJSON` from `LedgerEngine`
2. Remove `ledgerPath()`, `FFLedgerPath`, `ffLedgerFile` constant
3. Remove `encoding/json` and `os` imports from `ledger.go`
4. Remove `forgefix_ledger.json` from `.gitignore` exclusion (if any)
5. Delete the `forgefix_ledger.json` file from existing projects (or leave as dead artifact)
6. Update integration tests to handle the safeguard prompt

## Acceptance Criteria

- [x] `go build ./...` compiles
- [x] All engine unit tests pass (ok ForgeFix/engine)
- [x] SC-004: Data migration preserves existing spec records via `ImportLedger` + `loadLedgerFromJSONFile`
- [x] SC-001: `LoadLedger` callers read from SQLite
- [x] SC-002: `SaveLedger` callers write to SQLite
- [ ] No references to `forgefix_ledger.json` remain in `engine/*.go` (except gitignored historical mention)
- [ ] No `LoadLedger` or `SaveLedger` function exists
- [ ] All `ff` subcommands (spec, commit, sync, ship, archive, specs, status) pass tests
- [ ] `forgefix_ledger.json` can be deleted without breaking any functionality
