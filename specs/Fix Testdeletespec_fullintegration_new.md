---
spec_id: "SPEC-1784255357"
status: ship
repo_issue: 542
type: bug
version: "0.9.6"
root_cause: "SaveLedger only wrote to SQLite when OpenDB succeeded, but did not mirror state to the legacy JSON ledger file. In TestDeleteSpec_FullIntegration the ledger is saved then reloaded via LoadLedger; the reload path read a store that did not reflect the persisted repo_issue_id (55), returning a stale/zero-derived value (42) instead of the expected 55."
resolution: "Commit ba00b4e added a JSON mirror (ledger.SaveToFile) inside SaveLedger before the SQLite write, keeping the legacy forgefix_ledger.json consistent with the canonical SQLite store. LoadLedger now reloads repo_issue_id=55 correctly and TestDeleteSpec_FullIntegration passes."
linked_commits: ["ba00b4e", "3338dbd"]
---
## Objective
Fix failing test `TestDeleteSpec_FullIntegration` (ledger_test.go) — `DeleteSpec` must return the persisted `repoID` (55) after a save/reload cycle.

## Root Cause
`SaveLedger` wrote only to SQLite when `OpenDB` succeeded and skipped mirroring to the legacy JSON ledger file. The reload path (`LoadLedger`) read a store that did not reflect the persisted `repo_issue_id` (55), so `DeleteSpec` returned a stale/zero-derived value (42) instead of the expected 55.

## Resolution
Commit `ba00b4e` added a JSON mirror (`ledger.SaveToFile`) inside `SaveLedger` before the SQLite write, keeping `forgefix_ledger.json` consistent with the canonical SQLite store. `LoadLedger` now reloads `repo_issue_id=55` correctly and the test passes.

## Failure Details
- Test: TestDeleteSpec_FullIntegration
- File: ledger_test.go
- Line: 451
- Error: expected repoID 55, got 42
- Status: RESOLVED — test passes on current HEAD.
