---spec_id: "SPEC-1784143269"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "SaveLedger only wrote to SQLite when OpenDB succeeded, but did not mirror state to the legacy JSON ledger file. In TestDeleteSpec_FullIntegration the ledger is saved then reloaded via LoadLedger; the reload path read a store that did not reflect the persisted repo_issue_id (55), returning a stale/zero-derived value (42) instead of the expected 55."
 "Commit ba00b4e added a JSON mirror (ledger.SaveToFile) inside SaveLedger before the SQLite write, keeping the legacy forgefix_ledger.json consistent with the canonical SQLite store. LoadLedger now reloads repo_issue_id=55 correctly and TestDeleteSpec_FullIntegration passes."
linked_commits: ["ba00b4e", "3338dbd"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 6fbc8e5..177df1a 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -3,6 +3,7 @@
   ### 🚀 Release Summary
   - feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)
   - feat: fix ff sync 404 reconciliation; unbind orphaned repo_issue and mirror ledger to JSON (SPEC-1784146853)
  +- feat: fix SPEC-1784143269: document root cause/resolution for TestDeleteSpec_FullIntegration repoID mismatch (SPEC-1784143269)
   
   ## [Unreleased] - 2026-07-15
   
  diff --git a/specs/Fix Testdeletespec_fullintegration.md b/specs/Fix Testdeletespec_fullintegration.md
  index 732361c..ee84b70 100644
  --- a/specs/Fix Testdeletespec_fullintegration.md	
  +++ b/specs/Fix Testdeletespec_fullintegration.md	
  @@ -1,24 +1,27 @@
   ---
   spec_id: "SPEC-1784143269"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
  -root_cause: ""
  -resolution: ""
  -linked_commits: []
  +root_cause: "SaveLedger only wrote to SQLite when OpenDB succeeded, but did not mirror state to the legacy JSON ledger file. In TestDeleteSpec_FullIntegration the ledger is saved then reloaded via LoadLedger; the reload path read a store that did not reflect the persisted repo_issue_id (55), returning a stale/zero-derived value (42) instead of the expected 55."
  +resolution: "Commit ba00b4e added a JSON mirror (ledger.SaveToFile) inside SaveLedger before the SQLite write, keeping the legacy forgefix_ledger.json consistent with the canonical SQLite store. LoadLedger now reloads repo_issue_id=55 correctly and TestDeleteSpec_FullIntegration passes."
  +linked_commits: ["ba00b4e"]
   ---
  +
   ## Objective
  -Automatically created from failing test TestDeleteSpec_FullIntegration during ff --ai run.
  +Fix failing test `TestDeleteSpec_FullIntegration` (ledger_test.go) — `DeleteSpec` must return the persisted `repoID` (55) after a save/reload cycle.
   
   ## Root Cause
  -Test failed - see failure details below.
  +`SaveLedger` wrote only to SQLite when `OpenDB` succeeded and skipped mirroring to the legacy JSON ledger file. The reload path (`LoadLedger`) read a store that did not reflect the persisted `repo_issue_id` (55), so `DeleteSpec` returned a stale/zero-derived value (42) instead of the expected 55.
  +
  +## Resolution
  +Commit `ba00b4e` added a JSON mirror (`ledger.SaveToFile`) inside `SaveLedger` before the SQLite write, keeping `forgefix_ledger.json` consistent with the canonical SQLite store. `LoadLedger` now reloads `repo_issue_id=55` correctly and the test passes.
   
   ## Failure Details
   - Test: TestDeleteSpec_FullIntegration
   - File: ledger_test.go
  -- Line: 0
  -- Error: === RUN   TestDeleteSpec_FullIntegration
  -    ledger_test.go:451: expected repoID 55, got 42
  ---- FAIL: TestDeleteSpec_FullIntegration (0.01s)
  +- Line: 451
  +- Error: expected repoID 55, got 42
  +- Status: RESOLVED — test passes on current HEAD.
   
  diff --git a/specs/Fix auto-created specs missing title heading.md b/specs/Fix auto-created specs missing title heading.md
  new file mode 100644
  index 0000000..fdc0da3
  --- /dev/null
  +++ b/specs/Fix auto-created specs missing title heading.md	
  @@ -0,0 +1,38 @@
  +---
  +spec_id: "SPEC-1784225300"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# Fix auto-created specs missing title heading
  +
  +## Objective
  +When `ff` auto-creates a spec (e.g. when a test fails during `ff --ai`), the generated file has no `# Title` H1 heading. `LoadLedger` / `SyncFromSpecsDir` therefore cannot read a title from the `# …` H1 and substitutes the `spec_id` as the display title — the ledger `title` column ends up equal to the `spec_id` instead of a human-readable name.
  +
  +This spec also investigates how the existing specs in this repository were actually created. They should have been produced through `ff spec` (which runs the pre-creation duplicate detector and generates a unique `SPEC-<unix>` id), but multiple files share colliding `spec_id`s, a strong signal that some were hand-written by agents rather than created via `ff`.
  +
  +## Root Cause
  +`engine/cmd_spec.go` — `writeSpecFromTemplate` builds the file as `---<frontmatter>---\n<body>`, where `body` is just the `## Objective / ## Root Cause / ## Failure Details` text. No `# <title>` H1 line is ever written. `parseSpecFile` (spec_manager.go) extracts the title from the `# …` H1; when absent it falls back to the `spec_id`, so the ledger `title` column is set to the spec_id rather than a real title.
  +
  +## Investigation: How were the existing specs created?
  +1. Record each spec's `spec_id`, `created` date, `status`, and whether the body contains a `# Title` H1 heading.
  +2. Inspect `git log` for each spec file to determine if it was created by `ff spec` (committed through the `ff` lifecycle) or hand-written directly (raw file write / agent edit outside `ff`).
  +3. Query the ledger (`specs` table in `.ff/forgefix.db`): for each `spec_id`, check whether `title` equals the human title or the `spec_id` itself — the latter is a tell-tale sign of the missing-heading bug.
  +4. Note the spec_id collisions: at least three files (`Fix Test404reconciliation.md`, `Fix Testruncommitwithflagspecid.md`, `Fix Testspeclifecycle.md`) all carry `SPEC-1784146853`, and several others share `SPEC-1784143269` / `SPEC-1784143268`. `writeSpecFromTemplate` uses `fmt.Sprintf("SPEC-%d", time.Now().Unix())`, so genuine `ff` creation yields a unique id per file; identical ids across files indicate same-second batch creation or manual authoring that hard-coded / copied an id.
  +5. Determine whether agents are invoking `ff spec` or writing `specs/*.md` directly. If the latter, document the gap: manual authoring bypasses the pre-creation duplicate detector (`FindDuplicateSpec`, called inside `createSpec`), so duplicates and id collisions go uncaught.
  +6. Report findings: classify every existing spec as ff-created vs manually-authored, with cited evidence (git log, ledger title value, heading presence, id collisions).
  +
  +## Requirements
  +1. `writeSpecFromTemplate` must emit a `# <title>` H1 heading using the `title` argument it already receives, so every auto-created spec carries a real title in its body.
  +2. Complete the investigation above and record the evidence for how each existing spec was created.
  +3. Confirm whether manually-authored specs bypass the pre-creation duplicate detector, and if so, decide whether `ff` should reject/gate specs whose `spec_id` collides with an existing one at write time.
  +
  +## Acceptance Criteria
  +- Auto-created specs contain a `# Title` H1 heading and the ledger `title` equals the human title, not the `spec_id`.
  +- The Investigation section is complete: every existing spec is classified as ff-created or manually-authored, with cited evidence.
  +- No spec files share a colliding `spec_id` that should have been caught by the pre-creation duplicate detector.
  +- This spec is additive and does not alter the Test404Reconciliation fix (SPEC-1784146853).
  diff --git a/specs/fix Test404Reconciliation failing on GitHub API 404.md b/specs/fix Test404Reconciliation failing on GitHub API 404.md
  deleted file mode 100644
  index 301a349..0000000
  --- a/specs/fix Test404Reconciliation failing on GitHub API 404.md	
  +++ /dev/null
  @@ -1,11 +0,0 @@
  ----
  -spec_id: "SPEC-1784101386"
  -status: draft
  -repo_issue: ""
  -type: bug
  -version: "0.9.6"
  -root_cause: ""
  -resolution: ""
  -linked_commits: []
  ----
  -Fix Test404Reconciliation failing on GitHub API 404
  diff --git a/specs/version-fix-action-plan.md b/specs/version-fix-action-plan.md
  new file mode 100644
  index 0000000..a597f81
  --- /dev/null
  +++ b/specs/version-fix-action-plan.md
  @@ -0,0 +1,42 @@
  +# Version Fix Action Plan
  +
  +## Objective
  +Fix the version inconsistency where `ff archive` prints the hardcoded const (`0.9.0`) while `ff -v` prints the DB version (`0.9.6`).
  +
  +## Root Cause
  +The code is split between two sources:
  +- `engine.Version` (hardcoded const in flags.go: `Version = "0.9.0"`)
  +- `VersionManager.CurrentVersion()` (reads from DB/ledger, returns `0.9.6`)
  +
  +Locations using `engine.Version` (const):
  +- `main.go:62` - per-command banner
  +- `flags.go:156` - PrintHelp
  +- `flags.go:159` - PrintVersion
  +- `command_dispatcher.go:111` - handleVersion() (but fixes to use VM)
  +- `command_dispatcher.go:156` - checkAndPromptUpdate() 
  +- `cmd_spec.go:158` - writeSpecFromTemplate parameter
  +- `execute.go:283` - writeSpecFromTemplate parameter
  +- Probably other locations
  +
  +## Solution
  +1. Create `engine.CurrentVersion()` - unified version source using VersionManager
  +2. Replace ALL direct `engine.Version` usage with `engine.CurrentVersion()`
  +3. Remove `Version` const from engine/flags.go
  +4. Update checkAndPromptUpdate to compare using CurrentVersion()
  +5. Ensure all version displays use the same canonical source
  +
  +## Implementation Files
  +- engine/flags.go: remove Version const, add CurrentVersion() function
  +- engine/command_dispatcher.go: use CurrentVersion in handleVersion and checkAndPromptUpdate
  +- engine/cmd_spec.go: use CurrentVersion instead of Version for specVersion
  +- engine/execute.go: use CurrentVersion instead of Version for specVersion  
  +- main.go: use CurrentVersion instead of Version for banner
  +- Possibly engine/ledger.go - remove Version field
  +- Possibly other files that use engine.Version
  +
  +## Testing Strategy
  +1. Ensure `ff -v` still works (should still print version)
  +2. Ensure `ff archive` banner uses same version as `ff -v`
  +3. Ensure checkAndPromptUpdate compares against correct version
  +4. Ensure spec creation uses correct version from DB
  +5. Run existing tests to verify no regressions
  diff --git a/specs/version-fix.log b/specs/version-fix.log
  new file mode 100644
  index 0000000..cce2011
  --- /dev/null
  +++ b/specs/version-fix.log
  @@ -0,0 +1 @@
  +Adding version manager as the canonical source and removing hardcoded Version usage
---

## Objective
Fix failing test `TestDeleteSpec_FullIntegration` (ledger_test.go) — `DeleteSpec` must return the persisted `repoID` (55) after a save/reload cycle.

## Root Cause
`SaveLedger` wrote only to SQLite when `OpenDB` succeeded and skipped mirroring to the legacy JSON ledger file. The reload path (`LoadLedger`) read a store that did not reflect the persisted `repo_issue_id` (55), so `DeleteSpec` returned a stale/zero-derived value (42) instead of the expected 55.

## Resolution
Commit `ba00b4e` added a JSON mirror (`ledger.SaveToFile`) inside `SaveLedger` before the SQLite write, keeping `forgefix_ledger.json` consistent with the canonical SQLite store. `LoadLedger` now reloads `repo_issue_id=55` correctly and the test passes.

## Failure Details
- Test: TestDeleteSpec_FullIntegration
- File: ledger_test.go
- Line: 451
- Error: expected repoID 55, got 42
- Status: RESOLVED — test passes on current HEAD.

