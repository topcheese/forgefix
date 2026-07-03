package main

import (
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
			flags := parseFlags(tc.args)

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
	printHelp(&buf)
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
	printVersion(&buf)
	output := buf.String()
	if !strings.Contains(output, engine.Version) {
		t.Errorf("expected version output to contain %q, got: %s", engine.Version, output)
	}
}

func TestPromptForSpecSelectionNewBug(t *testing.T) {
	tmpDir := t.TempDir()

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
	ledgerPath := filepath.Join(ledgerDir, "forgefix_ledger.json")
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
	if err := ledger.SaveToFile(ledgerPath); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	oldFindLedgerDir := findLedgerDir
	findLedgerDir = func(dir string) string {
		return tmpDir
	}
	defer func() { findLedgerDir = oldFindLedgerDir }()

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

	specID, err := promptForSpecSelection(tmpDir)
	<-done
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("promptForSpecSelection returned error: %v", err)
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
	ledgerPath := filepath.Join(ledgerDir, "forgefix_ledger.json")
	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := ledger.SaveToFile(ledgerPath); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	oldFindLedgerDir := findLedgerDir
	findLedgerDir = func(dir string) string {
		return tmpDir
	}
	defer func() { findLedgerDir = oldFindLedgerDir }()

	// Category: 4 (All), Spec: 1 (SPEC-123)
	input := "4\n1\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(input))
		w.Close()
	}()

	specID, err := promptForSpecSelection(tmpDir)
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("promptForSpecSelection returned error: %v", err)
	}
	if specID != "SPEC-123" {
		t.Errorf("expected SPEC-123, got: %s", specID)
	}
}

func TestPromptForSpecSelectionSkip(t *testing.T) {
	tmpDir := t.TempDir()

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
	ledgerPath := filepath.Join(ledgerDir, "forgefix_ledger.json")
	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := ledger.SaveToFile(ledgerPath); err != nil {
		t.Fatalf("saving ledger: %v", err)
	}

	oldFindLedgerDir := findLedgerDir
	findLedgerDir = func(dir string) string {
		return tmpDir
	}
	defer func() { findLedgerDir = oldFindLedgerDir }()

	// Category: Enter (skip)
	input := "\n"
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		w.Write([]byte(input))
		w.Close()
	}()

	specID, err := promptForSpecSelection(tmpDir)
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("promptForSpecSelection returned error: %v", err)
	}
	if specID != "" {
		t.Errorf("expected empty string for Enter selection, got: %s", specID)
	}
}

func TestRunCommitWithFlagSpecID(t *testing.T) {
	// Test that -s flag bypasses the interactive menu
	tmpDir := t.TempDir()

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
	ledgerPath := filepath.Join(ledgerDir, "forgefix_ledger.json")
	ledger := engine.NewLedgerEngine()
	entry := &engine.SpecEntry{
		SpecID:        "SPEC-123",
		RepoIssueID:   1,
		Status:        "in-progress",
		LinkedCommits: []string{},
		Type:          "feature",
	}
	ledger.SetSpecEntry("SPEC-123", entry)
	if err := ledger.SaveToFile(ledgerPath); err != nil {
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

	oldFindLedgerDir := findLedgerDir
	findLedgerDir = func(dir string) string {
		return tmpDir
	}
	defer func() { findLedgerDir = oldFindLedgerDir }()

	// Call runCommit with flagSpecID set - this should bypass promptForSpecSelection
	err = runCommit(tmpDir, "test commit message", "SPEC-123", "", "")
	if err != nil {
		t.Fatalf("runCommit with flagSpecID failed: %v", err)
	}

	// Verify commit was created with SPEC-123
	cmd = exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(output), "SPEC-123") {
		t.Errorf("expected commit to contain SPEC-123, got: %s", string(output))
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
