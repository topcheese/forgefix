---
spec_id: "SPEC-1784742157"
status: review
repo_issue: 558
type: bug
version: "0.9.8"
root_cause: "resolveTargetStatus in cmd_commit.go returned 'draft' for AI non-chore commits (line 1109), so ff commit --ai never promoted specs to 'review'. promoteReviewSpecs in sync.go then found no 'review' entries in the ledger to promote to 'ship'."
linked_commits: ["62634a6"]
resolution: "Changed resolveTargetStatus to return 'review' instead of 'draft' for AI non-chore commits. Updated docstring and test."
---
# Fix Ff Sync Not Promoting Review Specs To Ship
## Objective
ff sync is supposed to promote specs from 'review' status to 'ship' status after successfully synchronizing with the remote issue tracker. Currently, specs are remaining in 'review' status even after a successful sync.

## Root Cause
`resolveTargetStatus` in `engine/cmd_commit.go` returned `"draft"` for AI non-chore commits (line 1109). This meant `ff commit --ai` set the spec to `"draft"` instead of `"review"`. When `ff sync --ai` later ran, `promoteReviewSpecs` in `engine/sync.go` searched the ledger for `"review"` entries and found none, so no specs were promoted to `"ship"`.

## Work Done
1. **Fixed `resolveTargetStatus`** in `engine/cmd_commit.go`: changed `target = "draft"` to `target = "review"` for AI non-chore commits.
2. **Updated docstring**: reflected that all AI commits now target `"review"`.
3. **Updated test**: `TestRunCommit_AIFeature_NoDowngradeFromInProgress` expects `"review"` (upgrade) instead of `"in-progress"` (no-change).

## Verification
- All tests pass: `go test ./...`
- `resolveTargetStatus` now returns `"review"` for all AI commits
- The `promoteReviewSpecs` function will find specs at `"review"` in the ledger and promote them to `"ship"`
