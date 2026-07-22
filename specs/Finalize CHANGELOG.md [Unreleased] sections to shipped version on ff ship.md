---
spec_id: "SPEC-1784100187"
status: ship
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "ShipController.Run() never promoted '## [Unreleased]' CHANGELOG sections to the shipped version. AppendChangelogEntry writes unreleased sections at commit time because the version is unknown, but ff ship never finalized them."
resolution: "Added FinalizeChangelogForRelease to engine/changelog.go, wired into ShipController.Run() after version determination and before push. Merges all Unreleased sections into a single versioned section, idempotent, best-effort."
linked_commits: ["0ddffb9"]
---

# Finalize CHANGELOG.md [Unreleased] Sections To The Shipped Version On `ff ship`

## Objective

`ff commit --ai` appends changelog entries under a dated `## [Unreleased] - YYYY-MM-DD`
section as a staging area, because the release version is not known at commit time.
The intended design is that `ff ship` promotes those staged entries to the actual
release version (e.g. `## [v0.9.6] - 2026-07-15`). Today that promotion never happens:
`ShipController.Run()` updates spec versions, pushes, creates tags/releases, and
closes issues, but never touches CHANGELOG.md. The result is a changelog that
accumulates perpetual `[Unreleased]` sections that never get versioned.

This spec adds the missing finalization step so the changelog reflects real
releases instead of forever showing "Unreleased".

## Root Cause Analysis

- `AppendChangelogEntry` (engine/changelog.go) is called from `cmd_commit.go`
  immediately before the commit. It deliberately writes `## [Unreleased] - <today>`
  because the version that will eventually ship the commit is unknown at commit time.
  This is correct and intended behavior.
- `ShipController.Run()` (engine/ship_controller.go) determines `shipVersion` via
  `vm.HandleShipVersion`, updates each shipped spec's `version:` field, pushes,
  creates a git tag `v<shipVersion>`, and creates a remote release — but it never
  renames the `[Unreleased]` sections to `[v<shipVersion>]`.
- Consequence: every day's commits create a new `## [Unreleased] - <date>` section
  that is never finalized. The current CHANGELOG.md has three such sections
  (2026-07-12, 2026-07-13, 2026-07-14) sitting above the last real release
  (`## [v0.9.0] - 2026-07-05`).

## Requirements

### New function: `FinalizeChangelogForRelease`

1. Add `FinalizeChangelogForRelease(wd, version string) error` to engine/changelog.go.
2. It collects every `## [Unreleased] - <date>` section in CHANGELOG.md, merges all
   of their `### 🚀 Release Summary` bullets into a single
   `## [v<version>] - <today>` section, and writes that section at the top of the
   file (before any existing versioned sections such as `## [v0.9.0]`).
3. The merged section preserves the `### 🚀 Release Summary` block and all bullets,
   in their original order (oldest day first, or as they appear top-to-bottom).
4. Idempotent: re-running with the same version must not create a duplicate
   `## [v<version>]` section. If a `## [v<version>]` section already exists, the
   function should either skip or merge into it without duplicating bullets.
5. Non-fatal / no-op when CHANGELOG.md is missing (returns nil, like
   `AppendChangelogEntry`).
6. Must not disturb or reorder any already-versioned sections (`## [v0.9.0]`,
   `## [v0.8.5]`, etc.).

### Wire it into `ff ship`

7. Call `FinalizeChangelogForRelease(sc.configDir, shipVersion)` from
   `ShipController.Run()` after `shipVersion` is determined (after the
   `HandleShipVersion` call at line 60) and before the git push, so the finalized
   changelog is included in the shipped commit. Best-effort: a failure warns but
   does not abort the ship (mirror the `AppendChangelogEntry` pattern in
   `cmd_commit.go`).

## Acceptance Criteria

- After `ff ship 0.9.6`, CHANGELOG.md contains **no** `## [Unreleased]` sections.
- All former `[Unreleased]` bullets appear under a single
  `## [v0.9.6] - YYYY-MM-DD` section at the top of the file.
- Existing versioned sections (`## [v0.9.0]`, `## [v0.8.5]`, ...) are preserved
  below the new section, unchanged.
- Re-running `ff ship` with the same version does not create a second
  `## [v0.9.6]` section or duplicate bullets.
- A missing CHANGELOG.md does not error or abort the ship.
- All existing changelog tests (`engine/changelog_test.go`) still pass, and new
  tests cover `FinalizeChangelogForRelease` (merge of multiple days, idempotency,
  missing file, preservation of versioned sections).

## Verification

- Unit test: seed CHANGELOG.md with three `## [Unreleased] - <date>` sections, call
  `FinalizeChangelogForRelease(tmpDir, "0.9.6")`, assert exactly one
  `## [v0.9.6]` section exists and all bullets are present, no `[Unreleased]` remains.
- Unit test: call it twice with the same version, assert no duplicate section.
- Manual: run `ff ship` on a branch with unreleased entries and inspect the
  resulting CHANGELOG.md diff before pushing.
