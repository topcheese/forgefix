---
spec_id: "SPEC-1783572691"
status: closed
repo_issue: 479
type: feature
version: "0.9.4"
root_cause: ""
resolution: ""
---
# Fix Spec Status Sync Across Commit Sync And Ship Lifecycle

## Objective

Ensure spec status transitions are consistently written to both disk (spec file frontmatter) and ledger across the commit, sync, and ship lifecycle stages, eliminating silent desync where one store is updated but not the other.

## Requirements

- `ff commit` must set status to "review" on both disk and ledger, not just ledger
- `ff sync` (promoteReviewSpecs) must write "ship" to both disk and ledger, not just ledger
- Post-push metadata sync must write "closed" to both ledger and disk, not just disk
- All transitions must use the frontmatter-safe updateSpecFileStatus helper instead of fragile strings.Replace
- The commit message must not double the [SPEC-XXXXX] prefix when the user's message already contains it

## Implementation

Three synchronization holes were identified and fixed:

1. **Hole B (cmd_commit.go: UpdateLedgerAfterCommit):** Changed status from "test" to "review", added updateSpecFileStatus call for disk write, swapped call order in handleCommit so the blocking disk+ledger write happens before the async SpawnBackgroundSync goroutine.

2. **Hole C (sync.go: promoteReviewSpecs):** Changed status check from "test" to "review", replaced per-spec interactive prompts with a single batch list UI, added updateSpecFileStatus call alongside each ledger update.

3. **SyncMetadata (sync.go: specMetadataSyncer.SyncMetadata):** Replaced fragile strings.Replace approach (which could match body content and didn't handle quote variants) with updateSpecFileStatus for disk writes plus a ledger update to "closed".

4. **Message dedup (cmd_commit.go: runCommit):** Added regexp-based stripping of existing [SPEC-XXXXX] prefix from the commit message before formatting to prevent doubling.

## Acceptance Criteria

- After `ff commit`, spec file on disk shows status: review and ledger shows "review"
- After `ff sync` promotion prompt, both disk and ledger show "ship"
- After `ff ship` and housekeeping SyncMetadata, both disk and ledger show "closed"
- `ff commit "feat: [SPEC-X] msg"` produces "feat: [SPEC-X] msg" not doubled
- All tests pass (go test ./...)

## Verification

- `go test ./... -count=1` passes all 4 modules (ForgeFix, engine, housekeeper, tests)
- `go vet ./...` clean
- Integration test TestSpecLifecycle covers the full spec→commit→sync flow

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->