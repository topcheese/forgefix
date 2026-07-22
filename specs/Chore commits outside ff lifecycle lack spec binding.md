---
spec_id: "SPEC-1784678126"
status: draft
repo_issue: 552
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---

# Chore commits outside ff lifecycle lack spec binding

## Objective

When `ff commit` amend cycle produces corrupted state that requires manual correction, the only recourse is raw `git commit`. These commits exist outside the ForgeFix lifecycle — they are not bound to any spec, have no `linked_commits` tracking, and appear as untracked housekeeping in the git log.

## Root Cause

`ff commit` uses an amend-based workflow: it creates a commit, then amends it to fold in metadata changes (spec file updates, ledger writes). The amend produces a new SHA that differs from the original commit. `finalizeCommitAfterAmend` attempts to update `linked_commits` with the final hash, but bugs in that function (wrong path, duplicate append — see SPEC-1784672632) can corrupt the linked_commits list. When this happens, the only way to fix the state is a raw `git commit` that bypasses `ff` entirely.

## Problems This Causes

- Raw commits are not bound to any spec — they violate ForgeFix's core invariant that every commit must be traceable to a spec
- `linked_commits` in the spec file and DB does not include these orphaned commits
- The git log shows commits without `[SPEC-XXXXX]` tags, breaking traceability
- Future `ff sync` or `ff ship` may flag these as orphaned commits

## Requirements

### 1. Prevent the need for raw commits
- Fix the bugs in `finalizeCommitAfterAmend` that corrupt `linked_commits` (see SPEC-1784672632, SPEC-1784678104)
- Make `ff commit` idempotent: re-running it should not corrupt already-correct linked_commits

### 2. Retroactive binding (recovery)
- Provide `ff commit --bind <spec_id> <hash>` to retroactively associate an orphaned commit with a spec
- Update the spec file's `linked_commits` and the DB accordingly
- Validate that the commit message does not already reference a different spec

### 3. Detection
- `ff status` or `ff sync` should detect commits in the log that lack a `[SPEC-XXXXX]` tag
- Report them as warnings so the user can decide whether to bind or ignore them

## Acceptance Criteria
- After fixing SPEC-1784672632 and SPEC-1784678104, no raw commits are needed for normal `ff commit` workflows
- `ff commit --bind SPEC-123 abc1234` associates commit `abc1234` with `SPEC-123` and updates linked_commits
- `ff status` warns about unbound commits in the recent git log
- Existing tests pass (449 tests, 0 failures)
