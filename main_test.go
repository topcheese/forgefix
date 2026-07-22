package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ForgeFix/engine"
)

func TestParseFlagsHandlesAllSemanticFlags(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantAI   bool
		wantHelp bool
		wantVer  bool
		wantMsg  string
	}{
		{"empty", []string{}, false, false, false, ""},
		{"--ai", []string{"--ai"}, true, false, false, ""},
		{"--help", []string{"--help"}, false, true, false, ""},
		{"-h", []string{"-h"}, false, true, false, ""},
		{"--version", []string{"--version"}, false, false, true, ""},
		{"-v", []string{"-v"}, false, false, true, ""},
		{"--message with value", []string{"--message", "fix: resolve timeout"}, false, false, false, "fix: resolve timeout"},
		{"-m with value", []string{"-m", "chore: update deps"}, false, false, false, "chore: update deps"},
		{"out of order", []string{"--version", "--ai", "--message", "msg"}, true, false, true, "msg"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := engine.ParseFlags(tc.args)

			if flags.AIMode != tc.wantAI {
				t.Errorf("AIMode: got %v, want %v", flags.AIMode, tc.wantAI)
			}
			if flags.Help != tc.wantHelp {
				t.Errorf("Help: got %v, want %v", flags.Help, tc.wantHelp)
			}
			if flags.Version != tc.wantVer {
				t.Errorf("Version: got %v, want %v", flags.Version, tc.wantVer)
			}
			if flags.Message != tc.wantMsg {
				t.Errorf("Message: got %q, want %q", flags.Message, tc.wantMsg)
			}
		})
	}
}

func TestPrintHelpContainsExpectedSections(t *testing.T) {
	var buf strings.Builder
	d := engine.NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	d.Execute("help", nil)
	output := buf.String()

	sections := []string{"ForgeFix", "--ai", "--help", "--version", "--message"}
	for _, s := range sections {
		if !strings.Contains(output, s) {
			t.Errorf("expected help text to contain %q", s)
		}
	}
}

func TestPrintVersionContainsSemVer(t *testing.T) {
	var buf strings.Builder
	d := engine.NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
	d.Execute("version", nil)
	output := buf.String()
	if !strings.Contains(output, engine.Version) {
		t.Errorf("expected version output to contain %q, got: %s", engine.Version, output)
	}
}

// createTestConfig writes a minimal _ff.yaml in dir so that SpecConfigDir
// and other discovery functions resolve correctly.
func createTestConfig(t *testing.T, dir string) {
	t.Helper()
	projectName := filepath.Base(dir)
	configPath := filepath.Join(dir, projectName+"_ff.yaml")
	content := "global_timeout_seconds: 120\nfailure_decay_seconds: 30\n"
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("creating test config: %v", err)
	}
}

func TestPromptForSpecSelectionNewBug(t *testing.T) {
	tmpDir := t.TempDir()
	createTestConfig(t, tmpDir)

	ffBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("getting test binary: %v", err)
	}
	if err := copyFile(ffBinary, filepath.Join(tmpDir, "ff")); err != nil {
		t.Fatalf("copying ff binary: %v", err)
	}

	ledgerDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("creating ledger dir: %v", err)
	}

	ledger := engine.NewLedgerEngine()
	// Add a dummy spec so we go through categorical selection
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-EXISTING",
		RepoIssueID:   1,
		Status:        "draft",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-EXISTING", entry)
	if err := engine.SaveLedger(ledger, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	// Category: 4 (All), Spec: 0 (New Bug), Title
	input := "4\n0\nNew Bug Title\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Write([]byte(input))
		w.Close()
	}()

	specID, err := engine.SelectSpec(tmpDir, true)
	<-done
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("SelectSpec returned error: %v", err)
	}
	if !strings.HasPrefix(specID, "SPEC-") || !strings.HasSuffix(specID, "-BUG") {
		t.Errorf("expected new bug spec ID format, got: %s", specID)
	}

	specFiles, _ := filepath.Glob(filepath.Join(tmpDir, "specs", "*.md"))
	if len(specFiles) != 1 {
		t.Errorf("expected 1 spec file, got %d", len(specFiles))
	}
}

func TestPromptForSpecSelectionExisting(t *testing.T) {
	tmpDir := t.TempDir()
	createTestConfig(t, tmpDir)

	ffBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("getting test binary: %v", err)
	}
	if err := copyFile(ffBinary, filepath.Join(tmpDir, "ff")); err != nil {
		t.Fatalf("copying ff binary: %v", err)
	}

	ledgerDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("creating ledger dir: %v", err)
	}

	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := engine.SaveLedger(ledger, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	// Category: 4 (All), Spec: 1 (SPEC-123)
	input := "4\n1\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(input))
		w.Close()
	}()

	specID, err := engine.SelectSpec(tmpDir, true)
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("SelectSpec returned error: %v", err)
	}
	if specID != "SPEC-123" {
		t.Errorf("expected SPEC-123, got: %s", specID)
	}
}

func TestPromptForSpecSelectionSkip(t *testing.T) {
	tmpDir := t.TempDir()
	createTestConfig(t, tmpDir)

	ffBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("getting test binary: %v", err)
	}
	if err := copyFile(ffBinary, filepath.Join(tmpDir, "ff")); err != nil {
		t.Fatalf("copying ff binary: %v", err)
	}

	ledgerDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("creating ledger dir: %v", err)
	}

	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := engine.SaveLedger(ledger, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	// Category: Enter (skip)
	input := "\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(input))
		w.Close()
	}()

	specID, err := engine.SelectSpec(tmpDir, true)
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("SelectSpec returned error: %v", err)
	}
	if specID != "" {
		t.Errorf("expected empty string for Enter selection, got: %s", specID)
	}
}

