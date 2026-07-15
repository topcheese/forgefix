---
spec_id: "SPEC-1784143269"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: ""
resolution: ""
linked_commits: []
---
## Objective
Automatically created from failing test TestIntegration_MultiplePipelinesFailures during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestIntegration_MultiplePipelinesFailures
- File: execute_test.go
- Line: 0
- Error: === RUN   TestIntegration_MultiplePipelinesFailures
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    execute_test.go:265: Expected 3 issue refs, got 0
--- FAIL: TestIntegration_MultiplePipelinesFailures (0.00s)

