---
spec_id: "SPEC-1784101804"
status: draft
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "FindDuplicateSpec (engine/duplicate.go) matches ONLY by normalized title similarity (Levenshtein, DuplicateThreshold=0.7). It never compares spec_id. Two spec files with identical spec_id but different titles (e.g. 'Fix Test404reconciliation' vs 'Fix Testspeclifecycle') score low title similarity, so the detector reports isDup=false and both files are written with the same SPEC-1784089531 id. No code path (writeSpecFromTemplate, createSpec, handleSpec) ever checks spec_id uniqueness."
resolution: ""
linked_commits: []
---

# Investigate How Duplicate spec_id SPEC-1784089531 Escaped The Duplicate Detector

## Objective

Two spec files in `specs/` (`Fix Test404reconciliation.md` and
`Fix Testspeclifecycle.md`) both declare `spec_id: "SPEC-1784089531"`.
The duplicate detector is supposed to catch duplicate specs at creation time,
yet both files exist. This spec investigates WHY the detector failed so the
automation gap can be closed (see the companion feature spec
"Add post-creation duplicate scan to ff spec").

## Root Cause Analysis (from code reading)

- `FindDuplicateSpec` (engine/duplicate.go:89) iterates `listSpecs()` and
  compares **only the normalized title** via `SimilarityRatio` (Levenshtein
  distance / max length). `DuplicateThreshold = 0.7`.
- It returns `isDup=true` only when title similarity exceeds 0.7. It
  **never reads or compares `spec_id`** at all.
- The two offending files have **different titles** ("Fix Test404reconciliation"
  vs "Fix Testspeclifecycle") → low similarity → detector reports no duplicate.
- `writeSpecFromTemplate` (engine/cmd_spec.go:85) generates the id as
  `fmt.Sprintf("SPEC-%d", time.Now().Unix())`. Nothing in `createSpec`
  or `handleSpec` validates id uniqueness against existing specs.
- `listSpecs` (duplicate.go:61) parses every `specs/*.md` for
  `spec_id` + `title`, but that parsed id is only used for the
  title-similarity check — never for an exact-id collision check.

## Why This Matters

A duplicate `spec_id` corrupts the ledger: `LoadLedger` /
`GetSpecEntry` key specs by id, so two files sharing one id collide,
and `ff sync` / `ff ship` can bind the wrong file to a remote issue.
The title-based detector is the wrong tool for catching id collisions.

## Requirements

1. Reproduce: confirm two `specs/*.md` files can carry the same
   `spec_id` while passing `FindDuplicateSpec` (different titles).
2. Identify every creation path that can emit a colliding id:
   `ff spec`, `ff spec --ai`, and any template-based creation.
3. Determine whether the root cause is (a) missing id-uniqueness check,
   (b) `time.Now().Unix()` collisions when two specs are created in the
   same second, or (c) a stale/cached id being reused on rename+recreate.
4. Document the fix location(s) but DO NOT implement here — implementation
   belongs to the companion automation feature spec.

## Acceptance Criteria

- Root cause is confirmed against `engine/duplicate.go` and `cmd_spec.go`
  with a concrete repro (two files, same id, different titles, detector
  returns false).
- The exact gap is named: "no `spec_id` collision check in the
  creation path."
- Companion feature spec exists that closes the gap.
- No source code is modified by this spec (investigation only).

