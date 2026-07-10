---
spec_id: "SPEC-1783825678"
status: closed
repo_issue: 57
version: "0.9.8"
---

# Fix ledger sync — reconcile spec file statuses with forgefix ledger

## Goal
When a spec file's frontmatter `status:` is changed (e.g. `closed` → `ship`), the forgefix ledger (`forgefix_ledger.json`) is not updated to match. The ledger stays stale.

## Root Cause
`SyncFromSpecsDir` in `ledger.go` has two defects:

1. **New-entries only** — line 464 checks `if le.specMappings[specID] == nil`, so it only **creates** entries for specs not already in the ledger. It never **updates** existing entries when the file's frontmatter changes.
2. **Closed-spec skip** — line 460-462 skips any spec with `status: closed`, so reopening a closed spec never syncs back to the ledger.

Additionally, `RunBackgroundSync` (the `ff sync` handler) returns `nil` immediately when no GitHub config is present — it never reconciles the ledger at all.

## Fix

1. **`ledger.go`** — `SyncFromSpecsDir`: remove the `closed` filter and change the logic to always update the status field, not just for new entries
2. **`sync.go`** — `RunBackgroundSync`: call ledger reconciliation from spec files before returning, regardless of GitHub config availability

## Acceptance Criteria
- [ ] Changing a spec file's `status:` and running `ff sync` updates the ledger
- [ ] `SyncFromSpecsDir` updates existing entries' status, not just creates new ones
- [ ] `ff sync` reconciles ledger even when remote NAS is unreachable
