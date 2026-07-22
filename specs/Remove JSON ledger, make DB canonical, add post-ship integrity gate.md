---
spec_id: "SPEC-1784679435"
status: draft
repo_issue: 553
type: feature
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---

# Remove JSON ledger, make DB canonical, add post-ship integrity gate

## Objective

Ensure local DB, spec files, and remote repo are always in sync, with a verification gate before shipping.

## Problem

The system has three sources of truth that can drift:
1. Spec files on disk (`.md` with frontmatter)
2. SQLite DB (`specs` table + `linked_commits`)
3. Remote issues (GitHub/Gitea)

Additionally, a legacy JSON ledger (`forgefix_ledger.json`) duplicates DB data, adding confusion and corruption risk. The current `SyncFromSpecsDir` scans every spec file on every `LoadLedger()` call, parsing frontmatter by hand to reconcile three stores. This circular flow (DB → JSON → spec files → DB) is fragile.

**Failure modes observed:**
- Agent edits spec file directly → `ff sync` fails or doesn't run → DB is stale
- `ff commit` amend cycle corrupts `linked_commits` in JSON and DB
- Post-ship: no verification that DB state matches remote repo
- JSON ledger drifts from DB because `SaveLedger` does DELETE-all + re-INSERT every time

## Requirements

### 1. Remove JSON ledger as source of truth
- `forgefix_ledger.json` becomes an optional export/debug artifact, not in the read/write path
- `LoadLedger` reads from SQLite only (no JSON fallback except for one-time migration)
- `SaveLedger` writes to SQLite only (JSON export is opt-in via `ff ledger export`)
- `SyncFromSpecsDir` writes changes directly to SQLite, not into an in-memory JSON mirror

### 2. DB is the canonical spec store
- All spec metadata (status, type, version, linked_commits, repo_issue) lives in SQLite
- Spec files on disk are the source of truth for body/content only
- Frontmatter in spec files is a mirror of DB state (written by ff commands, not read as authority)
- `ff spec`, `ff commit`, `ff ship` write to DB first, then update spec file frontmatter

### 3. Pre-ship consistency gate
Before `ff ship` proceeds, verify:
- All specs in `ship` status have a valid `repo_issue` linked
- All `linked_commits` hashes exist in the local git repo
- No spec in `ship` status has unsynced changes (DB version matches spec file version)
- Gate fails with clear error listing inconsistent specs

### 4. Post-ship integrity verification
After `ff ship` pushes to remote, verify:
- For each shipped spec, fetch the remote issue and compare body/title/status
- Compare DB `linked_commits` against git log for the shipped version tag
- Report any drift as warnings (non-blocking for first release, blocking after)
- Write verification result to `ff ship --report` output

### 5. Sync flow cleanup
- `ff sync` writes spec file changes to DB (not just DB to remote)
- `ff sync` reconciles remote issue state with DB (not with spec files directly)
- Remove `SyncFromSpecsDir` as the primary sync mechanism; replace with targeted DB writes
- Keep `SyncFromSpecsDir` only as a recovery/catch-up tool for when DB is corrupted

## Acceptance Criteria
- `forgefix_ledger.json` is not read or written during normal `ff commit`, `ff sync`, or `ff ship` operations
- `ff ship` fails if pre-ship consistency gate finds issues
- `ff ship` outputs a post-ship verification report comparing DB vs remote
- Existing tests pass (449 tests, 0 failures)
- Unit test: create spec, commit, sync, ship, verify DB matches remote issue body
- Unit test: corrupt DB entry, run ship, verify gate catches inconsistency

## Out of Scope
- Removing the JSON file entirely (keep as optional export)
- Changing the remote issue tracker API (GitHub/Gitea)
- Modifying the kanban board functionality
