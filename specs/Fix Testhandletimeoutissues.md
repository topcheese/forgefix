---
spec_id: "SPEC-1784089531"
status: draft
repo_issue: ""
type: bug
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: []
---
## Objective
Automatically created from failing test TestHandleTimeoutIssues during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestHandleTimeoutIssues
- File: issue_coordinator_test.go
- Line: 0
- Error: === RUN   TestHandleTimeoutIssues
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    issue_coordinator_test.go:1117: Expected 1 queued operation, got 0
--- FAIL: TestHandleTimeoutIssues (0.00s)

