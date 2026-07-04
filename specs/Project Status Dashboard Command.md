---
spec_id: "SPEC-1783157281"
status: backlog
repo_issue: 460
type: feature
version: "v0.9.0"
root_cause: ""
resolution: ""
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

1. Create `cmd_status.go` command handler
2. Aggregate data from ledger
3. Format output similar to existing TUI dashboard
4. Exit code 0 for healthy, non-zero for issues

## Acceptance Criteria

- [ ] `ff status` shows spec count breakdown by status
- [ ] `ff status` shows last sync timestamp
- [ ] `ff status` indicates if ship would be blocked
- [ ] Output is color-coded and human-readable

## Verification

```bash
ff status           # Shows dashboard
ff status --json    # (future) JSON output option
```
