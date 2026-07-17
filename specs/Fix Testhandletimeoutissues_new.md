---
spec_id: "SPEC-1784255439"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleTimeoutIssues delegates to handleDetonationIssues, which was called with an empty configDir string in some tests, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir."
resolution: "Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. handleTimeoutIssues delegates to handleDetonationIssues, so it is fixed by the same change. The test now passes."
linked_commits: ["f84bb9c"]
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

## Resolution
The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. Since `handleTimeoutIssues` simply delegates to `handleDetonationIssues(d, configDir)` (engine/execute.go:455-457), the same fix resolves both:
1. Added template directory creation with spec_template.md in the test setup
2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory

The test `TestHandleTimeoutIssues` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.
