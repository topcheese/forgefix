---spec_id: "SPEC-1784102178"
status: review
repo_issue: 535
type: bug
version: "0.9.6"
root_cause: "main.go:62 prints engine.Version (the compile-time const flags.Version = \"0.9.0\" from flags.go:10) as the banner on EVERY command except version/help. ff -v instead calls handleVersion() which reads the canonical version from the DB via VersionManager.CurrentVersion() (0.9.6). So the per-command banner uses a stale hardcoded constant while the version command uses the real DB value. checkAndPromptUpdate (command_dispatcher.go:144,147) also compares against the const, so the update checker uses the wrong baseline too."
 ""
linked_commits: ["e3bbe4e"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index c31bc84..b89e381 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -1,3 +1,8 @@
  +## [Unreleased] - 2026-07-16
  +
  +### 🚀 Release Summary
  +- feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)
  +
   ## [Unreleased] - 2026-07-15
   
   ### 🚀 Release Summary
  diff --git a/engine/cmd_spec.go b/engine/cmd_spec.go
  index 11438a6..4978b6f 100644
  --- a/engine/cmd_spec.go
  +++ b/engine/cmd_spec.go
  @@ -154,10 +154,7 @@ func createSpec(configDir, name, bodyContent string, d *CommandDispatcher, flags
   	}
   
   	// Resolve version from DB (not the compile-time const).
  -	specVersion := Version
  -	if pv := NewVersionManager(configDir).CurrentVersion(); pv != "0.0.0" {
  -		specVersion = pv
  -	}
  +	specVersion := CurrentVersion(configDir)
   	if flags.SpecVersion != "" {
   		specVersion = flags.SpecVersion
   	}
  diff --git a/engine/command_dispatcher.go b/engine/command_dispatcher.go
  index 77d2510..6e5cae1 100644
  --- a/engine/command_dispatcher.go
  +++ b/engine/command_dispatcher.go
  @@ -108,7 +108,7 @@ func (d *CommandDispatcher) handleHelp() CommandResult {
   // The displayed version comes from the project DB (the canonical source),
   // falling back to the compile-time const when no project context exists.
   func (d *CommandDispatcher) handleVersion() CommandResult {
  -	version := Version
  +	version := CurrentVersion(d.ConfigDir)
   	if vm := NewVersionManager(d.ConfigDir); vm != nil {
   		if pv := vm.CurrentVersion(); pv != "0.0.0" {
   			version = pv
  @@ -141,10 +141,10 @@ func checkAndPromptUpdate(configDir string, stdout, stderr io.Writer, aiMode boo
   	if release.TagName == "" {
   		return
   	}
  -	if semver.Compare(release.TagName, "v"+Version) <= 0 {
  +	if semver.Compare(release.TagName, "v"+CurrentVersion(configDir)) <= 0 {
   		return
   	}
  -	fmt.Fprintf(stdout, "New version available: %s (current: %s)\n", release.TagName, Version)
  +	fmt.Fprintf(stdout, "New version available: %s (current: %s)\n", release.TagName, CurrentVersion(configDir))
   	if aiMode {
   		return
   	}
  diff --git a/engine/execute.go b/engine/execute.go
  index bbd42ba..2278167 100644
  --- a/engine/execute.go
  +++ b/engine/execute.go
  @@ -278,10 +278,7 @@ func handleDetonationIssues(d *Dashboard, configDir string) {
   			body := fmt.Sprintf("## Objective\nAutomatically created from failing test %s during ff --ai run.\n\n## Root Cause\nTest failed - see failure details below.\n\n## Failure Details\n- Test: %s\n- File: %s\n- Line: %d\n- Error: %s",
   				info.Name, info.Name, info.FilePath, info.FailureLine, info.ErrorTrace)
   
  -			specVersion := Version
  -			if pv := NewVersionManager(configDir).CurrentVersion(); pv != "0.0.0" {
  -				specVersion = pv
  -			}
  +			specVersion := CurrentVersion(configDir)
   			sid, specPath, werr := writeSpecFromTemplate(configDir, title, title, body, "bug", specVersion, "")
   			if werr != nil {
   				d.AddSystemError(fmt.Sprintf("failed to create spec for %s: %v", info.Name, werr))
  diff --git a/engine/flags.go b/engine/flags.go
  index 431b6d5..25ec56b 100644
  --- a/engine/flags.go
  +++ b/engine/flags.go
  @@ -7,8 +7,27 @@ import (
   	"strings"
   )
   
  +// Version is the compile-time fallback, used ONLY as the last-resort value by
  +// CurrentVersion() when no project context (DB/ledger) is available. No display
  +// or update-check path may reference this const directly — call
  +// CurrentVersion(configDir) instead so every command reports the same version.
   const Version = "0.9.0"
   
  +// CurrentVersion returns the canonical ForgeFix version for a project.
  +// Resolution order: project DB meta.version -> legacy ledger version -> the
  +// compile-time Version const (last-resort fallback only). Pass an empty configDir to
  +// skip the DB/ledger lookup and return the const directly (non-project context).
  +func CurrentVersion(configDir string) string {
  +	if configDir != "" {
  +		if vm := NewVersionManager(configDir); vm != nil {
  +			if pv := vm.CurrentVersion(); pv != "" && pv != "0.0.0" {
  +				return pv
  +			}
  +		}
  +	}
  +	return Version
  +}
  +
   type CLIArgs struct {
   	AIMode           bool
   	Help             bool
  @@ -156,5 +175,5 @@ Examples:
   }
   
   func PrintVersion(w io.Writer) {
  -	fmt.Fprintf(w, "ForgeFix %s\n", Version)
  +	fmt.Fprintf(w, "ForgeFix %s\n", CurrentVersion(""))
   }
  diff --git a/main.go b/main.go
  index a9c3958..4026327 100644
  --- a/main.go
  +++ b/main.go
  @@ -59,7 +59,7 @@ func main() {
   	}
   
   	if cmd != "version" && cmd != "-v" && cmd != "--version" && cmd != "help" && cmd != "--help" && cmd != "" {
  -		fmt.Fprintf(os.Stderr, "ForgeFix %s\n", engine.Version)
  +		fmt.Fprintf(os.Stderr, "ForgeFix %s\n", engine.CurrentVersion(projectRoot))
   	}
   
   	disp := engine.NewCommandDispatcher(projectRoot, wd, os.Stdout, os.Stderr)
  diff --git a/specs/Fix inconsistent version display audit all sources.md b/specs/Fix inconsistent version display audit all sources.md
  index e1f6c7a..a081869 100644
  --- a/specs/Fix inconsistent version display audit all sources.md	
  +++ b/specs/Fix inconsistent version display audit all sources.md	
  @@ -1,6 +1,6 @@
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

