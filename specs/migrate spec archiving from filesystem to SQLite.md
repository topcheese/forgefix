---
spec_id: "SPEC-1783722120"
status: review
repo_issue: ""
type: refactor
version: "0.9.0"
root_cause: "ff archive moves closed spec files to specs/archive/archive_YYYYMMDD.md — file-based, unqueryable, creates stale files. DB already has a specs table; archiving should use it instead."
resolution: ""
---
# Migrate Spec Archiving From Filesystem To SQLite

## Objective

Replace the file-based archive system (`specs/archive/archive_YYYYMMDD.md`)
with DB-based archiving. `ff archive` sets `status = 'archived'` in the
SQLite `specs` table and deletes the spec file. Old archive files are
imported into the DB and then deleted. The `specs/archive/` directory is
removed after migration.

## Requirements

1. **Import existing archives**: Read all entries from existing
   `specs/archive/archive_*.md` files, convert each to a `specs` table row
   with `status = 'archived'`, and insert into `.ff/forgefix.db`.
2. **Update `ff archive`**: Instead of moving spec files to an archive
   markdown file, set `status = 'archived'` in the DB `specs` table and
   delete the spec file from `specs/`.
3. **Remove archive directory**: After importing existing archives, delete
   the `specs/archive/` directory entirely.
4. **Remove file-moving code**: The `ArchiveResolvedSpecs` function in
   `engine/archive.go` currently reads spec files, appends to an archive
   document, and removes the originals. Replace this with DB updates.
5. **Keep JSON ledger in sync**: When archiving, also update the ledger
   entry (or remove it) so `ff specs` doesn't show archived specs.

## Implementation

- Add migration `003` to `engine/db.go`: adds an index on `specs.status`
  for fast archived-spec queries.
- Modify `ArchiveResolvedSpecs` (or create a new `ArchiveToDB` function):
  - Iterates specs in "closed" or "resolved" status.
  - Sets `status = 'archived'` in the DB `specs` table.
  - Deletes the spec file from `specs/`.
  - Removes the ledger entry (or sets status to "archived").
  - Skips file-based archive creation entirely.
- Add `ImportArchiveFiles(configDir string, db *DB) error`:
  - Reads each `specs/archive/archive_*.md`.
  - Parses frontmatter and body for each spec entry.
  - Inserts into DB via `db.UpsertSpec(...)`.
  - After all files are imported, deletes `specs/archive/` directory.
- Remove `archive_*.md` file generation from `engine/archive.go`.
- Remove `specs/archive/` directory creation logic.

## Acceptance Criteria

- `ff archive --ai` sets specs to "archived" in DB and deletes spec files.
- No new `archive_YYYYMMDD.md` files are created in `specs/archive/`.
- Existing archive files are imported into DB and the archive directory is
  deleted after migration.
- `ff specs` does not show archived specs.
- All existing tests pass.

## Verification

- Run `ff archive --ai`, then `sqlite3 .ff/forgefix.db "SELECT COUNT(*) FROM specs WHERE status = 'archived'"` > 0.
- `ls specs/archive/` returns "no such file or directory".
- `go test ./... -count=1` green.
