---
spec_id: "SPEC-1783152969"
status: in-progress
repo_issue: 454
type: bug
root_cause: "`performReconciliation` in `issue_coordinator.go` identified orphaned remote issues (issues with no matching local spec) but only printed a report. It never closed them. This left stale open issues on the remote tracker that accumulated over time — e.g., 7 open issues on the remote but only 2 active local specs. Additionally, the reconciliation ran BEFORE the main sync loop, so issues that were about to be bound by title matching were incorrectly identified as orphans and would have been closed prematurely."
resolution: ""
---
# Fix Reconciliation To Close Orphaned Remote Issues Instead Of Just Reporting

## Objective
Automatically close remote issues that have no corresponding local spec, and prevent premature closure of issues that are about to be bound during sync.

## Requirements
1. Orphaned remote issues must be closed with an explanatory comment during sync.
2. Reconciliation must run AFTER the main sync loop so title-matched bindings are respected.
3. `ff ship` should warn the user to run `ff sync` to close remote issues.

## Implementation
- `engine/issue_coordinator.go` `performReconciliation`: For each orphaned issue, post a closure comment and call `CloseIssueByNumber`.
- `engine/issue_coordinator.go` `SyncSpecs`: Move `performReconciliation` call to the end of the function, after all specs have been synced and their `repo_issue` fields updated. Re-read the specs directory before reconciliation to pick up updated bindings.
- `engine/execute.go` `ShipReconciliation`: Add a message prompting the user to run `ff sync` to process housekeeping tasks and close remote issues.

## Acceptance Criteria
- [ ] Running `ff sync` closes orphaned remote issues and posts a closure comment.
- [ ] Issues matched by title during sync are NOT prematurely closed.
- [ ] `ff ship` prints a message telling the user to run `ff sync`.

## Verification