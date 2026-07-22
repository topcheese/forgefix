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
# Fix Testissuetitlevalidator_validtypes
## Objective
Automatically created from failing test TestIssueTitleValidator_ValidTypes during ff --ai run.

## Root Cause
Test failed - see failure details below.

## Failure Details
- Test: TestIssueTitleValidator_ValidTypes
- File: issue_validator_test.go
- Line: 0
- Error: === RUN   TestIssueTitleValidator_ValidTypes
    issue_validator_test.go:25: Validate("feat/engine: add new dashboard renderer") = regex mismatch for "feat/engine: add new dashboard renderer": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("fix/sync: resolve issue closing logic") = regex mismatch for "fix/sync: resolve issue closing logic": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("docs/config: update setup guide") = regex mismatch for "docs/config: update setup guide": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("refactor/engine: simplify issue coordinator") = regex mismatch for "refactor/engine: simplify issue coordinator": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("feat/config: add multi-backend support") = regex mismatch for "feat/config: add multi-backend support": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("fix/driver: handle 404 errors") = regex mismatch for "fix/driver: handle 404 errors": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("ops/engine: maintenance cleanup") = regex mismatch for "ops/engine: maintenance cleanup": invalid issue title format, want nil
    issue_validator_test.go:25: Validate("chore/config: update dependencies") = regex mismatch for "chore/config: update dependencies": invalid issue title format, want nil
--- FAIL: TestIssueTitleValidator_ValidTypes (0.00s)

