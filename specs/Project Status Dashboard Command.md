---
spec_id: "SPEC-1783157281"
status: in-progress
repo_issue: 460
type: feature
version: "0.8.2"
root_cause: "No central dashboard to quickly assess project health, spec status distribution, sync state, and ship-gate readiness."
resolution: "Created engine/cmd_status.go with handleStatus method on CommandDispatcher. Added 'status' case to command_dispatcher.go switch. Dashboard shows: spec counts by status (color-coded via buildColorMap), pipeline test stats, last sync time, pending sync failures, and ship-gate blocking specs. Also created engine/cmd_status_test.go with 5 test cases validating routing, blocking spec detection, sync failure display, healthy state, and spec count output."
---
# Project Status Dashboard Command

## Objective

Add `ff status` command to show an overview of project health including spec counts by status, sync status, and recent activity.

## Requirements

1. `ff status` shows summary dashboard
2. Display counts: total specs, by status (backlog, in-progress, review, ship, closed)
3. Show last sync time and any sync errors
4. Show any specs blocking ship (in backlog or in-progress when ship attempted)
5. Color-coded output for quick scanning

## Implementation

1. Created `engine/cmd_status.go` — `handleStatus()` method on `CommandDispatcher`
2. Added `case "status"` to switch in `command_dispatcher.go`
3. Data sources:
   - `LoadLedger(configDir)` → `GetAllSpecEntries()` for spec counts by status
   - `LoadSyncScheduleState(configDir)` → last sync timestamp
   - `HasPendingSyncFailures()` → pending sync error warnings
   - `checkBlockingSpecs()` → specs not in ship/closed status that block gate
4. Uses `buildColorMap()` from `cmd_list.go` for status colorization
5. Uses Unicode box-drawing for header, ANSI color for health indicators
6. Created `engine/cmd_status_test.go` with 5 tests:
   - `TestCommandDispatcher_StatusRoutesCorrectly` — basic routing
   - `TestCommandDispatcher_StatusWithBlockingSpecs` — ship gate blockers
   - `TestCommandDispatcher_StatusWithSyncFailures` — sync error display
   - `TestCommandDispatcher_StatusHealthy` — clear gate output
   - `TestCommandDispatcher_StatusDisplaysSpecCounts` — spec count totals

## Acceptance Criteria

- [x] `ff status` shows spec count breakdown by status
- [x] `ff status` shows last sync timestamp
- [x] `ff status` indicates if ship would be blocked
- [x] Output is color-coded and human-readable
- [x] 5 unit tests covering all states

## Verification

```bash
ff status           # Shows dashboard
ff status --json    # (future) JSON output option
```
