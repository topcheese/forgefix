---spec_id: "SPEC-1784100187"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: ""
 "Implemented FinalizeChangelogForRelease in engine/changelog.go and wired it into ShipController.Run() in engine/ship_controller.go. The function promotes all '## [Unreleased] - <date>' sections to a single '## [v<version>] - <today>' section, merges bullets, preserves existing versioned sections, and is idempotent. Called after version determination and before git push so the finalized changelog rides along in the shipped commit. Best-effort: failure warns but does not abort ship."
linked_commits: ["0ddffb9", "f168118"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 7f6622b..8a4fb15 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -4,6 +4,7 @@
   - feat: clean up 6 rogue SPEC-1784089531 duplicate spec files and remove junk DB row (SPEC-1784104910)
   - feat: Add post-creation duplicate scan to ff spec (SPEC-1784101811) (SPEC-1784101811)
   - feat: Remove test-specs directory from commit (SPEC-1784101811)
  +- feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)
   
   ## [Unreleased] - 2026-07-14
   
  diff --git a/specs/Clean up 6 rogue SPEC-1784089531 duplicate files.md b/specs/Clean up 6 rogue SPEC-1784089531 duplicate files.md
  index f01244a..6a0c5da 100644
  --- a/specs/Clean up 6 rogue SPEC-1784089531 duplicate files.md	
  +++ b/specs/Clean up 6 rogue SPEC-1784089531 duplicate files.md	
  @@ -6,7 +6,7 @@ type: bug
   version: "0.9.6"
   root_cause: "Six specs/*.md files all declare spec_id SPEC-1784089531 (a duplicate-ID collision traced in SPEC-1784101804). The DB has exactly one junk row for that id (status=draft, title='SPEC-1784089531' — a placeholder). These files are leftovers from a prior broken session and were never archived, so ff archive left them on disk. They are rogue duplicates that must be removed so specs/ holds only active drafts."
   resolution: "Deleted the 6 rogue SPEC-1784089531 .md files and removed the single junk DB row (spec_id SPEC-1784089531) so the spec table no longer carries the placeholder. Legitimate ship-status specs (SPEC-1783784714, SPEC-1783788768) were preserved."
  -linked_commits: []
  +linked_commits: ["0ddffb9"]
   ---
   
   # Clean Up 6 Rogue SPEC-1784089531 Duplicate Spec Files
  diff --git a/specs/Finalize CHANGELOG.md [Unreleased] sections to shipped version on ff ship.md b/specs/Finalize CHANGELOG.md [Unreleased] sections to shipped version on ff ship.md
  index 27ef639..2be6bda 100644
  --- a/specs/Finalize CHANGELOG.md [Unreleased] sections to shipped version on ff ship.md	
  +++ b/specs/Finalize CHANGELOG.md [Unreleased] sections to shipped version on ff ship.md	
  @@ -1,12 +1,12 @@
   ---
   spec_id: "SPEC-1784100187"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
   root_cause: ""
  -resolution: ""
  -linked_commits: []
  +resolution: "Implemented FinalizeChangelogForRelease in engine/changelog.go and wired it into ShipController.Run() in engine/ship_controller.go. The function promotes all '## [Unreleased] - <date>' sections to a single '## [v<version>] - <today>' section, merges bullets, preserves existing versioned sections, and is idempotent. Called after version determination and before git push so the finalized changelog rides along in the shipped commit. Best-effort: failure warns but does not abort ship."
  +linked_commits: ["0ddffb9"]
   ---
   
   # Finalize CHANGELOG.md [Unreleased] Sections To The Shipped Version On `ff ship`
  diff --git a/specs/Fix Test404reconciliation.md b/specs/Fix Test404reconciliation.md
  new file mode 100644
  index 0000000..de9ba65
  --- /dev/null
  +++ b/specs/Fix Test404reconciliation.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143269"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test Test404Reconciliation during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: Test404Reconciliation
  +- File: integration_lifecycle_test.go
  +- Line: 0
  +- Error: === RUN   Test404Reconciliation
  +    integration_lifecycle_test.go:511: ff sync failed: exit status 1
  +        ForgeFix 0.9.0
  +        ⚠ `ff sync` talks to the remote issue tracker and may push metadata. Continue (y/N/q): sync: aborted — not confirmed.
  +--- FAIL: Test404Reconciliation (0.58s)
  +
  diff --git a/specs/Fix Testdeletespec_fullintegration.md b/specs/Fix Testdeletespec_fullintegration.md
  new file mode 100644
  index 0000000..732361c
  --- /dev/null
  +++ b/specs/Fix Testdeletespec_fullintegration.md	
  @@ -0,0 +1,24 @@
  +---
  +spec_id: "SPEC-1784143269"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestDeleteSpec_FullIntegration during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestDeleteSpec_FullIntegration
  +- File: ledger_test.go
  +- Line: 0
  +- Error: === RUN   TestDeleteSpec_FullIntegration
  +    ledger_test.go:451: expected repoID 55, got 42
  +--- FAIL: TestDeleteSpec_FullIntegration (0.01s)
  +
  diff --git a/specs/Fix Testhandledetonationissues.md b/specs/Fix Testhandledetonationissues.md
  new file mode 100644
  index 0000000..4a3c096
  --- /dev/null
  +++ b/specs/Fix Testhandledetonationissues.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143268"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestHandleDetonationIssues during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestHandleDetonationIssues
  +- File: issue_coordinator_test.go
  +- Line: 0
  +- Error: === RUN   TestHandleDetonationIssues
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +    issue_coordinator_test.go:644: Expected 1 queued operation, got 0
  +--- FAIL: TestHandleDetonationIssues (0.01s)
  +
  diff --git a/specs/Fix Testhandledetonationissues_existingissue.md b/specs/Fix Testhandledetonationissues_existingissue.md
  new file mode 100644
  index 0000000..76bc672
  --- /dev/null
  +++ b/specs/Fix Testhandledetonationissues_existingissue.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143268"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestHandleDetonationIssues_ExistingIssue during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestHandleDetonationIssues_ExistingIssue
  +- File: issue_coordinator_test.go
  +- Line: 0
  +- Error: === RUN   TestHandleDetonationIssues_ExistingIssue
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +    issue_coordinator_test.go:699: Expected 1 queued operation, got 0
  +--- FAIL: TestHandleDetonationIssues_ExistingIssue (0.00s)
  +
  diff --git a/specs/Fix Testhandletimeoutissues.md b/specs/Fix Testhandletimeoutissues.md
  new file mode 100644
  index 0000000..66a62e0
  --- /dev/null
  +++ b/specs/Fix Testhandletimeoutissues.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143269"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestHandleTimeoutIssues during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestHandleTimeoutIssues
  +- File: issue_coordinator_test.go
  +- Line: 0
  +- Error: === RUN   TestHandleTimeoutIssues
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +    issue_coordinator_test.go:1117: Expected 1 queued operation, got 0
  +--- FAIL: TestHandleTimeoutIssues (0.00s)
  +
  diff --git a/specs/Fix Testintegration_detonationtodefusedfullcycle.md b/specs/Fix Testintegration_detonationtodefusedfullcycle.md
  new file mode 100644
  index 0000000..b8fc177
  --- /dev/null
  +++ b/specs/Fix Testintegration_detonationtodefusedfullcycle.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143269"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestIntegration_DetonationToDefusedFullCycle during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestIntegration_DetonationToDefusedFullCycle
  +- File: execute_test.go
  +- Line: 0
  +- Error: === RUN   TestIntegration_DetonationToDefusedFullCycle
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +    execute_test.go:153: After detonation: IssueRefs len = 0, want 1
  +--- FAIL: TestIntegration_DetonationToDefusedFullCycle (0.00s)
  +
  diff --git a/specs/Fix Testintegration_multiplepipelinesfailures.md b/specs/Fix Testintegration_multiplepipelinesfailures.md
  new file mode 100644
  index 0000000..5864de8
  --- /dev/null
  +++ b/specs/Fix Testintegration_multiplepipelinesfailures.md	
  @@ -0,0 +1,26 @@
  +---
  +spec_id: "SPEC-1784143269"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestIntegration_MultiplePipelinesFailures during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestIntegration_MultiplePipelinesFailures
  +- File: execute_test.go
  +- Line: 0
  +- Error: === RUN   TestIntegration_MultiplePipelinesFailures
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +[DEBUG] HTTP client configured: Proxy=true, TLSInsecure=false
  +    execute_test.go:265: Expected 3 issue refs, got 0
  +--- FAIL: TestIntegration_MultiplePipelinesFailures (0.00s)
  +
  diff --git a/specs/Fix Testspeclifecycle.md b/specs/Fix Testspeclifecycle.md
  new file mode 100644
  index 0000000..c82167f
  --- /dev/null
  +++ b/specs/Fix Testspeclifecycle.md	
  @@ -0,0 +1,23 @@
  +---
  +spec_id: "SPEC-1784143268"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +## Objective
  +Automatically created from failing test TestSpecLifecycle during ff --ai run.
  +
  +## Root Cause
  +Test failed - see failure details below.
  +
  +## Failure Details
  +- Test: TestSpecLifecycle
  +- File: 
  +- Line: 0
  +- Error: === RUN   TestSpecLifecycle
  +--- FAIL: TestSpecLifecycle (0.58s)
  +
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
