---
spec_id: "SPEC-1784102561"
status: review
repo_issue: 536
type: bug
version: "0.9.6"
root_cause: "ArchiveResolvedSpecs leaves rogue .md files on disk. db.ArchiveSpec removes the file via findSpecFileByID + os.Remove, but only when the file can be located by spec_id; if findSpecFileByID fails the removal is skipped. The deferred cleanupOrphanFiles also skips any file where parseSpecFileForCommit errors, so malformed files become permanent rogue files."
resolution: "File is deleted only after successful DB archive. cleanupOrphanFiles now quarantines unparseable files to specs/_quarantine/ instead of silently skipping them."
linked_commits: ["76a8ff7"]
---

# Fix `ff archive` Leaving Rogue Spec Files On Disk

## Objective

`ff archive` reported "Archived 74 resolved specs" yet left spec files on disk.
The `specs/` folder must be cleaned so it contains ONLY the current draft
specs. This spec fixes the broken cleanup, guarantees a **single spec table**
in the DB (archived specs are rows in that one table, not a second "archived"
table), simplifies the overcomplicated archive path, and defers the live
leftover investigation (see "Out of Scope").

## Root Cause (from code reading)

- `db.ArchiveSpec` (db.go:226) already deletes the file via
  `findSpecFileByID` + `os.Remove` (lines 230-235) — **but only after a
  successful DB upsert**, and only if the file is locatable by `spec_id`.
  When `findSpecFileByID` fails (filename/frontmatter mismatch), the removal
  is skipped and the file lingers on disk.
- `cleanupOrphanFiles` (archive.go:72-101) skips any file where
  `parseSpecFileForCommit` errors or returns an empty `spec_id` (lines 88-89)
  via `continue` — malformed/edge-case files are never removed.
- The archive path is overcomplicated: it imports file-based archives into
  the DB, walks the ledger, runs a deferred orphan sweep, AND relies on
  `db.ArchiveSpec`'s own removal. Duplicate/redundant checks that already
  happened earlier in the chain are re-done here.

## Requirements

### 1. Delete the file ONLY after a successful DB archive
- The `.md` file must be removed **only once `db.ArchiveSpec` has
  successfully persisted the spec as `status='archived'` in the DB** — never
  speculatively. If `findSpecFileByID` cannot locate the file by `spec_id`,
  fall back to locating it by filename/title before giving up, and surface a
  clear warning if it still cannot be found (so the file is not silently left).

### 2. Guarantee a SINGLE spec table (no second "archived" table)
- Confirm there is exactly **one** `specs` table in `.ff/forgefix.db`
  (db.go:672). Archived specs are rows in that table with
  `status='archived'` (db.go:236 `UpsertSpec(... "archived" ...)`), NOT a
  separate `archived_specs` / `archive` table.
- The cleanup/cross-check must query the **one** `specs` table (filter by
  `status='archived'`), never assume or create a second table.
- Add a guard/test asserting only one spec table exists and that every
  archived spec is a row in it (not a separate structure).

### 3. Simplify / clean up the overcomplicated path
- Remove redundant duplicate checks: spec uniqueness and duplicate detection
  already occur earlier in the chain (see SPEC-1784101804 / the post-creation
  duplicate scan). `ArchiveResolvedSpecs` should not re-implement them.
- Collapse the ledger-walk + orphan-sweep + DB-import into the minimal steps
  needed: (a) for each closed/resolved spec, archive to the single spec table
  and remove its file; (b) a single reconciliation pass that deletes any
  on-disk `.md` whose `spec_id` is present in the spec table as `archived`.
- All specs present in the DB spec table are, by definition, archived specs
  (per #2) — use that invariant instead of cross-referencing the ledger.

### 4. Handle parse failures without silent skips
- Files that fail to parse (empty/malformed `spec_id`) must not be `continue`d
  past. Either (a) tolerant re-parse, or (b) move to `specs/_quarantine/` and
  record them, so they are visibly dealt with instead of lingering.

## Out of Scope (held per instruction)

- **#4 from the original draft — live investigation of current leftovers is
  HELD.** Do NOT run the cross-check against the existing `specs/` folder or
  delete/quarantine the current rogue files as part of this spec's
  implementation. The cleanup logic is built; the one-time sweep of the
  existing folder is deferred to a separate decision.

## Related Spec (raised separately — point #3)

- The `ff -v` "no asset found" error (command_dispatcher.go:187) is a
  SEPARATE bug: it only prints to stderr and returns — it is neither captured
  into the background queue for attention nor logged persistently, despite
  prior intent to route such errors there. A new spec/issue must be raised
  for it (see companion spec). At minimum it should be logged to console.

## Acceptance Criteria

- A `.md` file is removed from disk **only after** its spec is successfully
  archived (`status='archived'`) in the single spec table; no speculative or
  pre-archive deletion.
- Exactly one `specs` table exists in `.ff/forgefix.db`; archived specs are
  rows in it (no second "archived" table). Verified by test.
- The archive path is simplified: no redundant duplicate checks re-done here;
  reconciliation uses the single spec table's `status='archived'` invariant.
- Malformed/empty-`spec_id` files are quarantined or logged, never silently
  skipped.
- The live leftover sweep of the existing folder is NOT performed by this
  spec (held per instruction).
- No regression to the 13 pre-existing failing integration tests (additive
  change; those remain tracked separately).
