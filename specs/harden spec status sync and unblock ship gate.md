---
spec_id: "SPEC-1783600141"
status: ship
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Harden Spec Status Sync And Unblock Ship Gate

## Objective

Follow-up hardening to the spec status-sync work (SPEC-1783572691). Closes three
holes where spec transitions silently desynced disk (frontmatter) and ledger, fixes a
sync-logic bug that wrongly flattened non-`open` remote states to `closed`, and clears
two stale ledger entries that were blocking `ff ship`.

## Requirements

1. Export the frontmatter status helper so tests and callers can update spec status
   safely instead of string-replacing raw markdown.
2. Commit-message dedup must strip only the *current* spec's tag, never references to
   other specs in the same message.
3. `ff sync` must only transition a spec to `closed` when the remote issue is actually
   `closed` — forgefix-internal states like `in-progress` must not be flattened.
4. The lifecycle integration test must keep disk and ledger consistent at every step.
5. Stale ledger entries with invalid/missing statuses must not block `ff ship`.

## Implementation

- **Export `UpdateSpecFileStatus`** (`engine/cmd_backlog.go`): renamed private
  `updateSpecFileStatus` to exported `UpdateSpecFileStatus`; updated all 5 internal
  callers (`cmd_backlog.go` ×2, `cmd_commit.go` ×1, `sync.go` ×2). The integration test
  now calls `engine.UpdateSpecFileStatus` directly instead of a fragile
  `strings.ReplaceAll` on the spec body.

- **Fix commit dedup** (`engine/cmd_commit.go:122-125`): replaced the global regex
  `regexp.MustCompile(...).ReplaceAll(...)` with `strings.ReplaceAll(msg, "["+specID+"]", "")`.
  This strips only the current spec's tag, preserving references to other specs
  (e.g. `[SPEC-456]` stays in a `SPEC-111` commit). Also removed the empty-body fallback
  (`if cleaned == "" { cleaned = msg }`) that doubled the tag when the message was only
  `[SPEC-NNN]`.

- **Clean up test assertions** (`engine/sync_test.go`): simplified the dual
  `status: closed` / `"status": "closed"` assertion to a single exact-format check
  matching what `UpdateSpecFileStatus` writes.

- **Fix sync state bug** (`engine/issue_coordinator.go:1009`): changed
  `remoteIssue.State != "open"` to `remoteIssue.State == "closed"`. In real
  Gitea/GitHub issues are only `open`/`closed`, so any non-`open` state was wrongly
  forcing the spec to `closed`; now only an actually-closed remote issue does.

- **Fix lifecycle test** (`tests/integration_lifecycle_test.go`): Step 4 now promotes
  the ledger entry to `ship` as well as the disk spec file (`review`→`ship` and
  `draft`→`ship`), keeping disk and ledger consistent through the ship step.

- **Close out stale specs** (`.ff/forgefix_ledger.json`): `SPEC-1783487662` moved
  `backlog`→`closed` (leftover entry from a deleted "accidental test spec" — commit
  f53745a never cleaned the ledger, so the strict ship gate blocked on it);
  `SPEC-1783495360` moved `resolved`→`closed` (`resolved` is not a valid status and
  `verifyCommitSpecBindings` rejects it as an orphaned-commit invalid status).

## Acceptance Criteria

- `ff ship` passes the strict gate (no `backlog`/`in-progress` entries; all statuses
  valid: `backlog`, `draft`, `in-progress`, `review`, `ship`, `closed`).
- A commit message containing `[SPEC-111] [SPEC-456]` dedups to only strip `[SPEC-111]`,
  leaving `[SPEC-456]` intact.
- `ff sync` against a remote issue in a non-`open`/non-`closed` state does NOT force the
  spec to `closed`.
- `TestSpecLifecycle` passes with disk and ledger consistent at every step.
- Full suite green: `ForgeFix`, `engine`, `engine/housekeeper`, `tests`.

## Verification

- `go test ./...` — all 4 modules pass.
- `ff ship` — `Ship validation passed. N spec(s) ready to ship.` (only fails later at
  `git push` because the remote has newer commits; `git pull --rebase` first).
- New tests: `TestRunCommit_DedupPreservesOtherSpecRefs`,
  `TestRunCommit_DedupOnlyTagNoBody`.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->
