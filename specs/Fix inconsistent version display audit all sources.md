---spec_id: "SPEC-1784102178"
status: review
repo_issue: 535
type: bug
version: "0.9.6"
root_cause: "main.go:62 prints engine.Version (the compile-time const flags.Version = \"0.9.0\" from flags.go:10) as the banner on EVERY command except version/help. ff -v instead calls handleVersion() which reads the canonical version from the DB via VersionManager.CurrentVersion() (0.9.6). So the per-command banner uses a stale hardcoded constant while the version command uses the real DB value. checkAndPromptUpdate (command_dispatcher.go:144,147) also compares against the const, so the update checker uses the wrong baseline too."
linked_commits: ["e3bbe4e"]
---
   spec_id: "SPEC-1784102178"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
  diff --git a/version_consistency_test.go b/version_consistency_test.go
  new file mode 100644
  index 0000000..0fc3cab
  --- /dev/null
  +++ b/version_consistency_test.go
  @@ -0,0 +1,67 @@
  +package main
  +
  +import (
  +	"bytes"
  +	"strings"
  +	"testing"
  +
  +	"ForgeFix/engine"
  +)
  +
  +// TestBannerMatchesVersionCommand is the regression guard for SPEC-1784102178.
  +//
  +// It asserts that the per-command banner (main.go prints
  +// "ForgeFix %s" with engine.CurrentVersion(projectRoot)) reports the SAME
  +// version as `ff -v`/`ff version` (command_dispatcher.handleVersion prints
  +// "ForgeFix %s" with CurrentVersion(d.ConfigDir)), even when the project DB
  +// holds a version that differs from the compile-time const fallback.
  +//
  +// Before this fix, the banner used the hardcoded const ("0.9.0") while
  +// `ff -v` used the DB value ("0.9.6"), so the tool reported two versions.
  +func TestBannerMatchesVersionCommand(t *testing.T) {
  +	const want = "9.9.9" // intentionally NOT the compile-time const "0.9.0"
  +
  +	tmp := t.TempDir()
  +	if _, err := engine.InitConfig(tmp); err != nil {
  +		t.Fatalf("InitConfig: %v", err)
  +	}
  +
  +	// Set a non-default project version in the DB (the canonical source).
  +	vm := engine.NewVersionManager(tmp)
  +	if vm == nil {
  +		t.Fatalf("NewVersionManager returned nil")
  +	}
  +	if err := vm.WriteVersion(want); err != nil {
  +		t.Fatalf("WriteVersion: %v", err)
  +	}
  +
  +	// The per-command banner uses engine.CurrentVersion(projectRoot).
  +	bannerVer := engine.CurrentVersion(tmp)
  +	if bannerVer != want {
  +		t.Fatalf("banner version source returned %q, want %q", bannerVer, want)
  +	}
  +	// Sanity: the DB value must not be the compile-time const fallback.
  +	if bannerVer == engine.Version {
  +		t.Fatalf("banner fell back to compile-time const %q instead of DB value %q", engine.Version, want)
  +	}
  +
  +	// `ff -v` / `ff version` via the dispatcher.
  +	var out bytes.Buffer
  +	d := engine.NewCommandDispatcher(tmp, tmp, &out, &out)
  +	res, err := d.Execute("version", nil)
  +	if err != nil {
  +		t.Fatalf("Execute(version): %v", err)
  +	}
  +	if res.ExitCode != 0 {
  +		t.Fatalf("version command exited %d: %s", res.ExitCode, out.String())
  +	}
  +	versionOut := out.String()
  +	if !strings.Contains(versionOut, "ForgeFix "+bannerVer) {
  +		t.Fatalf("version output = %q, want it to contain %q", versionOut, "ForgeFix "+bannerVer)
  +	}
  +
  +	// The banner and `ff -v` must agree exactly.
  +	if !strings.Contains("ForgeFix "+bannerVer, strings.TrimSpace(strings.TrimPrefix(versionOut, "ForgeFix "))) {
  +		t.Fatalf("banner version %q does not match `ff -v` output %q", bannerVer, versionOut)
  +	}
  +}
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
