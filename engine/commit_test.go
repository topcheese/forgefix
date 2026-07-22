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
	hash, specID, commitMsg, err := 	runCommit(tmpDir, "implement feature [SPEC-123]", "SPEC-123", "", "", false, d, "", false)
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
	_, _, commitMsg, err := 	runCommit(tmpDir, "[SPEC-456] add new feature", "SPEC-456", "", "", false, d, "", false)
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
	_, _, commitMsg, err := 	runCommit(tmpDir, "[SPEC-111] integrate with [SPEC-456] and [SPEC-789]", "SPEC-111", "", "", false, d, "", false)
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
	_, _, commitMsg, err := 	runCommit(tmpDir, "[SPEC-999]", "SPEC-999", "", "", false, d, "", false)
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
	_, _, commitMsg, err := 	runCommit(tmpDir, "implement feature", "SPEC-789", "", "", false, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if commitMsg != "feat: [SPEC-789] implement feature" {
		t.Errorf("expected clean format, got: %q", commitMsg)
	}
}

func TestRunCommit_BodyAppended(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-BODY-1"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test Spec
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-BODY-1.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	hash, specID, commitMsg, err := 	runCommit(tmpDir, "my message", "SPEC-BODY-1", "", "", false, d, "Additional body text", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty commit hash")
	}
	if specID != "SPEC-BODY-1" {
		t.Errorf("expected SPEC-BODY-1, got %s", specID)
	}
	// Commit message should contain body appended with \n\n separator (FR-4)
	expectedSubject := "feat: [SPEC-BODY-1] my message"
	if !strings.HasPrefix(commitMsg, expectedSubject) {
		t.Errorf("expected commitMsg to start with %q, got: %q", expectedSubject, commitMsg)
	}
	if !strings.Contains(commitMsg, "\n\nAdditional body text") {
		t.Errorf("expected body to be appended with \\n\\n separator in commitMsg, got: %q", commitMsg)
	}
	// Verify git log contains the full message body
	logOut := runGit(t, tmpDir, "log", "--format=%B", "-1")
	if !strings.Contains(logOut, "Additional body text") {
		t.Errorf("expected git log to contain body text, got:\n%s", logOut)
	}
	if !strings.Contains(logOut, expectedSubject) {
		t.Errorf("expected git log to contain subject, got:\n%s", logOut)
	}
}

