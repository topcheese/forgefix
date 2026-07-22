---
spec_id: "SPEC-1784743300"
status: draft
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# Fix Testissuetitlevalidator_trailingpunctuation
## Objective
Automatically created from failing test TestIssueTitleValidator_TrailingPunctuation during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestIssueTitleValidator_TrailingPunctuation
- File: issue_validator_test.go
- Line: 0
- Error: === RUN   TestIssueTitleValidator_TrailingPunctuation
    issue_validator_test.go:127: Validate("feat/engine: add new feature.") = nil, want error for trailing punctuation
    issue_validator_test.go:127: Validate("fix/sync: resolve bug!") = nil, want error for trailing punctuation
    issue_validator_test.go:127: Validate("docs/config: update guide?") = nil, want error for trailing punctuation
    issue_validator_test.go:127: Validate("refactor/engine: simplify code...") = nil, want error for trailing punctuation
    issue_validator_test.go:127: Validate("feat/engine: test...") = nil, want error for trailing punctuation
--- FAIL: TestIssueTitleValidator_TrailingPunctuation (0.00s)

