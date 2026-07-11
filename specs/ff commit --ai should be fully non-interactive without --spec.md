---
spec_id: "SPEC-1783573136"
status: review
repo_issue: 486
type: feature
version: "0.9.0"
root_cause: "ff commit --ai parsed the flag but never passed it to runCommit — the AIMode field was ignored, so --ai still prompted for spec selection interactively"
resolution: "Implemented in 6855c7e, eb99bf1, 6cf191f, 05bd9df. autoDetectSpecFromWorkingTree picks the most recently modified active spec when --ai is set and no --spec is provided."
---
# Ff Commit   Ai Should Be Fully Non Interactive Without   Spec

## Objective

`ff commit --ai` was parsed by `ParseFlags` but `handleCommit` never passed
`flags.AIMode` to `runCommit`. The `--ai` flag was effectively a no-op —
it still fell through to the interactive `promptForSpecSelection`. Fix:
thread `aiMode` through to `runCommit` and add `autoDetectSpecFromWorkingTree`
to auto-detect the spec when `--ai` is set.

## Requirements

1. `handleCommit` passes `flags.AIMode` to `runCommit`.
2. `runCommit` accepts `aiMode bool` and, when true + no spec ID, calls
   `autoDetectSpecFromWorkingTree(wd)` instead of `promptForSpecSelection`.
3. `autoDetectSpecFromWorkingTree` scans `specs/`, parses frontmatter, and
   returns the `spec_id` of the most recently modified active (non-ship,
   non-closed) spec file.
4. Pre-existing `--objective`, `--requirements`, `--acceptance` flags are
   preserved for non-interactive TUI use.
5. All existing `TestRunCommit_*` tests pass unchanged.

## Implementation

- `engine/cmd_commit.go`: Added `aiMode bool` parameter to `runCommit`.
- `engine/cmd_commit.go`: In `handleCommit`, pass `flags.AIMode`.
- `engine/cmd_commit.go`: Added `autoDetectSpecFromWorkingTree(wd)` function
  that prefers active specs (not shipped/closed) and picks the most recently
  modified.
- `autoDetectSpecFromWorkingTree` handles the self-referential case:
  `ff spec --ai "foo"` → `ff commit --ai "msg"` picks the just-created spec.
- Multiple tests: `TestRunCommit_DedupPreservesOtherSpecRefs`,
  `TestRunCommit_DedupOnlyTagNoBody`, `TestRunCommit_DedupNoSpecInMessage`.
- Also added: pre-commit metadata write to fold spec status and ledger binding
  into the commit (no untracked side effects), and `amendLastCommit` to fold
  any post-commit metadata.

## Acceptance Criteria

- `ff commit --ai "msg"` with no `--spec` auto-detects and commits.
- `ff commit --ai --spec SPEC-X "msg"` still works for explicit binding.
- `ff commit` without `--ai` still prompts interactively.
- `ff commit --ai "msg"` with a `[SPEC-X]` in the message uses that spec.
- All `TestRunCommit_*` tests pass.

## Verification

- `go test ./engine -run TestRunCommit -v` — all pass.
- `go test ./... -count=1` — full suite green.
