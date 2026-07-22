---
spec_id: "SPEC-1784742157"
status: ship
repo_issue: 558
type: bug
version: "0.9.8"
root_cause: "promoteReviewSpecs in sync.go was called AFTER the AutoIssueManagement gate in RunBackgroundSync. When auto_issue_management was false (default), the function returned before reaching promoteReviewSpecs, so specs at 'review' were never promoted to 'ship'. Additionally, syncSingleSpec didn't save the cleared repo_issue to the DB on 404, leaving stale references."
linked_commits: ["62634a6", "e21ae2f"]
resolution: "Moved promoteReviewSpecs before the AutoIssueManagement gate so it always runs. Added SaveLedger call in syncSingleSpec 404 path to persist cleared repo_issue to DB."
---
# Fix Ff Sync Not Promoting Review Specs To Ship
## Objective
ff sync is supposed to promote specs from 'review' status to 'ship' status after successfully synchronizing with the remote issue tracker. Currently, specs are remaining in 'review' status even after a successful sync.

## Root Cause
`promoteReviewSpecs` in `engine/sync.go` was placed at line 401 of `RunBackgroundSync` — **after** the `AutoIssueManagement` gate at line 343. When `auto_issue_management` was `false` (the default), the function returned early before ever reaching `promoteReviewSpecs`. The promotion code was effectively dead in the default config.

Separately, `syncSingleSpec` handled 404s from deleted remote issues by clearing `repo_issue` in the spec file, but never saved the change to the DB. The DB retained stale `repo_issue_id` values.

## Work Done
1. **Moved `promoteReviewSpecs`** before the `AutoIssueManagement` gate in `RunBackgroundSync`. This is a local operation (updates spec files + DB) and doesn't touch the remote, so it should not be gated by `AutoIssueManagement`.
2. **Added DB save in 404 path** in `syncSingleSpec`: after clearing `repo_issue` from the spec file, also updates the DB entry and calls `SaveLedger`.
3. **Wrote tests**: `TestPromoteReviewSpecs_PromotesReviewToShipInAiMode`, `TestPromoteReviewSpecs_SkipsNonReview`, `TestPromoteReviewSpecs_SkipsNonTerminal`.

## Verification
- All tests pass: `go test ./...`
- `promoteReviewSpecs` now runs before any gates, so `ff sync --ai` promotes review → ship regardless of `auto_issue_management` setting
- 404 reconciliation persists the cleared `repo_issue` to both spec file and DB
