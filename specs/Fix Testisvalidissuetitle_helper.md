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
# Fix Testisvalidissuetitle_helper
## Objective
Automatically created from failing test TestIsValidIssueTitle_Helper during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestIsValidIssueTitle_Helper
- File: issue_validator_test.go
- Line: 0
- Error: === RUN   TestIsValidIssueTitle_Helper
    issue_validator_test.go:190: IsValidIssueTitle("feat/engine: add feature") = false, want true
    issue_validator_test.go:190: IsValidIssueTitle("fix/sync: fix bug") = false, want true
    issue_validator_test.go:190: IsValidIssueTitle("docs/config: update") = false, want true
    issue_validator_test.go:190: IsValidIssueTitle("refactor/engine: clean") = false, want true
    issue_validator_test.go:190: IsValidIssueTitle("ops/engine: maintenance") = false, want true
    issue_validator_test.go:190: IsValidIssueTitle("chore/config: update deps") = false, want true
--- FAIL: TestIsValidIssueTitle_Helper (0.00s)

