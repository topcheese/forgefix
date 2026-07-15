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
Automatically created from failing test TestIntegration_DetonationToDefusedFullCycle during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestIntegration_DetonationToDefusedFullCycle
- File: execute_test.go
- Line: 0
- Error: === RUN   TestIntegration_DetonationToDefusedFullCycle
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
    execute_test.go:153: After detonation: IssueRefs len = 0, want 1
--- FAIL: TestIntegration_DetonationToDefusedFullCycle (0.00s)

