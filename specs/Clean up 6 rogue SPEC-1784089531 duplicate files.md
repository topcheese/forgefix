---
spec_id: "SPEC-1784104910"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "Six specs/*.md files all declare spec_id SPEC-1784089531 (a duplicate-ID collision traced in SPEC-1784101804). The DB has exactly one junk row for that id (status=draft, title='SPEC-1784089531' — a placeholder). These files are leftovers from a prior broken session and were never archived, so ff archive left them on disk. They are rogue duplicates that must be removed so specs/ holds only active drafts."
resolution: "Deleted the 6 rogue SPEC-1784089531 .md files and removed the single junk DB row (spec_id SPEC-1784089531) so the spec table no longer carries the placeholder. Legitimate ship-status specs (SPEC-1783784714, SPEC-1783788768) were preserved."
linked_commits: []
---

# Clean Up 6 Rogue SPEC-1784089531 Duplicate Spec Files

## Objective

Remove the rogue duplicate spec files from `specs/` so the folder contains
only active draft specs. This is the live cleanup step held from
SPEC-1784102561 (point #4), now authorized.

## Inventory (captured before deletion)

All six files below declare `spec_id: SPEC-1784089531` and `status: draft`:

| # | File | spec_id | Disposition |
|---|------|---------|-------------|
| 1 | Fix Testdeletespec_fullintegration.md | SPEC-1784089531 | DELETE (rogue dup) |
| 2 | Fix Testhandletimeoutissues.md | SPEC-1784089531 | DELETE (rogue dup) |
| 3 | Fix Testintegration_multiplepipelinesfailures.md | SPEC-1784089531 | DELETE (rogue dup) |
| 4 | Fix Testhandledetonationissues_existingissue.md | SPEC-1784089531 | DELETE (rogue dup) |
| 5 | Fix Testintegration_detonationtodefusedfullcycle.md | SPEC-1784089531 | DELETE (rogue dup) |
| 6 | Fix Testhandledetonationissues.md | SPEC-1784089531 | DELETE (rogue dup) |

## Preserved (NOT rogue — legitimate pre-existing specs)

| spec_id | status | title | reason |
|---------|--------|-------|--------|
| SPEC-1783784714 | ship | Fix Ff V Update Downloads No Asset Found Error | real spec, kept |
| SPEC-1783788768 | ship | Add Commit Tracking Field To Spec Frontmatter | real spec, kept |

## DB cross-check

- `SELECT spec_id, status, title FROM specs WHERE spec_id='SPEC-1784089531'`
  → one row: `draft`, title `SPEC-1784089531` (junk placeholder).
- This single junk row is also removed during cleanup.
- Logs and ledger are NOT modified — only the rogue files and the one junk
  spec-table row are touched.

## Acceptance Criteria

- After cleanup, zero files in `specs/` declare `SPEC-1784089531`.
- `specs/` contains only active draft specs + the two legitimate ship specs.
- The junk DB row for SPEC-1784089531 is gone; logs/ledger untouched.
- Committed via `ff commit --ai --spec SPEC-1784104910` (commit only, no ship).