func TestRunCommitWithFlagSpecID(t *testing.T) {
	// Test that -s flag bypasses the interactive menu
	tmpDir := t.TempDir()
	createTestConfig(t, tmpDir)

	ffBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("getting test binary: %v", err)
	}
	if err := copyFile(ffBinary, filepath.Join(tmpDir, "ff")); err != nil {
		t.Fatalf("copying ff binary: %v", err)
	}

	ledgerDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("creating ledger dir: %v", err)
	}

	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := engine.SaveLedger(ledger, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	// Create spec file in specs/ directory (required by loadActiveSpecs)
	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("creating specs dir: %v", err)
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
		t.Fatalf("creating spec file: %v", err)
	}

	// Create a test file to commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Stage the test file and commit directly via engine functions
	commitHash, err := engine.AutoStageAndCommit(tmpDir, "feat: [SPEC-123] test commit message")
	if err != nil {
		t.Fatalf("AutoStageAndCommit failed: %v", err)
	}

	if err := engine.UpdateLedgerAfterCommit(tmpDir, "SPEC-123", commitHash); err != nil {
		t.Fatalf("UpdateLedgerAfterCommit failed: %v", err)
	}

	// Verify commit was created with SPEC-123
	cmd = exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = tmpDir
	output2, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(output2), "SPEC-123") {
		t.Errorf("expected commit to contain SPEC-123, got: %s", string(output2))
	}

	// Verify ledger was updated
	ledger2, err := engine.LoadLedger(tmpDir)
	if err != nil {
		t.Fatalf("loading ledger: %v", err)
	}
	specEntry := ledger2.GetSpecEntry("SPEC-123")
	if specEntry == nil {
		t.Fatal("SPEC-123 not found in ledger")
	}
	if specEntry.Status != "in-progress" {
		t.Errorf("expected status to remain in-progress after plain commit (promotion requires --review), got: %s", specEntry.Status)
	}

	// Verify spec file on disk was also updated (status unchanged; promotion requires --review flag)
	if data, err := os.ReadFile(specFile); err == nil {
		if strings.Contains(string(data), "status: review") {
			t.Errorf("spec file should NOT be promoted to review on a plain commit (use --review), got:\n%s", string(data))
		}
	} else {
		t.Errorf("reading spec file: %v", err)
	}
	if len(specEntry.LinkedCommits) == 0 {
		t.Error("expected linked commits, got none")
	}
}

func TestUpdateLedgerAfterCommit_SpecFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	createTestConfig(t, tmpDir)

	ledgerDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ledgerDir, 0755); err != nil {
		t.Fatalf("creating ledger dir: %v", err)
	}

	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := engine.SaveLedger(ledger, tmpDir); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	// Do NOT create spec file on disk — should get an error
	// Create a .git dir so findGitRootWalk succeeds
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	err := engine.UpdateLedgerAfterCommit(tmpDir, "SPEC-123", "abc123")
	if err == nil {
		t.Fatal("expected error when spec file not found, got nil")
	}
	if !strings.Contains(err.Error(), "spec file not found") {
		t.Errorf("expected 'spec file not found' error, got: %v", err)
	}

	// Verify ledger was NOT updated — status should still be "in-progress"
	ledger2, err := engine.LoadLedger(tmpDir)
	if err != nil {
		t.Fatalf("loading ledger: %v", err)
	}
	specEntry := ledger2.GetSpecEntry("SPEC-123")
	if specEntry == nil {
		t.Fatal("SPEC-123 not found in ledger")
	}
	if specEntry.Status != "in-progress" {
		t.Errorf("expected status to remain 'in-progress' (disk write should not have happened), got: %s", specEntry.Status)
	}
	if len(specEntry.LinkedCommits) != 0 {
		t.Errorf("expected 0 linked commits (ledger save skipped on error), got %d", len(specEntry.LinkedCommits))
	}
}

func TestPromptForInitInstallsBinaryOnYes(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := filepath.Base(tmpDir)

	// Copy test binary as the local ff binary
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ffPath := filepath.Join(tmpDir, "ff")
	if err := copyFile(testBinary, ffPath); err != nil {
		t.Fatal(err)
	}

	// Create a go.mod so InitConfig produces a Go pipeline
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set HOME so binary installs into tmpDir/.local/bin/
	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/zsh")

	// Mock stdin with "y"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte("y\n"))
		w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	// Capture stderr to check for error messages
	var stderrBuf strings.Builder
	oldStderr := os.Stderr
	pr, pw, _ := os.Pipe()
	os.Stderr = pw
	done := make(chan struct{})
	go func() {
		defer close(done)
		var buf strings.Builder
		_, _ = io.Copy(&buf, pr)
		stderrBuf = buf
	}()

	// Run promptForInit - it should create config + install binary
	result := promptForInit(tmpDir)

	// Close stderr pipe and wait for reader
	pw.Close()
	<-done
	os.Stderr = oldStderr

	if !result {
		t.Fatalf("promptForInit returned false, expected true. stderr: %s", stderrBuf.String())
	}

	// Verify config file was created
	configPath := filepath.Join(tmpDir, projectName+"_ff.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config not created at %s", configPath)
	}

	// Verify binary was installed globally
	binPath := filepath.Join(tmpDir, ".local", "bin", "ff")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Errorf("ff binary not installed to %s", binPath)
	}

	// The PATH warning is expected in an isolated temp dir without shell profiles
	// Verify the warning mentions PATH (expected) not an actual install error
	stderrText := stderrBuf.String()
	if stderrText != "" && !strings.Contains(stderrText, "could not update PATH") {
		t.Errorf("unexpected stderr output: %s", stderrText)
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
