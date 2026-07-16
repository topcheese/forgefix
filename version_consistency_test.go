package main

import (
	"bytes"
	"strings"
	"testing"

	"ForgeFix/engine"
)

// TestBannerMatchesVersionCommand is the regression guard for SPEC-1784102178.
//
// It asserts that the per-command banner (main.go prints
// "ForgeFix %s" with engine.CurrentVersion(projectRoot)) reports the SAME
// version as `ff -v`/`ff version` (command_dispatcher.handleVersion prints
// "ForgeFix %s" with CurrentVersion(d.ConfigDir)), even when the project DB
// holds a version that differs from the compile-time const fallback.
//
// Before this fix, the banner used the hardcoded const ("0.9.0") while
// `ff -v` used the DB value ("0.9.6"), so the tool reported two versions.
func TestBannerMatchesVersionCommand(t *testing.T) {
	const want = "9.9.9" // intentionally NOT the compile-time const "0.9.0"

	tmp := t.TempDir()
	if _, err := engine.InitConfig(tmp); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Set a non-default project version in the DB (the canonical source).
	vm := engine.NewVersionManager(tmp)
	if vm == nil {
		t.Fatalf("NewVersionManager returned nil")
	}
	if err := vm.WriteVersion(want); err != nil {
		t.Fatalf("WriteVersion: %v", err)
	}

	// The per-command banner uses engine.CurrentVersion(projectRoot).
	bannerVer := engine.CurrentVersion(tmp)
	if bannerVer != want {
		t.Fatalf("banner version source returned %q, want %q", bannerVer, want)
	}
	// Sanity: the DB value must not be the compile-time const fallback.
	if bannerVer == engine.Version {
		t.Fatalf("banner fell back to compile-time const %q instead of DB value %q", engine.Version, want)
	}

	// `ff -v` / `ff version` via the dispatcher.
	var out bytes.Buffer
	d := engine.NewCommandDispatcher(tmp, tmp, &out, &out)
	res, err := d.Execute("version", nil)
	if err != nil {
		t.Fatalf("Execute(version): %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("version command exited %d: %s", res.ExitCode, out.String())
	}
	versionOut := out.String()
	if !strings.Contains(versionOut, "ForgeFix "+bannerVer) {
		t.Fatalf("version output = %q, want it to contain %q", versionOut, "ForgeFix "+bannerVer)
	}

	// The banner and `ff -v` must agree exactly.
	if !strings.Contains("ForgeFix "+bannerVer, strings.TrimSpace(strings.TrimPrefix(versionOut, "ForgeFix "))) {
		t.Fatalf("banner version %q does not match `ff -v` output %q", bannerVer, versionOut)
	}
}
