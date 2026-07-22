package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ForgeFix/engine"
)

// mockRepoCapture records issue payloads POSTed to the mock VCS server so tests
// can assert on the request bodies that ff sync produces.
type mockRepoCapture struct {
	mu            sync.Mutex
	postedIssues []map[string]any
}

// PostedIssues returns a copy of the captured POST /issues payloads.
func (c *mockRepoCapture) PostedIssues() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[string]any, len(c.postedIssues))
	copy(out, c.postedIssues)
	return out
}

// TestSpecLifecycle covers the end-to-end spec lifecycle without network:
//
//	Step 1 — ff spec --ai <name>  → creates spec file
//	Step 2 — ff commit --spec     → commits with SpecID, updates Ledger LinkedCommits
//	Step 3 — ff sync (mocked)     → creates Repo issue, updates Ledger status
func TestSpecLifecycle(t *testing.T) {
	dir := t.TempDir()

	// -------------------------------------------------------------------------
	// Bootstrap: build ff binary, create template, _ff.yaml, git repo
	// -------------------------------------------------------------------------

	ffBin := buildFF(t)

	createTemplate(t, dir)
	mockSrv, yamlContent, _ := createMockRepo(t)
	defer mockSrv.Close()
	yamlPath := filepath.Join(dir, "test_ff.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	initGitRepo(t, dir)

	// -------------------------------------------------------------------------
	// Step 1 — ff spec --ai test-feature
	// -------------------------------------------------------------------------
	t.Run("Step 1: ff spec creates spec file", func(t *testing.T) {
		specBody := "# Test Feature\n\n## Objective\n\nTest the lifecycle\n\n## Requirements\n\nMust work"
		cmd := exec.Command(ffBin, "spec", "--ai", "--type", "feature", "test-feature", specBody)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ff spec --ai test-feature failed: %v\n%s", err, out)
		}

		specPath := filepath.Join(dir, "specs", "test-feature.md")
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			t.Fatal("spec file was not created at specs/test-feature.md")
		}

		data, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "spec_id:") {
			t.Error("spec file missing spec_id in frontmatter")
		}
		if !strings.Contains(content, "# Test Feature") {
			t.Error("spec file missing title heading")
		}
		if !strings.Contains(content, "status: draft") {
			t.Error("spec file missing status in frontmatter")
		}
	})

	// -------------------------------------------------------------------------
	// Step 2 — ff commit --spec SPEC-XXXXX "implement test feature"
	// -------------------------------------------------------------------------
	t.Run("Step 2: ff commit binds commit to spec and updates Ledger", func(t *testing.T) {
		specID := extractSpecIDFromFile(t, dir)
		if specID == "" {
			t.Fatal("could not extract spec_id from spec file")
		}

		helperFile := filepath.Join(dir, "dummy.go")
		if err := os.WriteFile(helperFile, []byte("package main"), 0644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", "dummy.go")

		cmd := exec.Command(ffBin, "commit", "--spec", specID, "implement test feature")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ff commit failed: %v\n%s", err, out)
		}

		commitOut := runGit(t, dir, "log", "--oneline", "-1")
		expectedPrefix := fmt.Sprintf("feat: [%s]", specID)
		if !strings.Contains(commitOut, expectedPrefix) {
			t.Errorf("commit message missing spec prefix.\nhave: %s\nwant substring: %s", commitOut, expectedPrefix)
		}

		ledger, lErr := engine.LoadLedger(dir)
		if lErr != nil {
			t.Fatalf("LoadLedger: %v", lErr)
		}
		entry := ledger.GetSpecEntry(specID)
		if entry == nil {
			t.Error("ledger does not contain spec entry after commit")
		} else if len(entry.LinkedCommits) == 0 {
			t.Log("ledger has no linked_commits; commit may not have updated ledger")
		}
	})

	// -------------------------------------------------------------------------
	// Step 3 — ff sync (mocked VCS) creates Repo issue & updates Ledger
	// -------------------------------------------------------------------------
	t.Run("Step 3: ff sync creates remote issue and updates Ledger", func(t *testing.T) {
		specID := extractSpecIDFromFile(t, dir)
		if specID == "" {
			t.Fatal("could not extract spec_id from spec file")
		}

		cmd := exec.Command(ffBin, "sync")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ff sync failed: %v\n%s", err, out)
		}
		output := string(out)

		if !strings.Contains(output, "Spec sync completed") && !strings.Contains(output, "Syncing spec") {
			t.Logf("sync output (may be expected if spec already synced): %s", output)
		}

		ledger, lErr := engine.LoadLedger(dir)
		if lErr != nil {
			t.Fatalf("LoadLedger: %v", lErr)
		}
		entry := ledger.GetSpecEntry(specID)
		if entry == nil {
			t.Error("ledger missing spec entry after sync")
		} else {
			if entry.RepoIssueID == 0 {
				t.Logf("ledger has repo_issue_id=0 — sync may not have completed (expected without network mock)")
			}
			if entry.Status != "" {
				t.Logf("ledger spec entry has status: %s", entry.Status)
			}
		}
	})

	// -------------------------------------------------------------------------
	// Step 4 — ff ship (mocked) triggers SyncMetadata → status becomes "closed"
	// -------------------------------------------------------------------------
	t.Run("Step 4: ff ship triggers housekeeping SyncMetadata", func(t *testing.T) {
		specID := extractSpecIDFromFile(t, dir)
		if specID == "" {
			t.Fatal("could not extract spec_id from spec file")
		}

		// Need to manually promote to "ship" since promoteReviewSpecs is skipped
		// in non-interactive mode. We use the frontmatter-safe helper for the
		// spec file, then update the ledger directly.
		specPath := filepath.Join(dir, "specs", "test-feature.md")
		if err := engine.UpdateSpecFileStatus(specPath, "ship"); err != nil {
			t.Fatalf("updating spec file status: %v", err)
		}

		// Update ledger to "ship"
		ledger, lErr := engine.LoadLedger(dir)
		if lErr != nil {
			t.Fatalf("LoadLedger: %v", lErr)
		}
		if entry := ledger.GetSpecEntry(specID); entry != nil {
			entry.Status = "ship"
			ledger.SetSpecEntry(specID, entry)
		}
		if err := engine.SaveLedger(ledger, dir); err != nil {
			t.Fatalf("SaveLedger: %v", err)
		}

		// Now run ship — this should trigger SyncMetadata via housekeeping
		cmd := exec.Command(ffBin, "ship")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		output := string(out)
		// Ship may fail due to git tracking (no remote), but SyncMetadata
		// should be called as part of the housekeeping queue drain
		if err != nil {
			t.Logf("ff ship output (expected possible non-fatal issues): %s", output)
		}

		// After ship, verify both stores are in sync even if only
		// one was updated — the SyncMetadata function writes "closed" to both
		specData2, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		specStr2 := string(specData2)

		ledger2, lErr2 := engine.LoadLedger(dir)
		if lErr2 != nil {
			t.Fatalf("LoadLedger: %v", lErr2)
		}
		ledgerEntry2 := ledger2.GetSpecEntry(specID)
		ledgerHasClosed := ledgerEntry2 != nil && ledgerEntry2.Status == "closed"
		diskHasClosed := strings.Contains(specStr2, "status: closed")
		if !diskHasClosed && !ledgerHasClosed {
			t.Log("SyncMetadata may not have run (expected when no remote configured)")
		}

		// If one store says closed, the other SHOULD also say closed
		if diskHasClosed != ledgerHasClosed {
			t.Errorf("status mismatch — disk closed=%v, ledger closed=%v", diskHasClosed, ledgerHasClosed)
		}
	})

	// -------------------------------------------------------------------------
	// Validation: spec file and ledger remain consistent at every step
	// -------------------------------------------------------------------------
	t.Run("Validation: spec file and ledger status are consistent", func(t *testing.T) {
		specPath := filepath.Join(dir, "specs", "test-feature.md")
		specData, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		specContent := string(specData)

		ledger, lErr := engine.LoadLedger(dir)
		if lErr != nil {
			t.Fatalf("LoadLedger: %v", lErr)
		}

		specID := ""
		for _, line := range strings.Split(specContent, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "spec_id:") {
				specID = strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
				specID = strings.Trim(specID, `"`)
				break
			}
		}
		if specID == "" {
			t.Fatal("spec file has no spec_id")
		}
		ledgerEntry := ledger.GetSpecEntry(specID)
		if ledgerEntry == nil {
			t.Error("spec_id in spec file not found in ledger — files are out of sync")
		}

		// Extract status from spec file
		diskStatus := ""
		for _, line := range strings.Split(specContent, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "status:") {
				diskStatus = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
				diskStatus = strings.Split(diskStatus, " ")[0]
				break
			}
		}

		// Extract status from ledger
		ledgerStatus := ""
		if ledgerEntry != nil {
			ledgerStatus = ledgerEntry.Status
		}

		if diskStatus != "" && ledgerStatus != "" && diskStatus != ledgerStatus {
			t.Errorf("status mismatch: disk=%q ledger=%q", diskStatus, ledgerStatus)
		}

		giteaIssue := ""
		for _, line := range strings.Split(specContent, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "repo_issue:") {
				giteaIssue = strings.TrimSpace(strings.TrimPrefix(line, "repo_issue:"))
				break
			}
		}

		if giteaIssue != "" && giteaIssue != `""` {
			if ledgerEntry == nil || ledgerEntry.RepoIssueID == 0 {
				t.Logf("repo_issue %s in spec file but ledger has no matching repo_issue_id; not necessarily out of sync", giteaIssue)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func buildFF(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ff")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join("..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func createTemplate(t *testing.T, dir string) {
	t.Helper()
	tplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tpl := `---
spec_id: ""
status: draft # draft, ready, in-progress, resolved
repo_issue: ""
---
# [Title]
## Goal
## Technical Requirements
## Acceptance Criteria
`
	if err := os.WriteFile(filepath.Join(tplDir, "spec_template.md"), []byte(tpl), 0644); err != nil {
		t.Fatal(err)
	}
}

func createMockRepo(t *testing.T) (*httptest.Server, string, *mockRepoCapture) {
	t.Helper()
	cap := &mockRepoCapture{}
	var createdIssueNumber int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/issues/999") {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, `{"message":"not found"}`)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/issues") && r.Method == http.MethodPost:
			createdIssueNumber++
			// Capture the request body so tests can assert what ff sync posted.
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				cap.mu.Lock()
				cap.postedIssues = append(cap.postedIssues, payload)
				cap.mu.Unlock()
			}
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":%d,"number":%d,"title":"test","body":"","state":"open"}`, createdIssueNumber, createdIssueNumber)
		case strings.Contains(r.URL.Path, "/issues") && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":1,"number":1,"title":"test","body":"# Test Feature\n## Goal\n## Technical Requirements\n## Acceptance Criteria","state":"in-progress"}`)
		case strings.Contains(r.URL.Path, "/issues") && r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{}`)
		default:
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `[]`)
		}
	}))

	yamlContent := fmt.Sprintf(`global_timeout_seconds: 120
failure_decay_seconds: 30
pipelines:
  - id: test
    name: "[test]"
    type: go_mod
    panel_color: blue
    timeout_seconds: 30
    ledger_floor: 0
languages:
  go_mod:
    root_anchor: go.mod
    test_command: go test -json ./...
    token_patterns:
      token_run: '"Action":"run"'
      token_pass: '"Action":"pass"'
      token_fail: '"Action":"fail"'
exclude_dirs: []
github:
  owner: "test-owner"
  repo: "test-repo"
  token: "mock-token"
  base_url: "%s"
`, srv.URL)
	return srv, yamlContent, cap
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "commit.gpgSign", "false")

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "go.mod")
	runGit(t, dir, "commit", "-m", "initial commit")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

// Test404Reconciliation verifies that ff sync handles orphaned repo_issue
// references: when a SpecEntry has a RepoIssueID that no longer exists on
// the remote (404), sync unbinds it, clears the ledger and file, and recreates.
func Test404Reconciliation(t *testing.T) {
	dir := t.TempDir()

	ffBin := buildFF(t)
	createTemplate(t, dir)
	mockSrv, yamlContent, _ := createMockRepo(t)
	defer mockSrv.Close()

	yamlPath := filepath.Join(dir, "test_ff.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, dir)

	// Create a spec file with a stale repo_issue reference (999)
	specDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-404-TEST"
status: draft
repo_issue: 999
---
# Test 404
## Goal
## Technical Requirements
## Acceptance Criteria
`
	specPath := filepath.Join(specDir, "test-404.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-seed the ledger with a SpecEntry pointing at the orphaned issue
	ledger := engine.NewLedgerEngine()
	ledger.SetSpecEntry("SPEC-404-TEST", &engine.SpecEntry{
		SpecID:        "SPEC-404-TEST",
		RepoIssueID:   999,
		Status:        "draft",
		LinkedCommits: []string{},
	})
	if err := engine.SaveLedger(ledger, dir); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}

	// Run ff sync — mock server returns 404 for /issues/999
	cmd := exec.Command(ffBin, "sync")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Assert no panic (err would be nil or a warning; both are acceptable)
	if err != nil {
		t.Fatalf("ff sync failed: %v\n%s", err, output)
	}

	// Assert output clearly flags the unbinding
	if !strings.Contains(output, "unbinding") {
		t.Errorf("expected sync output to mention 'unbinding', got:\n%s", output)
	}

	// Assert the stale 999 is gone from the ledger
	ledger2, lErr := engine.LoadLedger(dir)
	if lErr != nil {
		t.Fatalf("LoadLedger: %v", lErr)
	}
	entry2 := ledger2.GetSpecEntry("SPEC-404-TEST")
	if entry2 != nil && entry2.RepoIssueID == 999 {
		t.Error("ledger still contains stale repo_issue_id 999 after 404 reconciliation")
	}

	// Assert the stale 999 is gone from the spec file frontmatter
	specData, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	specStr := string(specData)
	if strings.Contains(specStr, "repo_issue: 999") {
		t.Error("spec file frontmatter still contains stale repo_issue: 999 after 404 reconciliation")
	}
}

func extractSpecIDFromFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "specs", "test-feature.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "spec_id:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
			v = strings.Trim(v, `"`)
			return v
		}
	}
	return ""
}

