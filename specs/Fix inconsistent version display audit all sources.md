---
spec_id: "SPEC-1784102178"
status: ship
repo_issue: 535
type: bug
version: "0.9.6"
root_cause: "main.go prints engine.Version (hardcoded const 0.9.0) as the banner on every command. ff -v reads from DB via CurrentVersion() (0.9.6). checkAndPromptUpdate also compares against the const, using the wrong baseline."
resolution: "Unified all version display to use CurrentVersion(). main.go banner, handleVersion, and checkAndPromptUpdate now all call CurrentVersion() instead of the const."
linked_commits: ["e3bbe4e"]
---

# Fix Inconsistent Version Display Across `ff` Commands

## Objective

`ff archive` prints `ForgeFix 0.9.0` while `ff -v` prints `ForgeFix 0.9.6`.
The same tool reports two different versions depending on the command. This
spec fixes the inconsistency AND audits every place `ff` displays or compares
its version, so no command or process can ever present a wrong/stale version
again.

## Root Cause (from code reading)

- `flags.go:10` — `const Version = "0.9.0"` (hardcoded compile-time constant).
- `main.go:62` — for every command except `version`/`help`/`""`, prints
  `fmt.Fprintf(os.Stderr, "ForgeFix %s\n", engine.Version)` using that const.
  This is the banner shown by `ff archive`, `ff sync`, `ff ship`, etc.
- `command_dispatcher.go:110-118` (`handleVersion`) — reads the canonical
  version from the DB via `VersionManager.CurrentVersion()` (DB → ledger →
  `0.9.6`). Correct source, but only used by `ff -v` / `ff version`.
- `command_dispatcher.go:144,147` (`checkAndPromptUpdate`) — compares the
  latest remote release against the const `Version`, not the DB value, so the
  "new version available" check uses the wrong baseline.
- `flags.go:156` (`PrintHelp`) — also prints `ForgeFix %s` with the const.

Net: there are **at least four** version-display/compare sites, split between
the hardcoded const and the DB. They disagree whenever the DB version differs
from `0.9.0`.

## Requirements

### 1. Unify on a single canonical version source
- Define ONE function (e.g. `engine.CurrentVersion()` or reuse
  `VersionManager.CurrentVersion()`) that every display/compare site calls.
- The canonical source is, in order: project DB `meta.version` → legacy ledger
  `version` → compile-time const as last-resort fallback only.
- Remove direct use of the `Version` const from all display paths.

### 2. Audit ALL version-presenting commands and processes
Enumerate and verify each of the following presents the SAME correct version:
- `ff -v` / `ff version` (already correct — must stay correct)
- The per-command banner (`ff archive`, `ff sync`, `ff ship`, `ff spec`,
  `ff commit`, `ff status`, `ff backlog`, `ff archive`, `ff kanban`, etc.)
- `ff --help` / `PrintHelp`
- The update checker (`checkAndPromptUpdate`) baseline comparison
- Any process/log header that stamps a version (housekeeper, background sync,
  audit logging)
- The `ff` binary's reported version in CI / install checks

### 3. Add a guard against regression
- A test that asserts `ff -v` and the `ff <any-command>` banner report the
  same version when the DB holds a non-const value (e.g. set DB version to
  something other than `0.9.0` and assert both outputs match).
- Optionally fail CI if the const drifts from the DB on a release build.

## Acceptance Criteria

- `ff archive` (and every other command) prints the same version as `ff -v`
  (the DB/ledger canonical value, e.g. `0.9.6`), not the hardcoded `0.9.0`.
- No code path prints or compares the `Version` const directly for display
  or update-check purposes.
- Audit table (Requirement 2) is complete: every listed command/process
  verified to present the identical correct version.
- Regression test added and passing: banner == `ff -v` output under a
  non-default DB version.
- The 13 pre-existing failing integration tests remain untouched (this spec
  is additive and does not fix them).
