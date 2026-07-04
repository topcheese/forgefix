---
id: SPEC-1783159999
type: fix
status: in-progress
version: "0.8.0"
---

# Fix version management to use ledger JSON directly

## Problem

The `readProjectVersion` and `writeProjectVersion` functions in `engine/execute.go` were creating a `.ff/version` file as a second source of truth. The `LedgerEngine` struct also had a `ProjectVersion` field added as a third source of truth. This created multiple places where the project version could diverge.

The `ff ship` command needs a single, authoritative location for the project version. The ledger file (`forgefix_ledger.json`) already has a `version` field at the root level that was being ignored in favor of these extra files/fields.

## Solution

1. Removed the `.ff/version` file concept entirely (deleted `projectVersionPath()` function).
2. Removed the `ProjectVersion` field from `LedgerEngine` struct (reverted `ledger.go` to original).
3. Changed `readProjectVersion()` to read the `"version"` field directly from `.ff/forgefix_ledger.json`.
4. Changed `writeProjectVersion()` to read the ledger JSON, update the `"version"` field, and write it back.

Now the project version lives exclusively in `forgefix_ledger.json`, which is the single source of truth. No extra files, no extra struct fields.

## Files Changed

- `engine/execute.go` — `readProjectVersion()` and `writeProjectVersion()` now read/write ledger JSON directly.
- `.ff/forgefix_ledger.json` — version field is the authoritative source.

## Verification

- `TMPDIR=/tmp go test ./...` passes.
- `.ff/forgefix_ledger.json` contains `"version": "0.8.0"`.