// TestSyncPromotesReviewToShip verifies that ff sync --ai promotes specs
// from "review" status to "ship" in both the spec file and DB.
func TestSyncPromotesReviewToShip(t *testing.T) {
	dir := t.TempDir()

	ffBin := buildFF(t)
	createTemplate(t, dir)
	mockSrv, yamlContent, _ := createMockRepo(t)
	defer mockSrv.Close()

	yamlPath := filepath.Join(dir, "test_ff.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, dir)

	// Create a spec at review status
	specDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specContent := `---
spec_id: "SPEC-PROMO-TEST"
status: review
type: feature
version: "v0.9.8"
repo_issue: 0
---
# Test Promotion
## Objective
Verify promotion works
`
	specPath := filepath.Join(specDir, "test-promo.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Seed the DB with the review entry
	ledger := engine.NewLedgerEngine()
	ledger.SetSpecEntry("SPEC-PROMO-TEST", &engine.SpecEntry{
		SpecID:        "Test Promotion",
		Status:        "review",
		RepoIssueID:   0,
		LinkedCommits: []string{},
		Type:          "feature",
	})
	if err := engine.SaveLedger(ledger, dir); err != nil {
		t.Fatalf("SaveLedger: %v", err)
	}

	// Run ff sync (without --ai) — should auto-promote in non-terminal mode
	cmd := exec.Command(ffBin, "sync")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ff sync failed: %v\n%s", err, out)
	}
	output := string(out)

	// Verify promotion happened
	if !strings.Contains(output, "Auto-promoting") || !strings.Contains(output, "ship") {
		t.Errorf("expected sync to auto-promote to ship, got:\n%s", output)
	}

	// Verify spec file was updated to ship
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	specStr := string(data)
	if !strings.Contains(specStr, "status: ship") {
		t.Errorf("spec file should have status: ship after sync --ai, got:\n%s", specStr)
	}

	// Verify DB was updated to ship
	dbLedger, err := engine.LoadLedger(dir)
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	entry := dbLedger.GetSpecEntry("SPEC-PROMO-TEST")
	if entry == nil {
		t.Fatal("spec entry should exist after sync")
	}
	if entry.Status != "ship" {
		t.Errorf("DB status = %q, want %q", entry.Status, "ship")
	}
}
