package engine

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoStageAndCommit(t *testing.T) {
	tmpDir := t.TempDir()

	initGitRepo(t, tmpDir)

	// Create a dummy file and make an initial commit
	initialFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(initialFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Modify the file WITHOUT staging
	if err := os.WriteFile(initialFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := AutoStageAndCommit(tmpDir, "test: [SPEC-TEST] auto-stage works")
	if err != nil {
		t.Fatalf("AutoStageAndCommit failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty commit hash")
	}

	// Verify via git log
	logOut := runGit(t, tmpDir, "log", "--oneline", "-1")
	if !strings.Contains(logOut, hash) {
		t.Errorf("expected git log to contain hash %q, got: %s", hash, logOut)
	}
	if !strings.Contains(logOut, "test: [SPEC-TEST] auto-stage works") {
		t.Errorf("expected git log to contain commit message, got: %s", logOut)
	}
}

func TestAutoStageAndCommitNoChanges(t *testing.T) {
	tmpDir := t.TempDir()

	initGitRepo(t, tmpDir)

	initialFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(initialFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// No changes — should return "no changes to commit"
	_, err := AutoStageAndCommit(tmpDir, "should not commit")
	if err == nil {
		t.Fatal("expected error for no changes, got nil")
	}
	if !strings.Contains(err.Error(), "no changes to commit") {
		t.Errorf("expected 'no changes to commit' error, got: %v", err)
	}
}

func TestRunCommit_MessageDedup(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-123.md")
	specContent := `---
spec_id: "SPEC-123"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// User passes message that already contains the spec ID tag — dedup should
	// strip [SPEC-123] before the function prepends "feat: [SPEC-123]"
	hash, specID, commitMsg, err := runCommit(tmpDir, "implement feature [SPEC-123]", "SPEC-123", "", "", false, d)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty commit hash")
	}
	if specID != "SPEC-123" {
		t.Errorf("expected SPEC-123, got %s", specID)
	}
	if commitMsg != "feat: [SPEC-123] implement feature" {
		t.Errorf("expected deduped commit msg without double spec tag, got: %q", commitMsg)
	}
	logOut := runGit(t, tmpDir, "log", "--oneline", "-1")
	if !strings.Contains(logOut, "feat: [SPEC-123] implement feature") {
		t.Errorf("expected git log to contain deduped message, got: %s", logOut)
	}
}

func TestRunCommit_DedupPreservesNonSpecContent(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-456"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-456.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "x.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-456] add new feature", "SPEC-456", "", "", false, d)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if commitMsg != "feat: [SPEC-456] add new feature" {
		t.Errorf("expected deduped msg, got: %q", commitMsg)
	}
}

func TestRunCommit_DedupPreservesOtherSpecRefs(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-111"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-111.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// Committing to SPEC-111, message references SPEC-456 and SPEC-789
	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-111] integrate with [SPEC-456] and [SPEC-789]", "SPEC-111", "", "", false, d)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if commitMsg != "feat: [SPEC-111] integrate with [SPEC-456] and [SPEC-789]" {
		t.Errorf("expected other spec refs preserved, got: %q", commitMsg)
	}
	logOut := runGit(t, tmpDir, "log", "--oneline", "-1")
	if !strings.Contains(logOut, "integrate with [SPEC-456] and [SPEC-789]") {
		t.Errorf("expected git log to preserve other spec refs, got: %s", logOut)
	}
}

func TestRunCommit_DedupOnlyTagNoBody(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-999.md")
	specContent := `---
spec_id: "SPEC-999"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "z.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-999]", "SPEC-999", "", "", false, d)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	// Must not repeat the tag: "feat: [SPEC-999] [SPEC-999]"
	if commitMsg != "feat: [SPEC-999]" {
		t.Errorf("expected no doubled tag, got: %q", commitMsg)
	}
	logOut := runGit(t, tmpDir, "log", "--oneline", "-1")
	if !strings.Contains(logOut, "feat: [SPEC-999]") {
		t.Errorf("expected git log to match, got: %s", logOut)
	}
}

func TestRunCommit_DedupNoSpecInMessage(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-789"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-789.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "y.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, commitMsg, err := runCommit(tmpDir, "implement feature", "SPEC-789", "", "", false, d)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if commitMsg != "feat: [SPEC-789] implement feature" {
		t.Errorf("expected clean format, got: %q", commitMsg)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@forgefix.dev")
	runGit(t, dir, "config", "user.name", "ForgeFix Test")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
