---
spec_id: "SPEC-1784101371"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "TestDeleteSpec_FullIntegration is a purely local test (NewLedgerEngine/SaveLedger/LoadLedger/DeleteSpec) with no GitHub API call. The 'GitHub API 404' failures referenced in the title were already fixed in commit f84bb9c (SPEC-1784101189), which addressed the detonation/defused/timeout issue-handling integration tests in execute_test.go and issue_coordinator_test.go — not this test."
resolution: "Verified already implemented. TestDeleteSpec_FullIntegration passes in isolation and as part of the full engine package suite (ok ForgeFix/engine). No code change required. The only GitHub API call in the delete path is DeleteSpecWithArchive (coord.GetIssueByNumber), exercised by separate TestDeleteSpec_ArchiveFirst_* tests, all of which pass."
linked_commits: ["f84bb9c"]
---
Fix TestDeleteSpec_FullIntegration failing on GitHub API 404

## Investigation

The spec asked to check whether this was already implemented before implementing.

**Finding: already implemented — no change needed.**

- `TestDeleteSpec_FullIntegration` (engine/ledger_test.go:429) is a local-only test. It calls `NewLedgerEngine`, `SaveLedger`, `LoadLedger`, and `DeleteSpec` — none of which perform any network I/O.
- The only network call in the delete path is `DeleteSpecWithArchive` (engine/ledger.go:498 → `coord.GetIssueByNumber`), which is covered by the separate `TestDeleteSpec_ArchiveFirst_*` tests. All pass.
- The "GitHub API 404" failures referenced in the title were resolved by commit `f84bb9c` (SPEC-1784101189), which fixed the detonation/defused/timeout issue-handling integration tests in `execute_test.go` and `issue_coordinator_test.go`.
- Current repo test failures are unrelated: `TestRunCommitWithFlagSpecID` (spec-status/ledger binding) and `TestSpecLifecycle` (`ff spec --ai` exits 1). Neither is a 404 and neither is this test.

**Verification:**
```
go test ./engine/ -run TestDeleteSpec -v   → all PASS
go test ./engine/                          → ok ForgeFix/engine
```
