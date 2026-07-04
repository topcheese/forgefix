---
spec_id: "SPEC-1781317040"
status: ship
type: type/standardization
version: version/v0.8.0
repo_issue: ""
root_cause: "No validation existed for issue titles synced to remote repositories; titles were free-form text with no consistent format"
resolution: "Implemented IssueTitleValidator in engine/issue_validator.go enforcing format [TYPE]/[CATEGORY]: [DESC] with types feat/fix/docs/refactor/ops/chore. Validation is enforced in issue_coordinator.go and sync.go — non-conforming titles are auto-prefixed with feat/spec:. Added full test suite in issue_validator_test.go. Compatible with Paperclip integration via machine-parseable structured titles."
---

# Standardize Repo Issue Naming Convention

## Goal
Implement a consistent, machine-parseable, and human-readable naming convention for all issues tracked within the `repo` to improve maintainability.

## Design Requirements
- **Naming Schema:** Enforce the following format for all new issues:
  `[TYPE]/[CATEGORY]: [SHORT_DESCRIPTION]`
- **TYPE Definitions:** Use the following standardized prefixes:
  - `feat/`: New functionality.
  - `fix/`: Bug or defect resolution.
  - `docs/`: Documentation updates.
  - `refactor/`: Code restructuring.
  - `ops/`: Maintenance, operations, or infrastructure.
  - `chore/`: Routine tasks and dependency updates.
- **CATEGORY Definitions:** Lowercase alphanumeric with hyphens (e.g., `engine`, `config`, `sync`, `driver`, `spec`).
- **Formatting Constraints:**
  - `SHORT_DESCRIPTION` must use imperative mood.
  - No trailing punctuation.
  - Maximum length of 60 characters for the full title.

## Implementation
- `engine/issue_validator.go` — `IssueTitleValidator` struct with regex `^(feat|fix|docs|refactor|ops|chore)/[a-z0-9-]+: [^.!?]+$`, max 60 chars, no trailing punctuation.
- `engine/issue_coordinator.go` — Validation enforced at `CreateIssueWithBody` (line 702); non-conforming spec titles auto-formatted as `feat/spec: <title>` (lines 1606-1607, 1647-1648).
- `engine/sync.go` — Same auto-formatting for spec-to-issue sync (line 533-534).
- `engine/issue_validator_test.go` — 11 test functions covering valid/invalid types, missing category/colon, length limits, trailing punctuation, empty/whitespace, invalid category characters, and the `IsValidIssueTitle` helper.

## Acceptance Criteria
- [x] ForgeFix CLI validates new issue titles against the `[TYPE]/[CATEGORY]: [DESC]` regex pattern.
- [x] Automated helpers for creating issues through ForgeFix utilize this template.
- [x] Documentation updated to reflect the new issue naming standard.
- [x] Paperclip-compatible naming convention enforced via machine-parseable structured titles.

## Verification
```bash
go test ./engine/ -run TestIssueTitle -v
# 11 tests pass covering all validation scenarios
```
