---
spec_id: "SPEC-1784254766"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued. The fix was to ensure the template directory exists and pass the correct configDir to handleDetonationIssues."
resolution: "Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. The test now passes."
linked_commits: ["f84bb9c"]
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

## Resolution
The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:
1. Added template directory creation with spec_template.md in the test setup
2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory

The test `TestHandleDetonationIssues_ExistingIssue` in `issue_coordinator_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.
