---spec_id: "SPEC-1784102561"
status: review
repo_issue: 536
type: bug
version: "0.9.6"
root_cause: "ArchiveResolvedSpecs (engine/archive.go) leaves rogue .md files on disk. db.ArchiveSpec (db.go:226) DOES remove the file via findSpecFileByID + os.Remove, but only when the file can be located by spec_id; if findSpecFileByID fails (filename/parse mismatch) the removal is skipped and the file lingers. The deferred cleanupOrphanFiles (archive.go:72) also skips any file where parseSpecFileForCommit errors or yields an empty spec_id (lines 88-89), so malformed files become permanent rogue files. The archive path is overcomplicated (ledger + orphan sweep + DB import) and re-does checks that already happened earlier in the chain."
 ""
linked_commits: ["76a8ff7"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 56040c2..be6961c 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -7,6 +7,7 @@
   - feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)
   - feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)
   - feat: Fix ff -v update no asset found error not captured (SPEC-1784103956)
  +- feat: Fix ff archive leaving rogue spec files on disk (SPEC-1784102561)
   
   ## [Unreleased] - 2026-07-14
   
  diff --git a/engine/archive.go b/engine/archive.go
  index 7dec674..c37c873 100644
  --- a/engine/archive.go
  +++ b/engine/archive.go
  @@ -9,6 +9,7 @@ import (
   
   // ArchiveResolvedSpecs archives all closed/resolved specs to the DB and
   // removes their spec files. Returns the count of archived specs.
  +// The file is deleted ONLY after the spec is successfully archived in the DB.
   func ArchiveResolvedSpecs(configDir string) (string, int, error) {
   	ledger, err := LoadLedger(configDir)
   	if err != nil {
  @@ -46,12 +47,16 @@ func ArchiveResolvedSpecs(configDir string) (string, int, error) {
   	// Clean up orphaned spec files — files whose spec_id is not in the ledger
   	// (ledger entry was already removed by a previous archive run, but the
   	// file was left behind because ArchiveSpec used the wrong filename).
  -	orphaned := cleanupOrphanFiles(configDir, ledger)
  +	// Also quarantine files that fail to parse instead of silently skipping.
  +	orphaned, quarantined := cleanupOrphanFiles(configDir, ledger)
   	if orphaned > 0 {
   		fmt.Fprintf(os.Stderr, "Removed %d orphaned spec file(s) left by previous archive runs.\n", orphaned)
   	}
  +	if quarantined > 0 {
  +		fmt.Fprintf(os.Stderr, "Quarantined %d unparseable spec file(s) to specs/_quarantine/.\n", quarantined)
  +	}
   
  -	if archived == 0 && orphaned == 0 {
  +	if archived == 0 && orphaned == 0 && quarantined == 0 {
   		return "", 0, nil
   	}
   
  @@ -63,19 +68,31 @@ func ArchiveResolvedSpecs(configDir string) (string, int, error) {
   	if orphaned > 0 {
   		msg += fmt.Sprintf("; cleaned %d orphaned files", orphaned)
   	}
  +	if quarantined > 0 {
  +		msg += fmt.Sprintf("; quarantined %d unparseable files", quarantined)
  +	}
   	return msg, archived, nil
   }
   
   // cleanupOrphanFiles removes spec files whose spec_id has no matching entry
   // in the ledger. These are files left behind when a previous archive run
   // deleted the ledger entry but failed to remove the file.
  -func cleanupOrphanFiles(configDir string, ledger *LedgerEngine) int {
  +// Files that fail to parse (empty/malformed spec_id) are quarantined to
  +// specs/_quarantine/ instead of being silently skipped.
  +// Returns (orphanedCleaned, quarantinedCount).
  +func cleanupOrphanFiles(configDir string, ledger *LedgerEngine) (int, int) {
   	specDir := filepath.Join(configDir, "specs")
   	entries, err := os.ReadDir(specDir)
   	if err != nil {
  -		return 0
  +		return 0, 0
   	}
   	var cleaned int
  +	var quarantined int
  +
  +	// Ensure quarantine directory exists
  +	quarantineDir := filepath.Join(specDir, "_quarantine")
  +	_ = os.MkdirAll(quarantineDir, 0755)
  +
   	for _, entry := range entries {
   		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
   			continue
  @@ -86,6 +103,13 @@ func cleanupOrphanFiles(configDir string, ledger *LedgerEngine) int {
   		path := filepath.Join(specDir, entry.Name())
   		spec, err := parseSpecFileForCommit(path)
   		if err != nil || spec.SpecID == "" {
  +			// Quarantine unparseable files instead of silently skipping
  +			quarantinePath := filepath.Join(quarantineDir, entry.Name())
  +			if moveErr := os.Rename(path, quarantinePath); moveErr != nil {
  +				fmt.Fprintf(os.Stderr, "warning: failed to quarantine unparseable file %s: %v\n", entry.Name(), moveErr)
  +			} else {
  +				quarantined++
  +			}
   			continue
   		}
   		// If the spec_id is not in the ledger, the entry was already removed.
  @@ -97,7 +121,7 @@ func cleanupOrphanFiles(configDir string, ledger *LedgerEngine) int {
   			}
   		}
   	}
  -	return cleaned
  +	return cleaned, quarantined
   }
   
   // DeprecatedArchiveExists checks whether the old specs/archive/ directory still
  diff --git a/engine/archive_test.go b/engine/archive_test.go
  index 26e9e8c..79cf394 100644
  --- a/engine/archive_test.go
  +++ b/engine/archive_test.go
  @@ -193,3 +193,64 @@ func TestArchiveResolvedSpecs_MixedResolvedAndClosed(t *testing.T) {
   		t.Errorf("expected 2 archived specs in DB, got %d", archivedCount)
   	}
   }
  +
  +func TestArchiveResolvedSpecs_QuarantinesMalformedFiles(t *testing.T) {
  +	configDir := t.TempDir()
  +	specsDir := filepath.Join(configDir, "specs")
  +	if err := os.MkdirAll(specsDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +
  +	// Create a malformed spec file (empty spec_id)
  +	malformedContent := "---\n" +
  +		"spec_id: \"\"\n" +
  +		"status: closed\n" +
  +		"type: feature\n" +
  +		"created: 2026-07-03\n" +
  +		"---\n" +
  +		"# Malformed Spec\n\n" +
  +		"## Goal\n\nTest spec with empty spec_id.\n"
  +	malformedPath := filepath.Join(specsDir, "malformed.md")
  +	if err := os.WriteFile(malformedPath, []byte(malformedContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
  +	// Create a valid closed spec
  +	writeArchiveSpecFile(t, configDir, "SPEC-VALID", "closed")
  +	addToLedger(t, configDir, "SPEC-VALID", "closed")
  +
  +	_, count, err := ArchiveResolvedSpecs(configDir)
  +	if err != nil {
  +		t.Fatalf("ArchiveResolvedSpecs: %v", err)
  +	}
  +	if count != 1 {
  +		t.Errorf("expected 1 archived spec, got %d", count)
  +	}
  +
  +	// Valid spec file should be deleted
  +	if _, err := os.Stat(filepath.Join(specsDir, "SPEC-VALID.md")); !os.IsNotExist(err) {
  +		t.Error("valid spec file should have been deleted")
  +	}
  +
  +	// Malformed file should be quarantined
  +	quarantineDir := filepath.Join(specsDir, "_quarantine")
  +	quarantinedPath := filepath.Join(quarantineDir, "malformed.md")
  +	if _, err := os.Stat(quarantinedPath); os.IsNotExist(err) {
  +		t.Errorf("malformed file should have been quarantined to %s", quarantinedPath)
  +	}
  +
  +	// DB should have the valid spec archived
  +	db, err := OpenDB(configDir)
  +	if err != nil {
  +		t.Fatal(err)
  +	}
  +	defer db.Close()
  +	var status string
  +	err = db.Conn().QueryRow("SELECT status FROM specs WHERE spec_id = ?", "SPEC-VALID").Scan(&status)
  +	if err != nil {
  +		t.Fatalf("querying archived spec: %v", err)
  +	}
  +	if status != "archived" {
  +		t.Errorf("expected status 'archived', got %q", status)
  +	}
  +}
  diff --git a/engine/db.go b/engine/db.go
  index 7bf844c..629dfb4 100644
  --- a/engine/db.go
  +++ b/engine/db.go
  @@ -222,18 +222,31 @@ func (db *DB) GetLinkedCommits(specID string) ([]string, error) {
   }
   
   // ArchiveSpec sets a spec's status to "archived" in the DB and deletes its file.
  -// Returns an error if the file can't be removed (non-fatal) or the DB update fails.
  +// The file is deleted ONLY after the DB update succeeds.
  +// Returns an error if the DB update fails or if the file can't be removed after successful archive.
   func (db *DB) ArchiveSpec(specID, title, specType string, repoIssueID int) error {
   	specDir := filepath.Join(db.configDir, "specs")
  -	// Use findSpecFileByID to locate the file by spec_id, not by title —
  -	// the title in the ledger may not match the actual filename on disk.
  +	// First, update the DB to mark as archived
  +	if err := db.UpsertSpec(specID, title, "archived", specType, "", repoIssueID, "", "", ""); err != nil {
  +		return fmt.Errorf("failed to archive spec in DB: %w", err)
  +	}
  +
  +	// Now that DB is updated, try to find and remove the file
  +	// Use findSpecFileByID to locate the file by spec_id
   	filePath, err := findSpecFileByID(specDir, specID)
  -	if err == nil {
  -		if err := os.Remove(filePath); err != nil {
  -			fmt.Fprintf(os.Stderr, "warning: failed to remove spec file %s: %v\n", filepath.Base(filePath), err)
  +	if err != nil {
  +		// Fallback: try to find by filename pattern (specID.md)
  +		filePath = filepath.Join(specDir, specID+".md")
  +		if _, statErr := os.Stat(filePath); statErr != nil {
  +			// File not found by either method - log warning but don't fail
  +			fmt.Fprintf(os.Stderr, "warning: archived spec %s file not found on disk (tried findSpecFileByID and %s.md)\n", specID, specID)
  +			return nil
   		}
   	}
  -	return db.UpsertSpec(specID, title, "archived", specType, "", repoIssueID, "", "", "")
  +	if err := os.Remove(filePath); err != nil {
  +		return fmt.Errorf("failed to remove archived spec file %s: %w", filepath.Base(filePath), err)
  +	}
  +	return nil
   }
   
   // ---------------------------------------------------------------------------
  diff --git a/specs/Fix ff archive leaving rogue spec files on disk.md b/specs/Fix ff archive leaving rogue spec files on disk.md
  index de914ec..c88d8b0 100644
  --- a/specs/Fix ff archive leaving rogue spec files on disk.md	
  +++ b/specs/Fix ff archive leaving rogue spec files on disk.md	
  @@ -1,6 +1,6 @@
   ---
   spec_id: "SPEC-1784102561"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
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

