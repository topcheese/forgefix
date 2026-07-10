---
spec_id: "SPEC-1783710741"
status: ship
repo_issue: 513
type: feature
version: "v0.8.0"
root_cause: "resolution frontmatter field is never auto-populated after implementation commits are made"
resolution: ""
---
# Auto-fill Resolution Field In Spec Frontmatter During Ff Sync

## Objective

When a spec has linked commits in the ledger (via `ff commit --ai`), the
`resolution` field in the spec file's YAML frontmatter stays empty. The only
way to fill it is manual editing, which never happens. Fix: during `ff sync`,
read the ledger's `linked_commits` for each spec and write them into the
`resolution` field automatically.

## Requirements

1. During `ff sync`, for each spec that has non-empty `linked_commits` in the
   ledger, write a formatted string into the spec file's `resolution` field.
2. Format: `"<abbrev_hash> - <commit_subject>"` per commit, semicolon-separated
   if multiple commits exist.
3. Only fill the field when it's currently empty — don't overwrite a manually
   written resolution.
4. Specs with zero linked commits are left untouched.
5. The same logic applies in any code path that updates spec frontmatter during
   sync (`syncSingleSpec`, `SyncSpecs`, `DrainHousekeepingQueue`).

## Implementation

- Add `func updateSpecFileResolution(filePath string, commits []string) error`
   to `engine/cmd_backlog.go` (alongside `UpdateSpecFileStatus`).
- Parse the spec file frontmatter, find the `resolution:` line, and replace it
   with `resolution: "<formatted>"`.
- For each commit hash, run `git log --oneline -1 <hash>` to get the subject.
- Format: `"hash1 - subject1; hash2 - subject2"`.
- Call this from `syncSingleSpec` (or the ledger update block within it) when
   the spec's `linked_commits` are non-empty and the current resolution is empty.
- Wrap the git log call in a helper that returns "(unknown)" on failure (e.g.,
   hash not found in history after rebase).

## Acceptance Criteria

- `ff sync` fills `resolution: "e77ab00 - spec for release creation; b028f48 - implementation"`
  for a spec with two linked commits.
- `ff sync` does NOT overwrite a manually set resolution.
- `ff sync` on a draft spec with zero linked commits does nothing.
- The `root_cause` field is NOT touched (it remains manual).

## Verification

- Unit test: `TestUpdateSpecFileResolution` verifies the frontmatter rewrite.
- Unit test: existing resolution is preserved when non-empty.
- Integration: run `ff sync` on a spec with linked commits, then check the
  spec file's resolution field.
- `go test ./... -count=1` green.
