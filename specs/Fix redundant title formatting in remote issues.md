---
spec_id: "SPEC-1784742186"
status: ship
repo_issue: 559
type: bug
version: "0.9.8"
root_cause: "prefixedTitle in issue_coordinator.go produced 'feat/spec: [type][status] Title' with redundant prefix, while issue_validator.go accepted legacy 'type/component: Title' format alongside new format"
linked_commits: ["25724ea", "03063fd"]
resolution: "Removed feat/spec prefix from prefixedTitle, updated issueTitleRegex to only accept [type][status] Title format, removed allowedTypes and maxOldTitleLength, updated all tests"
---
# Fix Redundant Title Formatting In Remote Issues

## Objective
Remote issue titles currently include a redundant prefix like 'feat/spec: [type][status] Title #123'. The 'feat/spec:' part is unnecessary and clutters the title. Additionally, the spec titles stored in the local database and files should consistently use the same '[type][status] Title' naming scheme to match the remote repository.

## Root Cause
The title generation logic in `issue_coordinator.go` `prefixedTitle()` prepended a hardcoded `feat/spec:` label, and `issue_validator.go` accepted both the legacy `type/component: Title` format and the new `[type][status] Title` format, allowing inconsistency.

## Work Done
1. **Removed redundant prefix**: `prefixedTitle()` now produces `[type][status] Title` without `feat/spec:` prefix.
2. **Unified validation**: `issueTitleRegex` updated to `^(\[[a-z][a-z0-9-]*\])+ .+$` — only accepts new format, rejects old `type/component: Title` format.
3. **Removed dead code**: Deleted `allowedTypes` map and `maxOldTitleLength` from `issue_validator.go`.
4. **Updated tests**: All validator tests (`issue_validator_test.go`) converted to new format; sync tests (`sync_test.go`, `sync_stress_test.go`) updated for clean type values in frontmatter; `CreateIssue` in `issue_coordinator.go` no longer validates error-detection titles against spec format.
5. **Cleaned spec linked_commits**: Removed 8 stale sync-bookkeeping commits, retained only the 2 actual fix commits.

## Verification
- All tests pass: `go test ./...`
- Legacy formats like `feat/engine: add feature` are rejected
- New format like `[feat][draft] add feature` is accepted
- Titles with trailing punctuation, empty, or malformed brackets are rejected
