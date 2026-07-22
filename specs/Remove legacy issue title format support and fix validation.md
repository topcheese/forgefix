---
spec_id: "SPEC-1784744046"
status: ship
repo_issue: 650
type: bug
version: "0.9.8"
root_cause: "issue_validator.go retained legacy 'type/component: Title' regex branch alongside new '[type][status] Title' format, and tests used old format titles; CreateIssue validated error-detection titles against spec format"

resolution: fixed in 1cbc61f
linked_commits: ["62634a6", "382d2ab", "8410bbf", "38eac89"]
---
# Remove Legacy Issue Title Format Support And Fix Validation
## Objective
The previous implementation incorrectly retained support for the legacy 'type/component: Title' format in the issue validator. The goal of SPEC-1784742186 was to unify on the new '[type][status] Title' format. This spec removes all legacy format support from the codebase.

## Work Done
1. **Removed legacy regex branch**: `issueTitleRegex` changed from accepting both `^(\[.+\])+ .+|^(feat|fix|...)/component: .+$` to only `^(\[[a-z][a-z0-9-]*\])+ .+$`.
2. **Removed dead code**: Deleted `allowedTypes` map and `maxOldTitleLength` constant.
3. **Removed validation from CreateIssue**: Error-detection issue titles (e.g., `feat/test: TestAddition`) are not spec titles and shouldn't be format-checked.
4. **Updated tests**: All validator tests converted from `feat/engine: title` to `[feat][draft] title`. Sync stress tests updated to use valid format titles. Sync tests fixed to use `status: <val>` (no quotes) for correct YAML parsing.

## Verification
- `go test ./...` passes
- Legacy format `feat/engine: add feature` is rejected
- New format `[feat][draft] add feature` is accepted
- Titles with trailing punctuation, empty, or malformed brackets are rejected
