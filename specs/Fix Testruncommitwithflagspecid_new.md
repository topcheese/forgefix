---spec_id: "SPEC-1784255528"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "TestRunCommitWithFlagSpecID asserted that a plain ff commit auto-promotes a spec to status: review. The intended design (established in commit db296db) is that a plain commit does NOT auto-promote — promotion to review is explicit/human-gated via the --review flag (ff commit --ai --review). The test's assertions were stale and contradicted the intended lifecycle."
 "Fixed the test, not the code. UpdateLedgerAfterCommit correctly records the commit hash in linked_commits and leaves status unchanged (no auto-promotion). The test now asserts the spec stays at its pre-commit status (in-progress) after a plain commit, and that linked_commits is populated. Promotion to review remains available explicitly via ff commit --ai --review (forceReview path in runCommit)."
linked_commits: ["3a3d43f"]
resolution: |
  diff --git a/.ff/forgefix_ledger.json b/.ff/forgefix_ledger.json
  index 1bcb35f..ed185b9 100644
  --- a/.ff/forgefix_ledger.json
  +++ b/.ff/forgefix_ledger.json
  @@ -6,7 +6,7 @@
         "total_passed": 437,
         "total_failed": 7,
         "historical_floor": 437,
  -      "last_update": "2026-07-17 02:39:44"
  +      "last_update": "2026-07-16T19:40:32-07:00"
       }
     },
     "spec_mappings": {
  @@ -309,7 +309,7 @@
         "type": "bug",
         "body": "a/.ff/forgefix_ledger.json\n  +++ b/.ff/forgefix_ledger.json\n  @@ -3,10 +3,10 @@\n       \"forgefix\": {\n         \"pipeline_id\": \"forgefix\",\n         \"total_ran\": 444,\n  -      \"total_passed\": 436,\n  -      \"total_failed\": 8,\n  -      \"historical_floor\": 436,\n  -      \"last_update\": \"2026-07-16 20:59:42\"\n  +      \"total_passed\": 437,\n  +      \"total_failed\": 7,\n  +      \"historical_floor\": 437,\n  +      \"last_update\": \"2026-07-16T19:37:14-07:00\"\n       }\n     },\n     \"spec_mappings\": {\n  @@ -288,14 +288,14 @@\n       \"SPEC-1784143268\": {\n         \"spec_id\": \"SPEC-1784143268\",\n         \"repo_issue_id\": 0,\n  -      \"status\": \"draft\",\n  +      \"status\": \"review\",\n         \"linked_commits\": [\n           \"60507de\"\n         ],\n         \"type\": \"bug\",\n  -      \"body\": \"## Objective\\nAutomatically created from failing test TestHandleDetonationIssues_ExistingIssue during ff --ai run.\\n\\n## Root Cause\\nTest failed - see failure details below.\\n\\n## Failure Details\\n- Test: TestHandleDetonationIssues_ExistingIssue\\n- File: issue_coordinator_test.go\\n- Line: 0\\n- Error: === RUN   TestHandleDetonationIssues_ExistingIssue\\n[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false\\n[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false\\n    issue_coordinator_test.go:699: Expected 1 queued operation, got 0\\n--- FAIL: TestHandleDetonationIssues_ExistingIssue (0.00s)\",\n  +      \"body\": \"a/CHANGELOG.md\\n  +++ b/CHANGELOG.md\\n  @@ -4,6 +4,7 @@\\n   - feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)\\n   - feat: fix ff sync 404 reconciliation; unbind orphaned repo_issue and mirror ledger to JSON (SPEC-1784146853)\\n   - feat: fix SPEC-1784143269: document root cause/resolution for TestDeleteSpec_FullIntegration repoID mismatch (SPEC-1784143269)\\n  +- feat: Update spec SPEC-1784143268 with root cause and resolution for TestHandleDetonationIssues fix (SPEC-1784143268)\\n   \\n   ## [Unreleased] - 2026-07-15\\n   \\n  diff --git a/specs/Fix Testhandledetonationissues.md b/specs/Fix Testhandledetonationissues.md\\n  index 4a3c096..bf73055 100644\\n  --- a/specs/Fix Testhandledetonationissues.md\\t\\n  +++ b/specs/Fix Testhandledetonationissues.md\\t\\n  @@ -4,9 +4,9 @@ status: draft\\n   repo_issue: \\\"\\\"\\n   type: bug\\n   version: \\\"0.9.6\\\"\\n  -root_cause: \\\"\\\"\\n  -resolution: \\\"\\\"\\n  -linked_commits: []\\n  +root_cause: \\\"handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues.\\\"\\n  +resolution: \\\"Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. The test now passes.\\\"\\n  +linked_commits: [\\\"f84bb9c\\\"]\\n   ---\\n   ## Objective\\n   Automatically created from failing test TestHandleDetonationIssues during ff --ai run.\\n  @@ -24,3 +24,10 @@ Test failed - see failure details below.\\n       issue_coordinator_test.go:644: Expected 1 queued operation, got 0\\n   --- FAIL: TestHandleDetonationIssues (0.01s)\\n   \\n  +## Resolution\\n  +The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:\\n  +1. Added template directory creation with spec_template.md in the test setup\\n  +2. Changed `handleDetonationIssues(d, \\\"\\\")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory\\n  +\\n  +The test `TestHandleDetonationIssues` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.\\n  +\\n  diff --git a/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md b/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md\\n  index 2557af2..b74bb68 100644\\n  --- a/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md\\t\\n  +++ b/specs/fix TestDeleteSpec_FullIntegration failing on GitHub API 404.md\\t\\n  @@ -1,11 +1,28 @@\\n   ---\\n   spec_id: \\\"SPEC-1784101371\\\"\\n  -status: draft\\n  +status: review\\n   repo_issue: \\\"\\\"\\n   type: bug\\n   version: \\\"0.9.6\\\"\\n  -root_cause: \\\"\\\"\\n  -resolution: \\\"\\\"\\n  -linked_commits: []\\n  +root_cause: \\\"TestDeleteSpec_FullIntegration is a purely local test (NewLedgerEngine/SaveLedger/LoadLedger/DeleteSpec) with no GitHub API call. The 'GitHub API 404' failures referenced in the title were already fixed in commit f84bb9c (SPEC-1784101189), which addressed the detonation/defused/timeout issue-handling integration tests in execute_test.go and issue_coordinator_test.go — not this test.\\\"\\n  +resolution: \\\"Verified already implemented. TestDeleteSpec_FullIntegration passes in isolation and as part of the full engine package suite (ok ForgeFix/engine). No code change required. The only GitHub API call in the delete path is DeleteSpecWithArchive (coord.GetIssueByNumber), exercised by separate TestDeleteSpec_ArchiveFirst_* tests, all of which pass.\\\"\\n  +linked_commits: [\\\"f84bb9c\\\"]\\n   ---\\n   Fix TestDeleteSpec_FullIntegration failing on GitHub API 404\\n  +\\n  +## Investigation\\n  +\\n  +The spec asked to check whether this was already implemented before implementing.\\n  +\\n  +**Finding: already implemented — no change needed.**\\n  +\\n  +- `TestDeleteSpec_FullIntegration` (engine/ledger_test.go:429) is a local-only test. It calls `NewLedgerEngine`, `SaveLedger`, `LoadLedger`, and `DeleteSpec` — none of which perform any network I/O.\\n  +- The only network call in the delete path is `DeleteSpecWithArchive` (engine/ledger.go:498 → `coord.GetIssueByNumber`), which is covered by the separate `TestDeleteSpec_ArchiveFirst_*` tests. All pass.\\n  +- The \\\"GitHub API 404\\\" failures referenced in the title were resolved by commit `f84bb9c` (SPEC-1784101189), which fixed the detonation/defused/timeout issue-handling integration tests in `execute_test.go` and `issue_coordinator_test.go`.\\n  +- Current repo test failures are unrelated: `TestRunCommitWithFlagSpecID` (spec-status/ledger binding) and `TestSpecLifecycle` (`ff spec --ai` exits 1). Neither is a 404 and neither is this test.\\n  +\\n  +**Verification:**\\n  +```\\n  +go test ./engine/ -run TestDeleteSpec -v   → all PASS\\n  +go test ./engine/                          → ok ForgeFix/engine\\n  +```\\n---\\n## Objective\\nAutomatically created from failing test TestHandleDetonationIssues during ff --ai run.\\n\\n## Root Cause\\nTest failed - see failure details below.\\n\\n## Failure Details\\n- Test: TestHandleDetonationIssues\\n- File: issue_coordinator_test.go\\n- Line: 0\\n- Error: === RUN   TestHandleDetonationIssues\\n[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false\\n[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false\\n    issue_coordinator_test.go:644: Expected 1 queued operation, got 0\\n--- FAIL: TestHandleDetonationIssues (0.01s)\\n\\n## Resolution\\nThe issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:\\n1. Added template directory creation with spec_template.md in the test setup\\n2. Changed `handleDetonationIssues(d, \\\"\\\")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory\\n\\nThe test `TestHandleDetonationIssues` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.\",\n         \"root_cause\": \"handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues.\",\n  -      \"resolution\": \"diff --git a/CHANGELOG.md b/CHANGELOG.md\\nindex 177df1a..41f52a6 100644\\n--- a/CHANGELOG.md\\n+++ b/CHANGELOG.md\\n@@ -4,6 +4,7 @@\\n - feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)\\n - feat: fix ff sync 404 reconciliation; unbind orphaned repo_issue and mirror ledger to JSON (SPEC-1784146853)\\n - feat: fix SPEC-1784143269: document root cause/resolution for TestDeleteSpec_FullIntegration repoID mismatch (SPEC-1784143269)\\n+- feat: Update spec SPEC-1784143268 with root cause and resolution for TestHandleDetonationIssues fix (SPEC-1784143268)\\n \\n ## [Unreleased] - 2026-07-15\\n \\ndiff --git a/specs/Fix Testhandledetonationissues.md b/specs/Fix Testhandledetonationissues.md\\nindex 4a3c096..bf73055 100644\\n--- a/specs/Fix Testhandledetonationissues.md\\t\\n+++ b/specs/Fix Testhandledetonationissues.md\\t\\n@@ -4,9 +4,9 @@ status: draft\\n repo_issue: \\\"\\\"\\n type: bug\\n version: \\\"0.9.6\\\"\\n-root_cause: \\\"\\\"\\n-resolution: \\\"\\\"\\n-linked_commits: []\\n+root_cause: \\\"handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues.\\\"\\n+resolution: \\\"Fixed in commit... [truncated at 10KB]
---
## Objective
Automatically created from failing test TestRunCommitWithFlagSpecID during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestRunCommitWithFlagSpecID
- File: main_test.go
- Line: 0
- Error: === RUN   TestRunCommitWithFlagSpecID
[main (root-commit) b20454c] feat: [SPEC-123] test commit message
  5 files changed, 23 insertions(+)
  create mode 100644 .ff/forgefix_ledger.json
  create mode 100644 001_ff.yaml
  create mode 100644 ff
  create mode 100644 specs/SPEC-123.md
  create mode 100644 test.txt
    main_test.go:356: expected review status after commit, got: in-progress
    main_test.go:362: expected spec file to contain status: review, got:
        ---
        spec_id: "SPEC-123"
        status: in-progress
        type: feature
        repo_issue: ""
        created: 2024-01-01
        linked_commits: ["b20454c"]
        linked_commits: ["b20454c"]
        # Test Spec
--- FAIL: TestRunCommitWithFlagSpecID (0.35s)

## Resolution / Proposed Change (for review)

**Diagnosis:** The test was stale, not the code. The intended ForgeFix lifecycle is:
- `ff commit --ai` → commits, binds the spec, records the commit in `linked_commits`, and leaves the spec at its current status (no auto-promotion). A human then reviews the file and manually promotes it to `review`.
- `ff commit --ai --review` (or `--spec X --review`) → explicitly promotes the spec to `status: review` via the `forceReview` path in `runCommit` (engine/cmd_commit.go:258-271).

This explicit-promotion design was introduced in commit `db296db` ("Fix ff commit --ai auto-detect forcing wrong spec to review"), which **intentionally removed** the auto-promotion from `UpdateLedgerAfterCommit` (it previously set `entry.Status = "review"` + `UpdateSpecFileStatus(specFile, "review")`). The removal was correct: it prevents specs from being accidentally flipped to `review`/`ship` without human intervention.

**Change made:** Updated `TestRunCommitWithFlagSpecID` (main_test.go) to match the intended design:
- Before: asserted `specEntry.Status == "review"` and that the spec file contains `status: review` after a plain `UpdateLedgerAfterCommit`.
- After: asserts the status **remains** `in-progress` (no auto-promotion on a plain commit) and that `linked_commits` is populated. A negative assertion confirms the file does NOT contain `status: review`.

**Verification:** `go test . -run TestRunCommitWithFlagSpecID` → PASS. Full `ForgeFix` and `ForgeFix/engine` packages pass. (The unrelated `TestSpecLifecycle` in `ForgeFix/tests` was already failing before this change and is out of scope for this spec.)

**Note on spec_id collision:** This spec was previously entangled in a duplicate-`spec_id` collision. The rogue duplicate file `Fix Testruncommitwithflagspecid.md` (which wrongly shared SPEC-1784256032 with `Fix Testspeclifecycle.md`) has been deleted. This file (SPEC-1784255528) is the canonical TestRunCommitWithFlagSpecID spec.
