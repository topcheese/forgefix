---spec_id: "SPEC-1784255528"
status: draft
repo_issue: 545
type: bug
version: "0.9.6"
root_cause: "TestRunCommitWithFlagSpecID asserted that a plain ff commit auto-promotes a spec to status: review. The intended design (established in commit db296db) is that a plain commit does NOT auto-promote — promotion to review is explicit/human-gated via the --review flag (ff commit --ai --review). The test's assertions were stale and contradicted the intended lifecycle."
linked_commits: ["3a3d43f"]
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