func TestRunCommit_BodyEmptyDoesNotAffect(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-BODY-EMPTY"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test Spec
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-BODY-EMPTY.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, commitMsg, err := 	runCommit(tmpDir, "no body here", "SPEC-BODY-EMPTY", "", "", false, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	// Empty body string should produce a single-line commit message with no body
	if commitMsg != "feat: [SPEC-BODY-EMPTY] no body here" {
		t.Errorf("expected commit msg without body, got: %q", commitMsg)
	}
	// Verify git log also has single line
	logOut := runGit(t, tmpDir, "log", "--format=%B", "-1")
	logOut = strings.TrimSpace(logOut)
	if strings.Contains(logOut, "\n\n") {
		t.Errorf("expected no body in git log when body is empty, got:\n%s", logOut)
	}
}

func TestRunCommit_BodyWithNewlines(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-BODY-NL"
status: in-progress
type: feature
repo_issue: ""
created: 2024-01-01
---
# Test
`
	if err := os.WriteFile(filepath.Join(specDir, "SPEC-BODY-NL.md"), []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	multilineBody := "Line one\n\nLine two\nLine three"
	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, commitMsg, err := 	runCommit(tmpDir, "multiline body test", "SPEC-BODY-NL", "", "", false, d, multilineBody, false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}
	expected := "feat: [SPEC-BODY-NL] multiline body test\n\nLine one\n\nLine two\nLine three"
	if commitMsg != expected {
		t.Errorf("commitMsg mismatch.\n  got:  %q\n  want: %q", commitMsg, expected)
	}
	// Verify git log preserves the multiline body
	logOut := runGit(t, tmpDir, "log", "--format=%B", "-1")
	if !strings.Contains(logOut, "Line one") || !strings.Contains(logOut, "Line two") || !strings.Contains(logOut, "Line three") {
		t.Errorf("expected git log to contain all body lines, got:\n%s", logOut)
	}
}

func TestConsolidateLinkedCommits_DuplicateKeys(t *testing.T) {
	input := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: ["aaa1111"]
linked_commits: ["bbb2222"]
linked_commits: ["ccc3333"]
---
# Body
`
	got, err := consolidateLinkedCommits(input)
	if err != nil {
		t.Fatalf("consolidateLinkedCommits failed: %v", err)
	}

	lines := strings.Split(got, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
	if !strings.Contains(got, `"aaa1111"`) || !strings.Contains(got, `"bbb2222"`) || !strings.Contains(got, `"ccc3333"`) {
		t.Errorf("expected all hashes preserved in consolidated key:\n%s", got)
	}
}

func TestConsolidateLinkedCommits_NoDuplicateKeys(t *testing.T) {
	input := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: ["aaa1111"]
---
# Body
`
	got, err := consolidateLinkedCommits(input)
	if err != nil {
		t.Fatalf("consolidateLinkedCommits failed: %v", err)
	}
	if !strings.Contains(got, `linked_commits: ["aaa1111"]`) {
		t.Errorf("expected single linked_commits preserved:\n%s", got)
	}
}

func TestUpdateSpecFileLinkedCommits_NoExistingKey(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "SPEC-TEST.md")
	content := `---
spec_id: "SPEC-TEST"
status: draft
---
# Body
`
	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateSpecFileLinkedCommits(specFile, "abc1234"); err != nil {
		t.Fatalf("UpdateSpecFileLinkedCommits failed: %v", err)
	}

	data, _ := os.ReadFile(specFile)
	got := string(data)
	if !strings.Contains(got, `linked_commits: ["abc1234"]`) {
		t.Errorf("expected linked_commits to be created:\n%s", got)
	}
	count := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
}

func TestReplaceSpecFileLastLinkedCommit_ReplacesNotAppends(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "SPEC-TEST.md")
	content := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: ["pre-amend-hash"]
---
# Body
`
	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceSpecFileLastLinkedCommit(specFile, "final-hash"); err != nil {
		t.Fatalf("ReplaceSpecFileLastLinkedCommit failed: %v", err)
	}

	data, _ := os.ReadFile(specFile)
	got := string(data)
	if strings.Contains(got, "pre-amend-hash") {
		t.Errorf("pre-amend hash should have been replaced:\n%s", got)
	}
	if !strings.Contains(got, `"final-hash"`) {
		t.Errorf("expected final-hash in linked_commits:\n%s", got)
	}
	count := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
}

func TestConsolidateLinkedCommits_EmptyList(t *testing.T) {
	input := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: []
---
# Body
`
	got, err := consolidateLinkedCommits(input)
	if err != nil {
		t.Fatalf("consolidateLinkedCommits failed: %v", err)
	}
	count := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
}

func TestConsolidateLinkedCommits_MixedEmptyAndValues(t *testing.T) {
	input := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: []
linked_commits: ["aaa1111"]
---
# Body
`
	got, err := consolidateLinkedCommits(input)
	if err != nil {
		t.Fatalf("consolidateLinkedCommits failed: %v", err)
	}
	count := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
	if !strings.Contains(got, `"aaa1111"`) {
		t.Errorf("expected aaa1111 preserved in consolidated key:\n%s", got)
	}
}

func TestReplaceSpecFileLastLinkedCommit_NoExistingLinkedCommits(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "SPEC-TEST.md")
	content := `---
spec_id: "SPEC-TEST"
status: draft
---
# Body
`
	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ReplaceSpecFileLastLinkedCommit(specFile, "new-hash"); err != nil {
		t.Fatalf("ReplaceSpecFileLastLinkedCommit failed: %v", err)
	}

	data, _ := os.ReadFile(specFile)
	got := string(data)
	if !strings.Contains(got, `"new-hash"`) {
		t.Errorf("expected new-hash in linked_commits:\n%s", got)
	}
	count := 0
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "linked_commits:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 linked_commits key, got %d:\n%s", count, got)
	}
}

