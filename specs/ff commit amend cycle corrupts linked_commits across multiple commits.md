---
spec_id: "SPEC-1784678104"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---

# ff commit amend cycle corrupts linked_commits across multiple commits

## Objective

The `ff commit` amend cycle overwrites `linked_commits` in both the ledger and spec file during each commit-amend pair, causing data loss and incorrect commit hashes on rapid successive commits.

## Root Cause

`ff commit` follows this flow:
1. `AutoStageAndCommit` creates a commit with hash H1
2. Metadata changes (spec file, ledger) are staged
3. `amendLastCommit` amends the commit, producing hash H2
4. `finalizeCommitAfterAmend` tries to replace H1 with H2 in `linked_commits`

The problem is that `UpdateLedgerAfterCommit` (step 0, before amend) appends H1 to `linked_commits`. Then `finalizeCommitAfterAmend` (step 4) replaces the last entry. But when multiple `ff commit` calls happen in succession:
- Commit A: appends H1_A, amend produces H2_A, replaces with H2_A ✓
- Commit B: appends H1_B, amend produces H2_B, replaces with H2_B — but H1_B was never in the list if A's amend already replaced it

The underlying issue is that the amend changes the commit hash *after* the hash was recorded, and the replacement logic doesn't account for the intermediate state correctly. Additionally, `UpdateLedgerAfterCommit` and `finalizeCommitAfterAmend` both write to the spec file and ledger, creating race conditions.

## Problems This Causes

- `linked_commits` contains wrong hashes (pre-amend instead of post-amend)
- Multiple consecutive `ff commit` calls corrupt the list, losing entries
- Requires manual raw git commits to fix (see SPEC-1784678126)
- Observed in production: three consecutive `ff commit` calls each corrupted the list

## Requirements

### 1. Atomic commit + metadata update
- The commit hash recorded in `linked_commits` must be the final post-amend hash
- The amend and the linked_commits update must be atomic — no intermediate state where the hash is wrong

### 2. No duplicate entries
- `finalizeCommitAfterAmend` must not append a hash that already exists in the list
- Already fixed in SPEC-1784672632, but the amend-then-replace cycle itself remains problematic

### 3. Idempotent commits
- Re-running `ff commit` with the same message should not corrupt an already-correct `linked_commits` list
- If the last linked_commit already equals HEAD, do nothing

## Acceptance Criteria
- Three consecutive `ff commit` calls produce a `linked_commits` list with exactly three correct post-amend hashes
- No duplicate entries in `linked_commits` after any number of consecutive commits
- Each entry in `linked_commits` corresponds to an actual commit in the git log
- Existing tests pass (449 tests, 0 failures)
- Unit test: run three consecutive `ff commit` calls, verify all three hashes are correct and unique
