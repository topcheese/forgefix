---spec_id: "SPEC-1784264619"
status: closed
repo_issue: ""
type: refactor
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: ["feab430"]

# Trim Remote Issue Body To Lightweight Summary

## Objective

The remote issue body currently duplicates the full spec content (body, root_cause) and the git diff stuffed into the `resolution` field. This is wasteful — the spec file on disk is the canonical source, git history (via `linked_commits`) tells the implementation story, and the DB cache serves the runtime. The remote issue should be a lightweight tracker: status, linked commits, and key metadata only.

## Root Cause

Three independent mechanisms collaborate to overstuff the remote issue:

1. **`cmd_commit.go` step 5 (FR-5)**: After every `ff commit`, the command runs `git diff HEAD~1..HEAD`, truncates it at 10KB, and writes the entire diff into the spec file's `resolution:` frontmatter field AND the DB `SpecEntry.Resolution`. The resolution field becomes a machine dump, not a human summary.

2. **`spec_manager.go:generateSpecBody()`**: Reads the spec file YAML and pushes Title, SpecID, Type, Version, Status, RootCause, and **Resolution** (which is the full diff) into the remote issue body. Issue #539 shows the result: a 10KB+ diff pasted into the body.

3. **`spec_manager.go:effectiveSpecBody()`**: Returns the full spec `Body` if it's not template scaffolding. This pushes the entire spec body content (requirements, acceptance criteria, etc.) to the remote issue as well.

4. **`issue_coordinator.go:PostResolutionComment()`**: On resolution/close, posts another comment containing the full resolution (diff) and full spec body — more duplication.

## Problems

- **Redundant**: Spec lives in `specs/*.md` (disk) and `SpecEntry` (DB). Copying it to the remote issue adds no information.
- **Noisy**: A 10KB+ diff in the issue body makes the issue harder to read at a glance.
- **Drift surface**: If the resolution field is a diff dump, the human-written root cause is the only semantic content on the remote. The actual implementation is in git.
- **Resolution field misused**: `resolution` should be a brief human explanation of how something was fixed, not a machine diff. The diff is already in git, referenced by `linked_commits`.

## Requirements

### 1. Stop writing git diff to spec resolution
- `cmd_commit.go` step 5 must NOT capture `git diff HEAD~1..HEAD` and write it to the spec file's `resolution` frontmatter or the DB `SpecEntry.Resolution`.
- The `resolution` field stays as-is if non-empty, or stays empty. It's a human field.
- The commit hash is already recorded in `linked_commits` — that's the pointer to the implementation.

### 2. Trim remote issue body to metadata summary
- `generateSpecBody()` must output only: SpecID, Type, Version, Status, linked commits, and optionally a human-written RootCause.
- No full spec body text.
- No resolution/diff content.
- `effectiveSpecBody()` must always use the trimmed `generateSpecBody()` — never push the full spec body to the remote.

### 3. Trim resolution comment
- `PostResolutionComment()` must output only: spec link, root_cause, linked commits.
- No full spec body.
- No resolution/diff content.

### 4. Source from DB, not file parse, for speed
- The sync path (`syncSingleSpec`, `SyncSpecs`) currently parses the spec file YAML to build the body. Switch to reading from the DB `SpecEntry` instead — it already has all the needed fields and avoids file I/O + YAML parse on every sync.

### 5. Resolution field preservation
- Existing specs that already have a human-written `resolution:` value in their frontmatter must be preserved. Only the automated diff-dump behavior should stop.
- The `resolution:` key stays in the frontmatter schema — it's just no longer auto-populated by the commit flow.

## Acceptance Criteria

- `ff commit` no longer writes `git diff` to the spec file's `resolution:` frontmatter or the DB entry.
- `ff sync` pushes only a lightweight metadata summary to the remote issue body: SpecID, Status, Version, Type, root_cause (if set), linked commits.
- `ff sync` does NOT push the spec body or resolution/diff to the remote issue.
- `PostResolutionComment` posts only: spec link, root_cause, linked commits. No diff, no full body.
- Existing human-written `resolution:` values are untouched.
- All existing tests pass without modification (this is additive behavior change).