func TestFinalizeCommitAfterAmend_ReplacesPreAmendHash(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create initial commit so we have a base
	initialFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(initialFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "initial commit")

	// Create .ff directory with minimal ledger
	ffDir := filepath.Join(tmpDir, ".ff")
	if err := os.MkdirAll(ffDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a staged change to commit (simulating the user's code change)
	if err := os.WriteFile(initialFile, []byte("code change"), 0644); err != nil {
		t.Fatal(err)
	}

	// This creates the first commit — returns hash_A
	hash1, err := AutoStageAndCommit(tmpDir, "feat: [SPEC-TEST] test commit")
	if err != nil {
		t.Fatalf("AutoStageAndCommit failed: %v", err)
	}

	// Now simulate UpdateLedgerAfterCommit: write a spec file with hash_A
	// This modification is UNSTAGED (just like in the real flow)
	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-TEST.md")
	specContent := `---
spec_id: "SPEC-TEST"
status: draft
linked_commits: ["` + hash1 + `"]
---
# Test Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a ledger entry for the spec (required by finalizeCommitAfterAmend)
	ledger, lerr := LoadLedger(ffDir)
	if lerr != nil {
		t.Fatalf("LoadLedger failed: %v", lerr)
	}
	ledger.SetSpecEntry("SPEC-TEST", &SpecEntry{
		SpecID:        "SPEC-TEST",
		Status:        "draft",
		LinkedCommits: []string{hash1},
	})
	if serr := SaveLedger(ledger, ffDir); serr != nil {
		t.Fatalf("SaveLedger failed: %v", serr)
	}

	// Amend the commit — folds the spec file into it, creating hash_B
	if err := amendLastCommit(tmpDir); err != nil {
		t.Fatalf("amendLastCommit failed: %v", err)
	}

	// Get the final hash after amend
	git := NewGitHelper(tmpDir)
	hash2, err := git.CurrentHash()
	if err != nil {
		t.Fatalf("CurrentHash failed: %v", err)
	}

	// The hashes must differ (amend included the spec file change)
	if hash1 == hash2 {
		t.Fatal("expected different hashes after amend, but they are equal")
	}

	// The spec file still has hash_A — run finalizeCommitAfterAmend to fix it
	// configDir is the repo root (where specs/ lives), same as SpecConfigDir returns
	if err := finalizeCommitAfterAmend(tmpDir, "SPEC-TEST"); err != nil {
		t.Fatalf("finalizeCommitAfterAmend failed: %v", err)
	}

	// Verify the spec file now has hash_B
	data, _ := os.ReadFile(specFile)
	got := string(data)
	if strings.Contains(got, hash1) {
		t.Errorf("pre-amend hash %s should have been replaced, got:\n%s", hash1, got)
	}
	if !strings.Contains(got, hash2) {
		t.Errorf("expected final hash %s in linked_commits:\n%s", hash2, got)
	}
}

func TestRunCommit_NoPromoteReview_KeepsStatus(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-NOREVIEW.md")
	specContent := `---
spec_id: "SPEC-NOREVIEW"
status: draft
type: feature
repo_issue: ""
---
# No Review Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, _, err := runCommit(tmpDir, "implement feature [SPEC-NOREVIEW]", "SPEC-NOREVIEW", "", "", false, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}

	// Status should still be draft — no --review flag was passed
	data, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "status: draft") {
		t.Errorf("expected status to remain draft without --review, got:\n%s", got)
	}
}

func TestRunCommit_PromoteReview_SetsStatus(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-YESREVIEW.md")
	specContent := `---
spec_id: "SPEC-YESREVIEW"
status: draft
type: feature
repo_issue: ""
---
# Yes Review Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	_, _, _, err := runCommit(tmpDir, "implement feature [SPEC-YESREVIEW]", "SPEC-YESREVIEW", "", "", false, d, "", true)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}

	// Status should be review — --review flag was passed
	data, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "status: review") {
		t.Errorf("expected status to be review with --review, got:\n%s", got)
	}
}

func TestRunCommit_AmbiguousAutoDetect_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create two active specs modified within the same time window
	for _, id := range []string{"SPEC-AMB1", "SPEC-AMB2"} {
		specFile := filepath.Join(specDir, id+".md")
		specContent := `---
spec_id: "` + id + `"
status: draft
type: feature
repo_issue: ""
---
# ` + id + `
`
		if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Touch both files to set mod times close together
	for _, id := range []string{"SPEC-AMB1", "SPEC-AMB2"} {
		specFile := filepath.Join(specDir, id+".md")
		data, _ := os.ReadFile(specFile)
		// Append a space to force a mod time update
		if err := os.WriteFile(specFile, append(data, ' '), 0644); err != nil {
			t.Fatal(err)
		}
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// No --spec given, AI mode, ambiguous auto-detect should error
	_, _, _, err := runCommit(tmpDir, "implement feature", "", "", "", true, d, "", false)
	if err == nil {
		t.Fatal("expected error for ambiguous auto-detect, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

func TestRunCommit_HumanChore_SetsShip(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-CHORE1.md")
	specContent := `---
spec_id: "SPEC-CHORE1"
status: in-progress
type: chore
repo_issue: ""
---
# Chore Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// Human commit (aiMode=false) on chore spec → should set "ship"
	_, _, _, err := runCommit(tmpDir, "update deps [SPEC-CHORE1]", "SPEC-CHORE1", "", "", false, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}

	data, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "status: ship") {
		t.Errorf("expected status to be ship for human chore commit, got:\n%s", got)
	}
}

func TestRunCommit_AIChore_SetsReview(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-CHORE2.md")
	specContent := `---
spec_id: "SPEC-CHORE2"
status: draft
type: chore
repo_issue: ""
---
# Chore Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// AI commit (aiMode=true) on chore spec → should set "review"
	_, _, _, err := runCommit(tmpDir, "update deps [SPEC-CHORE2]", "SPEC-CHORE2", "", "", true, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}

	data, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "status: review") {
		t.Errorf("expected status to be review for AI chore commit, got:\n%s", got)
	}
}

func TestRunCommit_AIFeature_NoDowngradeFromInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specFile := filepath.Join(specDir, "SPEC-AIF1.md")
	specContent := `---
spec_id: "SPEC-AIF1"
status: in-progress
type: feature
repo_issue: ""
---
# Feature Spec
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	workFile := filepath.Join(tmpDir, "work.txt")
	if err := os.WriteFile(workFile, []byte("work"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	// AI commit on feature spec at in-progress — should upgrade to review (not downgrade to draft)
	_, _, _, err := runCommit(tmpDir, "implement feature [SPEC-AIF1]", "SPEC-AIF1", "", "", true, d, "", false)
	if err != nil {
		t.Fatalf("runCommit failed: %v", err)
	}

	data, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "status: review") {
		t.Errorf("expected status to be review after AI commit (upgrade from in-progress), got:\n%s", got)
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
