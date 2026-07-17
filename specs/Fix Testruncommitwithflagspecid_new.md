---
spec_id: "SPEC-1784255528"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: ""
resolution: ""
linked_commits: []
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
  create mode 100755 ff
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
