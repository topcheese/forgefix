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

