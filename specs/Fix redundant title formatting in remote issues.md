---
spec_id: "SPEC-1784742186"
status: draft
repo_issue: 559
type: bug
version: "0.9.8"
root_cause: ""





linked_commits: ["25724ea", "66a78ae", "a633f01", "3eceff4", "03063fd"]
resolution: fixed in 4102e00
---
# Fix Redundant Title Formatting In Remote Issues

## Objective
Remote issue titles currently include a redundant prefix like 'feat/spec: [type][status] Title #123'. The 'feat/spec:' part is unnecessary and clutters the title. Additionally, the spec titles stored in the local database and files should consistently use the same '[type][status] Title' naming scheme to match the remote repository.

## Root Cause
The title generation logic in `issue_coordinator.go` and `sync.go` prepends a hardcoded `feat/spec:` label. Local storage (DB/Files) currently stores the raw title without the type/status metadata, leading to inconsistency.

## Requirements
1. Remove the 'feat/spec:' prefix from remote issue titles.
2. Ensure the title follows the format: '[type][status] Title'.
3. Update the local database and spec files so that stored titles include the `[type][status]` prefix, ensuring consistency across local and remote environments.
4. Verify that all spec types (feat, bug, refactor, chore) are formatted consistently.
5. Update existing tests to match the new title format and consistency requirements.
