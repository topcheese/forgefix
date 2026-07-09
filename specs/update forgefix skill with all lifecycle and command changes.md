---
spec_id: "SPEC-1783610857"
status: review
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Update Forgefix Skill With All Lifecycle And Command Changes

## Objective

Rewrite the forgefix-git-workflow skill to reflect all session changes:
auto-detect, auto-promote, housekeeping drain, ff passthrough, and no raw git.

## Requirements

1. All raw `git` commands replaced with `ff` passthrough equivalents.
2. Lifecycle updated for `--ai` auto-detect, auto-promote (review→ship), and auto-close (ship→closed).
3. All original detail preserved — branching, worktrees, rationalizations, verification.
4. New sections for GitHelper, metadata folding, pre-commit write + amend.
5. Copied to both `~/.agents/skills/forgefix-git-workflow/SKILL.md` and `<repo>/skills/forgefix-git-workflow.md`.

## Implementation

- Rewrote skill preserving all 658 lines of original content with updated ff commands.
- Replaced every `git <cmd>` reference with `ff <cmd>` passthrough.
- Updated lifecycle table: draft→review→ship→closed→archived with `--ai`.
- Added Auto-Detect Heuristic section describing `autoDetectSpecFromWorkingTree`.
- Added Commit Message Dedup section describing `strings.ReplaceAll` targeting.
- Added Internal Architecture section describing `GitHelper` and metadata folding.
- Updated promote logic: runs BEFORE SyncSpecs, respects aiMode.
- Updated ship gate: drains housekeeping on completion.
- Verification checklist updated with `--ai` variants.

## Acceptance Criteria

- Skill covers the full `ff` lifecycle without referencing raw `git`.
- All original branching, worktree, rationalization content preserved.
- No instances of raw `git` (except passthrough examples showing `ff → git` mapping).
- `ff commit --ai`, `ff sync --ai`, `ff ship --ai`, `ff archive --ai` documented.
- Internal architecture (`GitHelper`, `promoteReviewSpecs`, pre-commit write) explained.

## Verification

- Skill copied to `~/.agents/skills/forgefix-git-workflow/SKILL.md`.
- Skill copied to `<repo>/skills/forgefix-git-workflow.md`.
- `git status --short` shows clean tree after committing.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->