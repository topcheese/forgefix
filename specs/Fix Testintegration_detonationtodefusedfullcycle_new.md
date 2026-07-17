---
spec_id: "SPEC-1784255477"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "handleDetonationIssues was called with an empty configDir string in some tests, causing writeSpecFromTemplate to fail silently (template file not found). The spec creation failed, so no issue was queued and IssueRefs stayed empty."
resolution: "Fixed in commit f84bb9c (SPEC-1784101189) by adding template directory creation and passing the correct configDir to handleDetonationIssues. The test now passes."
linked_commits: ["f84bb9c"]
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

## Resolution
The issue was fixed in commit f84bb9c (SPEC-1784101189). The root cause was that `handleDetonationIssues` was being called with an empty `configDir` in some tests, which caused `writeSpecFromTemplate` to fail silently because the spec template file wasn't found. The fix:
1. Added template directory creation with spec_template.md in the test setup
2. Changed `handleDetonationIssues(d, "")` to `handleDetonationIssues(d, tmpDir)` to pass the correct config directory

The test `TestIntegration_DetonationToDefusedFullCycle` in `execute_test.go` already had the correct template setup and passed `tmpDir` correctly, so it passes after the fix.
