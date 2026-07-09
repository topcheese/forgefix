---
spec_id: "SPEC-1783603629"
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Make ff commit --ai Auto-detect Spec From Working Tree

## Objective

`ff commit --ai` currently ignores the `--ai` flag entirely (`AIMode` is parsed in
`handleCommit` but never passed to `runCommit`). When no `--spec` is provided, it
falls through to an interactive prompt (`promptForSpecSelection`) which fails in
non-interactive environments and defeats the purpose of the `--ai` flag.

Fix: when `--ai` is set and no spec ID is provided (via `--spec` or in the message),
auto-detect the most recently modified spec file in `specs/`. This makes
`ff spec --ai <title>` → `ff commit --ai` work end-to-end without interaction.

## Requirements

1. `handleCommit` must pass `flags.AIMode` to `runCommit`.
2. `runCommit` must accept `aiMode bool` and, when true + no spec ID, call
   `autoDetectSpecFromWorkingTree(wd)` instead of `promptForSpecSelection`.
3. `autoDetectSpecFromWorkingTree(wd)` scans `specs/`, parses each `.md` file's
   frontmatter, and returns the `spec_id` of the most recently modified file.
4. If no spec files are found, return a clear error suggesting
   `ff spec --ai <title>`.

## Implementation

- Add `aiMode bool` parameter to `runCommit`.
- In `handleCommit`, pass `flags.AIMode` to the `runCommit` call.
- Add `autoDetectSpecFromWorkingTree(wd string) (string, error)` that reads `specs/`
  dir entries, gets each file's `Info().ModTime()`, parses frontmatter via
  `parseSpecFileForCommit`, and returns the spec ID with the latest mtime.
- In `runCommit`, replace the `promptForSpecSelection` fallback with a branch:
  when `aiMode`, call the auto-detect; otherwise, prompt interactively.
- The function handles the self-referential case: `ff commit --ai` run after
  `ff spec --ai` picks up the just-created spec because it has the newest mtime.
- Unit test verifies `autoDetectSpecFromWorkingTree` returns the correct spec
  when multiple spec files exist.

## Acceptance Criteria

- `ff commit --ai` with no `--spec` flag and no `[SPEC-XXXX]` in the message
  auto-detects the most recently created/modified spec and commits bound to it.
- `ff commit` (without `--ai`) still prompts interactively as before.
- Unit test for auto-detect function with multiple spec files.

## Verification

- `go test ./...` — all 4 modules pass.
- Manual: `ff spec --ai "test"` → `ff commit --ai "message"` creates a commit
  bound to the new spec without prompting.
