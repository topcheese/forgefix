---
spec_id: "SPEC-1783788768"
status: ship
repo_issue: 526
type: feature
version: "0.9.5"
root_cause: "Spec commits are tracked in the JSON ledger but not in the spec file itself. The resolution field uses bare hashes with no linkable reference."
resolution: "Implemented in 0adf749. linked_commits field added to template, parsed in SpecFile, written by UpdateLedgerAfterCommit via UpdateSpecFileLinkedCommits helper."
linked_commits: ["d7adac0"]
linked_commits: ["d7adac0"]
# Add Commit Tracking Field To Spec Frontmatter

## Objective

Add a `linked_commits` field to the spec file's YAML frontmatter so each spec
tracks which commits implemented it directly in the file — not just in the
ledger. This makes the resolution field referencable with clickable commit
hashes that Gitea renders as links.

## Requirements

1. Add `linked_commits: []` to `templates/spec_template.md`.
2. When `ff commit --ai` binds a commit via `UpdateLedgerAfterCommit`, also
   write the commit hash into the spec file's `linked_commits` frontmatter.
3. Add `updateSpecFileLinkedCommits(filePath string, commits []string)` helper
   that reads the spec file, finds or creates the `linked_commits` YAML field,
   and appends new hashes.
4. Parse the field in `parseSpecFile` so it's available to other code.
5. The `ff specs` command should show linked commits (it already reads from
   the ledger — the spec file becomes an additional source).

## Implementation

- `templates/spec_template.md`: add `linked_commits: []` line.
- New helper `updateSpecFileLinkedCommits` in `engine/cmd_backlog.go`
  (alongside `UpdateSpecFileStatus`).
- In `UpdateLedgerAfterCommit`, after saving the ledger, call the helper to
  append the commit hash to the spec file.
- `parseSpecFile` (in `spec_manager.go`): add `LinkedCommits []string` field
  and parse it from the frontmatter.

## Acceptance Criteria

- `ff commit --ai "msg"` writes the commit hash into the spec file's frontmatter.
- `ff specs` shows linked commits (from the spec file or ledger).
- Existing specs without the field are not broken (field is optional).
- All existing tests pass.

## Verification

- `ff commit --ai "test"` then `grep linked_commits specs/...md` shows the hash.
- `go test ./... -count=1` green.
