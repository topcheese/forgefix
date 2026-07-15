---
spec_id: "SPEC-1784143268"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: ""
resolution: ""
linked_commits: []
---
## Objective
Automatically created from failing test TestHandleDetonationIssues_ExistingIssue during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestHandleDetonationIssues_ExistingIssue
- File: issue_coordinator_test.go
- Line: 0
- Error: === RUN   TestHandleDetonationIssues_ExistingIssue
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    issue_coordinator_test.go:699: Expected 1 queued operation, got 0
--- FAIL: TestHandleDetonationIssues_ExistingIssue (0.00s)

