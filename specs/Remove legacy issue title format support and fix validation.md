---
spec_id: "SPEC-1784744046"
status: review
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: "issue_validator.go retained legacy 'type/component: Title' regex branch alongside new '[type][status] Title' format, and tests used old format titles; CreateIssue validated error-detection titles against spec format"
resolution: "Updated issueTitleRegex to only accept [type][status] Title format, removed allowedTypes and maxOldTitleLength, removed validation from CreateIssue, updated all tests to use new format"
linked_commits: []
---
# Remove Legacy Issue Title Format Support And Fix Validation
## Objective
The previous implementation incorrectly retained support for the legacy 'type/component: Title' format in the issue validator. The goal of SPEC-1784742186 was to unify on the new '[type][status] Title' format. This spec removes all legacy format support from the codebase.

## Requirements
1. Remove legacy format regex and logic from engine/issue_validator.go.
2. Ensure IsValidIssueTitle only accepts the new '[type][status] Title' format.
3. Update any tests that rely on the legacy format to use the new format.
4. Verify that all remote and local titles consistently use the new format.
