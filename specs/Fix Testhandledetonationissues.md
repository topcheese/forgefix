---spec_id: "SPEC-1784143268"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues."
 "Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. The test now passes."
linked_commits: ["f84bb9c", "60507de"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 177df1a..41f52a6 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -4,6 +4,7 @@
   - feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)
   - feat: fix ff sync 404 reconciliation; unbind orphaned repo_issue and mirror ledger to JSON (SPEC-1784146853)
   - feat: fix SPEC-1784143269: document root cause/resolution for TestDeleteSpec_FullIntegration repoID mismatch (SPEC-1784143269)
  +- feat: Update spec SPEC-1784143268 with root cause and resolution for TestHandleDetonationIssues fix (SPEC-1784143268)
   
   ## [Unreleased] - 2026-07-15
   
  diff --git a/specs/Fix Testhandledetonationissues.md b/specs/Fix Testhandledetonationissues.md
  index 4a3c096..bf73055 100644
  --- a/specs/Fix Testhandledetonationissues.md	
  +++ b/specs/Fix Testhandledetonationissues.md	
  @@ -4,9 +4,9 @@ status: draft
   repo_issue: ""
   type: bug
   version: "0.9.6"
  -root_cause: ""
  -resolution: ""
  -linked_commits: []
  +root_cause: "handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues."
  +resolution: "Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. The test now passes."
  +linked_commits: ["f84bb9c"]
   ---
   ## Objective
   Automatically created from failing test TestHandleDetonationIssues during ff --ai run.
  @@ -24,3 +24,10 @@ Test failed - see failure details below.
       issue_coordinator_test.go:644: Expected 1 queued operation, got 0
   --- FAIL: TestHandleDetonationIssues (0.01s)
   
  +## Resolution
  +The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:
  +1. Added template directory creation with spec_template.md in the test setup
  +2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory
  +
  +The test `TestHandleDetonationIssues` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.
  +
  diff --git a/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md b/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md
  index 2557af2..b74bb68 100644
  --- a/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md	
  +++ b/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md	
  @@ -1,11 +1,28 @@
   ---
   spec_id: "SPEC-1784101371"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
  -root_cause: ""
  -resolution: ""
  -linked_commits: []
  +root_cause: "TestDeleteSpec_FullIntegration is a purely local test (NewLedgerEngine/SaveLedger/LoadLedger/DeleteSpec) with no GitHub API call. The 'GitHub API 404' failures referenced in the title were already fixed in commit f84bb9c (SPEC-1784101189), which addressed the detonation/defused/timeout issue-handling integration tests in execute_test.go and issue_coordinator_test.go — not this test."
  +resolution: "Verified already implemented. TestDeleteSpec_FullIntegration passes in isolation and as part of the full engine package suite (ok ForgeFix/engine). No code change required. The only GitHub API call in the delete path is DeleteSpecWithArchive (coord.GetIssueByNumber), exercised by separate TestDeleteSpec_ArchiveFirst_* tests, all of which pass."
  +linked_commits: ["f84bb9c"]
   ---
   Fix TestDeleteSpec_FullIntegration failing on GitHub API 404
  +
  +## Investigation
  +
  +The spec asked to check whether this was already implemented before implementing.
  +
  +**Finding: already implemented — no change needed.**
  +
  +- `TestDeleteSpec_FullIntegration` (engine/ledger_test.go:429) is a local-only test. It calls `NewLedgerEngine`, `SaveLedger`, `LoadLedger`, and `DeleteSpec` — none of which perform any network I/O.
  +- The only network call in the delete path is `DeleteSpecWithArchive` (engine/ledger.go:498 → `coord.GetIssueByNumber`), which is covered by the separate `TestDeleteSpec_ArchiveFirst_*` tests. All pass.
  +- The "GitHub API 404" failures referenced in the title were resolved by commit `f84bb9c` (SPEC-1784101189), which fixed the detonation/defused/timeout issue-handling integration tests in `execute_test.go` and `issue_coordinator_test.go`.
  +- Current repo test failures are unrelated: `TestRunCommitWithFlagSpecID` (spec-status/ledger binding) and `TestSpecLifecycle` (`ff spec --ai` exits 1). Neither is a 404 and neither is this test.
  +
  +**Verification:**
  +```
  +go test ./engine/ -run TestDeleteSpec -v   → all PASS
  +go test ./engine/                          → ok ForgeFix/engine
  +```
---
## Objective
Automatically created from failing test TestHandleDetonationIssues during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestHandleDetonationIssues
- File: issue_coordinator_test.go
- Line: 0
- Error: === RUN   TestHandleDetonationIssues
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    issue_coordinator_test.go:644: Expected 1 queued operation, got 0
--- FAIL: TestHandleDetonationIssues (0.01s)

## Resolution
The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:
1. Added template directory creation with spec_template.md in the test setup
2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory

The test `TestHandleDetonationIssues` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.

