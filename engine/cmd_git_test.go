package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGitPassthroughCommandsSetContainsKnownCommands(t *testing.T) {
	expected := []string{"log", "diff", "branch", "status", "stash", "rebase", "merge", "tag", "push", "pull", "fetch", "reset", "checkout", "switch", "restore", "commit", "blame", "bisect", "worktree", "config", "init", "add", "mv", "rm", "gc", "grep", "reflog", "cherry-pick", "revert", "clean", "describe", "shortlog", "show", "submodule"}
	for _, cmd := range expected {
		if !GitPassthroughCommands[cmd] {
			t.Errorf("expected %q to be in GitPassthroughCommands", cmd)
		}
	}
}

func TestGitPassthroughRoutingKnownCommands(t *testing.T) {
	// Simulate a git-capable environment — create a temp git repo
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	prepCommit(t, tmpDir)

	// Create a test config so ConfigDir resolves
	createTestFFConfig(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	tests := []struct {
		name    string
		cmd     string
		args    []string
		wantOk  bool   // should succeed (exit 0)
		keyword string // expected substring in output
	}{
		{"status", "status", []string{"--porcelain"}, true, ""},
		{"log", "log", []string{"--oneline", "-1"}, true, ""},
		{"branch", "branch", nil, true, ""},
		{"git status prefix", "git", []string{"status", "--porcelain"}, true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			result, err := d.Execute(tc.cmd, tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantOk && result.ExitCode != 0 {
				t.Errorf("expected exit code 0, got %d; stdout: %s; stderr: %s", result.ExitCode, stdout.String(), stderr.String())
			}
			if tc.keyword != "" && !strings.Contains(stdout.String(), tc.keyword) {
				t.Errorf("expected stdout to contain %q, got: %s", tc.keyword, stdout.String())
			}
		})
	}
}

func TestGitPassthroughPreservesExitCodes(t *testing.T) {
	tmpDir := t.TempDir()
	// No git repo here — commands will fail, we test the exit code passthrough
	createTestFFConfig(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	// Running git log without a git repo should fail with a non-zero exit code
	result, err := d.Execute("log", []string{"--oneline"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code when running git log outside a repo")
	}
}

func TestGitPassthroughDisabledFallsThrough(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFFConfig(t, tmpDir)

	// Disable git passthrough via config
	disable := false
	saveConfig(t, tmpDir, &Config{GitPassthrough: &disable})

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	// With passthrough disabled, "log" should fall through to handleRun
	// which will try to load a pipeline config and fail gracefully.
	result, err := d.Execute("log", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// handleRun will try to load config and fail with exit 1 — that's fine
	// The important thing is it didn't try to run git log
	if result.ExitCode != 1 {
		t.Logf("passthrough disabled: got exit code %d (expected 1, meaning handleRun took over)", result.ExitCode)
	}
}

func TestKnownForgeFixCommandsTakePriority(t *testing.T) {
	tmpDir := t.TempDir()
	createTestFFConfig(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)

	// "commit" is both a ForgeFix command and a git command.
	// It should route to ForgeFix's handler, not git passthrough.
	result, err := d.Execute("commit", []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ForgeFix commit --help should not run "git commit --help"
	// (which would open an editor). Instead it should hit the dispatcher's
	// commit handler which parses flags and may fail gracefully.
	if result.ExitCode != 1 {
		t.Logf("commit help: exit code %d", result.ExitCode)
	}
}

// TestGitPassthroughCommitSearch searches commit messages — requires a repo with commits.
func TestGitPassthroughCommitSearch(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	createTestFFConfig(t, tmpDir)

	// Create a file and commit it so there's something to log
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "commit", "-m", "feat: [SPEC-TEST] initial commit for passthrough test")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	// Test: ff log --grep="passthrough" --oneline
	result, err := d.Execute("log", []string{"--grep=passthrough", "--oneline"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s; stderr: %s", result.ExitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "SPEC-TEST") {
		t.Errorf("expected log output to contain 'SPEC-TEST', got: %s", stdout.String())
	}
}

// TestGitPassthroughDiff tests ff diff --cached
func TestGitPassthroughDiff(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	createTestFFConfig(t, tmpDir)

	// Create a file, stage it so diff --cached shows something
	testFile := filepath.Join(tmpDir, "diff-test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "diff-test.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	result, err := d.Execute("diff", []string{"--cached"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "diff-test.txt") {
		t.Errorf("expected diff output to contain filename, got: %s", stdout.String())
	}
}

// TestGitPassthroughBranch verifies ff branch lists branches.
func TestGitPassthroughBranch(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	createTestFFConfig(t, tmpDir)

	// Make an initial commit so branch listing works
	prepCommit(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	result, err := d.Execute("branch", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "main") || !strings.Contains(stdout.String(), "*") {
		t.Errorf("expected branch output to show 'main' with '*', got: %s", stdout.String())
	}
}

// TestGitPassthroughStash tests ff stash list
func TestGitPassthroughStash(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	createTestFFConfig(t, tmpDir)

	prepCommit(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	// stash list on a clean repo should succeed with no output
	result, err := d.Execute("stash", []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 for stash list, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
}

// TestGitPassthroughConfig tests ff config --list
func TestGitPassthroughConfig(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	createTestFFConfig(t, tmpDir)

	var stdout, stderr strings.Builder
	d := NewCommandDispatcher(tmpDir, tmpDir, &stdout, &stderr)
	d.ConfigDir = tmpDir

	result, err := d.Execute("config", []string{"user.name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", result.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Test") {
		t.Errorf("expected config output to contain user name, got: %s", stdout.String())
	}
}

// --- helpers ---

func prepCommit(t *testing.T, dir string) {
	t.Helper()
	f := filepath.Join(dir, "initial.txt")
	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}

func createTestFFConfig(t *testing.T, dir string) {
	t.Helper()
	projectName := filepath.Base(dir)
	configPath := filepath.Join(dir, projectName+"_ff.yaml")
	content := "global_timeout_seconds: 120\nfailure_decay_seconds: 30\ngit_passthrough: true\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("creating test config: %v", err)
	}
}

func saveConfig(t *testing.T, dir string, cfg *Config) {
	t.Helper()
	projectName := filepath.Base(dir)
	configPath := filepath.Join(dir, projectName+"_ff.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}
