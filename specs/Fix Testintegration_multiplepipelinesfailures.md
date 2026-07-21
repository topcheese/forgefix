---spec_id: "SPEC-1784143269"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string in some tests, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued and IssueRefs stayed empty (Expected 3 issue refs, got 0)."
linked_commits: ["f84bb9c", "f989c31"]
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

## Resolution
The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:
1. Added template directory creation with spec_template.md in the test setup
2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory

The test `TestIntegration_MultiplePipelinesFailures` in `execute_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.
