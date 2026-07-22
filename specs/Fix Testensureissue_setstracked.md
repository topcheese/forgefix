---
spec_id: "SPEC-1784743251"
status: draft
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# Fix Testensureissue_setstracked
## Objective
Automatically created from failing test TestEnsureIssue_SetsTracked during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestEnsureIssue_SetsTracked
- File: issue_coordinator_test.go
- Line: 0
- Error: === RUN   TestEnsureIssue_SetsTracked
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    issue_coordinator_test.go:603: EnsureIssue() error = issue title validation failed: regex mismatch for "feat/test: TrackedTest": invalid issue title format
--- FAIL: TestEnsureIssue_SetsTracked (0.00s)

