package engine

import (
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
