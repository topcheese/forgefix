---
spec_id: "SPEC-1784101804"
status: review
repo_issue: 533
type: bug
version: "0.9.6"
root_cause: "FindDuplicateSpec matches only by normalized title similarity. It never compares spec_id. Two files with identical spec_id but different titles pass the detector. No creation path checked spec_id uniqueness."
resolution: "Post-creation spec_id collision check added in SPEC-1784257550 via FindSpecByID (cmd_spec.go:322). Detects real collisions, skips self-matches, auto-links in AI mode."
linked_commits: ["a329c2b"]
---

# Investigate How Duplicate spec_id SPEC-1784089531 Escaped The Duplicate Detector

## Objective

Two spec files in `specs/` both declared `spec_id: "SPEC-1784089531"`.
The duplicate detector is supposed to catch duplicate specs at creation time,
yet both files existed. This spec investigates WHY the detector failed so the
automation gap can be closed.

## Root Cause Analysis (from code reading)

- `FindDuplicateSpec` (engine/duplicate.go:89) iterates `listSpecs()` and
  compares **only the normalized title** via `SimilarityRatio` (Levenshtein
  distance / max length). `DuplicateThreshold = 0.7`.
- It returns `isDup=true` only when title similarity exceeds 0.7. It
  **never reads or compares `spec_id`** at all.
- The two offending files had **different titles** → low similarity →
  detector reported no duplicate.
- `writeSpecFromTemplate` generates the id as `fmt.Sprintf("SPEC-%d", time.Now().Unix())`.
  Nothing in `createSpec` or `handleSpec` validated id uniqueness.

## Why This Matters

A duplicate `spec_id` corrupts the ledger: `LoadLedger` / `GetSpecEntry`
key specs by id, so two files sharing one id collide, and `ff sync` /
`ff ship` can bind the wrong file to a remote issue.

## Verification Results (2026-07-16)

The post-creation spec_id collision check was implemented in
`engine/cmd_spec.go` line 322 as `FindSpecByID()` called after
`writeSpecFromTemplate()`. The self-match bug was fixed in commit
`a329c2b` (SPEC-1784257550): `FindSpecByID` now returns the matching
file path and the collision guard checks `existingPath != filePath`.

**All investigation requirements confirmed:**
- [x] Two files with same spec_id, different titles → FindDuplicateSpec returns false (title-only). FindSpecByID catches it post-creation.
- [x] All creation paths converge through createSpec → writeSpecFromTemplate → FindSpecByID check.
- [x] Root cause was missing id-uniqueness check in creation path. Closed by post-creation scan.
- [x] Gap closed: spec_id collision detection now works end-to-end.
