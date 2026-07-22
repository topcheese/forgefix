---
spec_id: "SPEC-1784263151"
status: draft
repo_issue: 547
type: bug
version: "0.9.7"
root_cause: "ArchiveResolvedSpecs called db.ArchiveSpec(specID, entry.SpecID, specType, entry.RepoIssueID) without passing Version, RootCause, Resolution, or Body. ArchiveSpec then called UpsertSpec with empty strings for all four, which overwrote existing DB values via ON CONFLICT DO UPDATE."
linked_commits: ["bf5c038", "8fdcdc5"]
---
# ff archive drops version root_cause resolution and body from DB

## Problem

When ff archive runs, it calls db.ArchiveSpec which passes empty strings for version, root_cause, resolution, and body to UpsertSpec. Since UpsertSpec uses ON CONFLICT DO UPDATE, these empty values overwrite whatever was previously stored in the DB. The archived spec record therefore has empty values for all four fields, losing the spec's resolution details permanently.

## Root Cause

archive.go: ArchiveResolvedSpecs() at line 39 calls:
    db.ArchiveSpec(specID, entry.SpecID, specType, entry.RepoIssueID)

It doesn't pass entry.Version, entry.RootCause, entry.Resolution, or entry.Body even though the ledger SpecEntry has all four fields populated (populated by SyncFromSpecsDir which reads them from the spec file frontmatter).

db.go: ArchiveSpec() at line 230 calls:
    db.UpsertSpec(specID, title, "archived", specType, "", repoIssueID, "", "", "")

The empty strings for version, rootCause, resolution, and body blow away existing DB values via the ON CONFLICT DO UPDATE clause.

Meanwhile, the normal sync path (ImportDB at line 637) properly passes all fields:
    db.UpsertSpec(specID, entry.SpecID, entry.Status, entry.Type, entry.Version, entry.RepoIssueID, entry.RootCause, entry.Resolution, entry.Body)

So the ledger has the data, the DB supports the columns, but the archive path drops it all.

## Requirements

1. ArchiveResolvedSpecs must pass Version, RootCause, Resolution, and Body from the ledger entry to ArchiveSpec
2. ArchiveSpec must accept and forward all four fields to UpsertSpec
3. No regression in the archive flow — archived specs still get status "archived" and their files removed
4. The archived DB record preserves the spec's version, root_cause, resolution, and body as they were before archiving
