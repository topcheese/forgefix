package engine

import (
	"os"
	"strings"
	"testing"
)

func TestCommandDispatcher_HelpCommandWritesToWriter(t *testing.T) {
	var buf strings.Builder
	d := NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	result, err := d.Execute("help", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ForgeFix") {
		t.Errorf("help output should contain 'ForgeFix', got:\n%s", buf.String())
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestCommandDispatcher_HelpViaDashDashHelp(t *testing.T) {
	var buf strings.Builder
	d := NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	result, err := d.Execute("--help", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ForgeFix") {
		t.Errorf("help output should contain 'ForgeFix', got:\n%s", buf.String())
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestCommandDispatcher_VersionCommandWritesToWriter(t *testing.T) {
	var buf strings.Builder
	d := NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	result, err := d.Execute("version", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ForgeFix") {
		t.Errorf("version output should contain 'ForgeFix', got:\n%s", buf.String())
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestCommandDispatcher_VersionViaShortFlag(t *testing.T) {
	var buf strings.Builder
	d := NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	result, err := d.Execute("-v", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "ForgeFix") {
		t.Errorf("version output should contain 'ForgeFix', got:\n%s", buf.String())
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestCommandDispatcher_UnknownCommandReturnsError(t *testing.T) {
	var buf strings.Builder
	d := NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	_, err := d.Execute("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

func TestNewCommandDispatcherSetsFields(t *testing.T) {
	d := NewCommandDispatcher("/cfg", "/work", nil, nil)
	if d.ConfigDir != "/cfg" {
		t.Errorf("ConfigDir = %q, want %q", d.ConfigDir, "/cfg")
	}
	if d.WorkDir != "/work" {
		t.Errorf("WorkDir = %q, want %q", d.WorkDir, "/work")
	}
	if d.Stdout != nil {
		t.Errorf("Stdout should be nil, got %v", d.Stdout)
	}
	if d.Stderr != nil {
		t.Errorf("Stderr should be nil, got %v", d.Stderr)
	}
}

func TestCommandDispatcher_ArchiveOnMissingSpecsDir(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("archive", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A missing specs/ dir is an error condition in ArchiveResolvedSpecs
	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode 1 for missing specs dir, got %d", result.ExitCode)
	}
	if !strings.Contains(stderr.String(), "reading specs directory") {
		t.Errorf("expected stderr to mention missing specs dir, got: %s", stderr.String())
	}
}

func TestCommandDispatcher_ArchiveNoResolvedSpecs(t *testing.T) {
	var stdout, stderr strings.Builder
	tmpDir := t.TempDir()
	// Create an empty specs/ dir so ArchiveResolvedSpecs runs without I/O error
	specsDir := tmpDir + "/specs"
	if err := os.Mkdir(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	result, err := d.Execute("archive", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No resolved specs to archive.") {
		t.Errorf("expected no-op message, got stdout: %s", stdout.String())
	}
}
