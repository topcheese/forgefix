---
spec_id: "SPEC-1783716117"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: "Spec storage is hardcoded to the JSON ledger file with no option to switch to SQLite, making the DB migration impossible without a config toggle"
resolution: ""
---
# Add Config Option For Spec Storage Backend: SQLite, File, Or Both

## Objective

Add a `spec_storage` configuration key to `_ff.yaml` that controls where spec
metadata is stored: the JSON ledger file (`file`), the SQLite database
(`sqlite`), or both (`both` for dual-write during migration). Default is
`file` for full backward compatibility.

## Requirements

1. New config key: `spec_storage: file | sqlite | both` in `_ff.yaml`.
2. `file` — current behavior: all reads/writes go through `LedgerEngine` to
   `forgefix_ledger.json`. DB is not used.
3. `sqlite` — all reads/writes go through the `DB` struct to `.ff/forgefix.db`.
   JSON ledger is not written.
4. `both` — writes go to both DB and JSON ledger. Reads prefer DB, fall back
   to file. Used during migration to ensure data consistency.
5. Config is parsed in `LoadPipelineConfig` and stored in the `Config` struct.
6. `LedgerEngine` and `DB` check the storage mode and behave accordingly.

## Implementation

- Add `SpecStorage` field to the `Config` struct (and YAML tag).
- Default value is `"file"` if not specified.
- In `LoadPipelineConfig`, parse the value and validate it's one of the three
  allowed values.
- `LedgerEngine` checks the config on write: if `sqlite` or `both`, also write
  to DB. On read: if `sqlite` or `both`, prefer DB.
- The `DB` struct gets a `StorageMode()` method that returns the current mode.
- No changes to existing code paths when mode is `file` (default).

## Acceptance Criteria

- `spec_storage: file` — everything works exactly as before.
- `spec_storage: sqlite` — `ff spec --ai` writes to DB, not JSON.
- `spec_storage: both` — both DB and JSON are written; reads come from DB.
- Invalid values produce a clear error at startup.
- All existing tests pass with default `file` mode.

## Verification

- Unit test: `TestConfigSpecStorageDefault` verifies `file` default.
- Unit test: `TestConfigSpecStorageInvalid` errors on bad value.
- Integration: set `spec_storage: sqlite`, run `ff spec --ai`, verify `.ff/forgefix.db` has the spec.
- Full suite: `go test ./... -count=1` green.
