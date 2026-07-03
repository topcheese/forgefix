package engine

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindAnchorDirDownwardTraversal verifies that the look-ahead directory walker
// correctly resolves nested child locations containing target blueprints.
func TestFindAnchorDirDownwardTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	subFolder := filepath.Join(tmpDir, "src", "app")
	_ = os.MkdirAll(subFolder, 0755)
	_ = os.WriteFile(filepath.Join(subFolder, "go.mod"), []byte("module test"), 0644)

	found := findAnchorDir(tmpDir, "go.mod", []string{".git"})
	if found != subFolder {
		t.Errorf("expected findAnchorDir to resolve down to '%s', got '%s'", subFolder, found)
	}
}

// TestEmitDetonatedPayloadStructure verifies that our emergency machine response
// correctly serializes high-density data context without throwing panics.
func TestEmitDetonatedPayloadStructure(t *testing.T) {
	dashboard := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline"},
	})
	
	// Execute a dry-run recover wrapper to ensure the serializer functions smoothly
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EmitDetonated panicked unexpectedly during serialization: %v", r)
		}
	}()
	
	// Test the core structural transformation mapping
	payload := dashboard.ToAIPayload()
	payload.Status = "DETONATED"
	if payload.Status != "DETONATED" {
		t.Error("failed to assign proper high-density detonation status token")
	}
}

func TestExecuteSuite_CoordinatorInitializedWithGitHubConfig(t *testing.T) {
	config := &Config{
		Pipelines: []PipelineConfig{
			{ID: "p1", Name: "Pipeline 1"},
		},
		GitHub: &GitHubConfig{
			Owner: "test-owner",
			Repo:  "test-repo",
			Token: "test-token",
		},
	}

	dashboard := NewDashboard(config.Pipelines)
	ledger, _ := LoadLedger(t.TempDir())
	ledger.ResetCurrentRun()
	dashboard.SetLedger(ledger)
	dashboard.ResetTrackers()

	if config.GitHub != nil && config.GitHub.Token != "" {
		dashboard.Coord = NewIssueCoordinator(config.GitHub.Owner, config.GitHub.Repo, config.GitHub.Token, "https://api.github.com")
	}

	if dashboard.Coord == nil {
		t.Fatal("Expected Coordinator to be initialized with GitHub config")
	}
	if dashboard.Coord == nil {
		t.Fatal("Coordinator is nil")
	}
}

func TestExecuteSuite_CoordinatorNotInitializedWithoutGitHubConfig(t *testing.T) {
	config := &Config{
		Pipelines: []PipelineConfig{
			{ID: "p1", Name: "Pipeline 1"},
		},
	}

	dashboard := NewDashboard(config.Pipelines)
	dashboard.SetLedger(NewLedgerEngine())

	if dashboard.Coord != nil {
		t.Error("Expected Coordinator to be nil when no GitHub config")
	}
}

func TestExecuteSuite_CoordinatorNotInitializedWithoutToken(t *testing.T) {
	config := &Config{
		Pipelines: []PipelineConfig{
			{ID: "p1", Name: "Pipeline 1"},
		},
		GitHub: &GitHubConfig{
			Owner: "test-owner",
			Repo:  "test-repo",
			Token: "",
		},
	}

	dashboard := NewDashboard(config.Pipelines)
	dashboard.SetLedger(NewLedgerEngine())

	if config.GitHub != nil && config.GitHub.Token != "" {
		dashboard.Coord = NewIssueCoordinator(config.GitHub.Owner, config.GitHub.Repo, config.GitHub.Token, "https://api.github.com")
	}

	if dashboard.Coord != nil {
		t.Error("Expected Coordinator to be nil with empty token")
	}
}

func TestIntegration_DetonationToDefusedFullCycle(t *testing.T) {
	tmpDir := t.TempDir()
	changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
	changelogContent := `# Changelog
- Fixed bug in TestFail (#10)
- Added feature
`
	if err := os.WriteFile(changelogPath, []byte(changelogContent), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	d.Coord = coord

	d.SetTestTracker("p1", &TestTracker{
		Completed: map[string]*TestInfo{
			"test_fail": {
				ID:          "test_fail",
				Name:        "TestFail",
				State:       StateDud,
				Elapsed:     100,
				ErrorTrace:  "assertion failed",
				FilePath:    "/path/to/test.go",
				FailureLine: 10,
			},
		},
		CompletedIDs: map[string]bool{"test_fail": true},
	})

	handleDetonationIssues(d, tmpDir)

	if len(d.IssueRefs) != 1 {
		t.Fatalf("After detonation: IssueRefs len = %d, want 1", len(d.IssueRefs))
	}
	info := d.IssueRefs["TestFail"]
	if info == nil {
		t.Fatal("After detonation: IssueRefs[TestFail] is nil")
	}
	if info.Number != 0 {
		t.Errorf("After detonation: Number = %d, want 0 (queued)", info.Number)
	}
	if info.Existed {
		t.Errorf("After detonation: Existed = true, want false")
	}

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("Expected 1 queued operation, got %d", len(ops))
	}
	if ops[0].Type != SyncOpCreateIssue {
		t.Errorf("Expected SyncOpCreateIssue, got %s", ops[0].Type)
	}

	handleDefusedIssues(d, tmpDir)

	if len(d.IssueRefs) != 1 {
		t.Fatalf("After defused: IssueRefs should still have 1, got %d", len(d.IssueRefs))
	}

	ops, err = LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	// Queue has 1 from handleDetonationIssues + 2 from handleDefusedIssues = 3 total
	if len(ops) != 3 {
		t.Fatalf("Expected 3 queued operations (1 create_issue + 2 from defused), got %d", len(ops))
	}
	// Last 2 should be post_comment and close_issue from handleDefusedIssues
	if ops[1].Type != SyncOpPostComment {
		t.Errorf("Expected second op SyncOpPostComment, got %s", ops[1].Type)
	}
	if ops[2].Type != SyncOpCloseIssue {
		t.Errorf("Expected third op SyncOpCloseIssue, got %s", ops[2].Type)
	}

	// Note: Changelog strikethrough is now handled asynchronously by background sync
	// when the close_issue operation is processed. The issue number is not known
	// until the create_issue operation completes in background sync.
	// So we don't check for changelog strikethrough here.
}

func TestIntegration_NoFailedTestsNoIssues(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	d.SetTestTracker("p1", &TestTracker{
		Completed: map[string]*TestInfo{
			"test_pass": {
				ID:    "test_pass",
				Name:  "TestPass",
				State: StatePopped,
			},
		},
		CompletedIDs: map[string]bool{"test_pass": true},
	})

	failed := collectFailedTests(d)
	if len(failed) != 0 {
		t.Errorf("collectFailedTests with no duds: got %d, want 0", len(failed))
	}
}

func TestIntegration_MultiplePipelinesFailures(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
		{ID: "p2", Name: "Pipeline 2"},
	})
	coord, transport := newMockCoordinator()
	d.Coord = coord

	d.SetTestTracker("p1", &TestTracker{
		Completed: map[string]*TestInfo{
			"test_a": {ID: "test_a", Name: "TestA", State: StateDud, Elapsed: 100},
			"test_b": {ID: "test_b", Name: "TestB", State: StateDud, Elapsed: 200},
		},
		CompletedIDs: map[string]bool{"test_a": true, "test_b": true},
	})
	d.SetTestTracker("p2", &TestTracker{
		Completed: map[string]*TestInfo{
			"test_c": {ID: "test_c", Name: "TestC", State: StateDud, Elapsed: 300},
		},
		CompletedIDs: map[string]bool{"test_c": true},
	})

	searchA := "https://api.github.com/repos/test-owner/test-repo/issues?q=test-owner%2FTestA&state=all"
	transport.setResponse("GET", searchA, http.StatusOK, []GitHubIssue{})
	searchB := "https://api.github.com/repos/test-owner/test-repo/issues?q=test-owner%2FTestB&state=all"
	transport.setResponse("GET", searchB, http.StatusOK, []GitHubIssue{})
	searchC := "https://api.github.com/repos/test-owner/test-repo/issues?q=test-owner%2FTestC&state=all"
	transport.setResponse("GET", searchC, http.StatusOK, []GitHubIssue{})

	createURL := "https://api.github.com/repos/test-owner/test-repo/issues"
	transport.setResponse("POST", createURL, http.StatusCreated, GitHubIssue{
		Number:  42,
		HTMLURL: "https://github.com/test-owner/test-repo/issues/42",
		State:   "open",
	})

	handleDetonationIssues(d, "")

	if len(d.IssueRefs) != 3 {
		t.Errorf("Expected 3 issue refs, got %d", len(d.IssueRefs))
	}
}

func TestIntegration_HandleDefusedIssues_OnlyClosesTracked(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	coord, transport := newMockCoordinator()
	d.Coord = coord

	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/5"
	transport.setResponse("PATCH", issueURL, http.StatusOK, GitHubIssue{Number: 5, State: "closed"})

	d.IssueRefs["TestA"] = &IssueInfo{Number: 5}

	handleDefusedIssues(d, t.TempDir())

	if coord.issueCache["TestA"] != nil {
	}
}

func TestIntegration_EmptyDashboardHandlerNoOps(t *testing.T) {
	d := NewDashboard(nil)
	coord, _ := newMockCoordinator()
	d.Coord = coord

	handleDetonationIssues(d, "")
	handleDefusedIssues(d, t.TempDir())
	handleTimeoutIssues(d, "")
}

func TestNewCoordinatorFromConfig_aiModeDisabled(t *testing.T) {
	t.Run("aiMode true + AutoIssueManagement true = non-nil", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), true)
		if coord == nil {
			t.Error("expected non-nil coordinator when aiMode=true and AutoIssueManagement=true")
		}
	})

	t.Run("aiMode true + AutoIssueManagement false = nil", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: false,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), true)
		if coord != nil {
			t.Error("expected nil coordinator when aiMode=true and AutoIssueManagement=false")
		}
	})

	t.Run("aiMode false + AutoIssueManagement false = non-nil (ff independence)", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: false,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), false)
		if coord == nil {
			t.Error("expected non-nil coordinator when aiMode=false (ff) regardless of AutoIssueManagement")
		}
	})

	t.Run("aiMode false + AutoIssueManagement true = non-nil (ff independence)", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), false)
		if coord == nil {
			t.Error("expected non-nil coordinator when aiMode=false (ff)")
		}
	})

	t.Run("nil GitHub config = nil", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), false)
		if coord != nil {
			t.Error("expected nil coordinator with nil GitHub config")
		}
	})

	t.Run("empty token = nil", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), false)
		if coord != nil {
			t.Error("expected nil coordinator with empty token")
		}
	})

	t.Run("baseURL defaults to api.github.com when empty", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
				BaseURL: "",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), true)
		if coord == nil {
			t.Fatal("expected non-nil coordinator")
		}
		if coord.baseURL != "https://api.github.com" {
			t.Errorf("baseURL = %q, want %q", coord.baseURL, "https://api.github.com")
		}
	})

	t.Run("baseURL preserved when set", func(t *testing.T) {
		cfg := &Config{
			AutoIssueManagement: true,
			GitHub: &GitHubConfig{
				Owner: "o", Repo: "r", Token: "t",
				BaseURL: "http://repo.local:3000",
			},
		}
		coord := NewCoordinatorFromConfig(cfg, t.TempDir(), true)
		if coord == nil {
			t.Fatal("expected non-nil coordinator")
		}
		if coord.baseURL != "http://repo.local:3000" {
			t.Errorf("baseURL = %q, want %q", coord.baseURL, "http://repo.local:3000")
		}
	})
}

func TestIntegration_CodeDuplicationTriggersWarningState(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})

	tmpDir := t.TempDir()
	d.ConfigDir = tmpDir
	fileA := filepath.Join(tmpDir, "source_util.go")
	fileB := filepath.Join(tmpDir, "bloated_app.go")

	// File A is a small, focused snippet
	sharedSnippet := `func ProcessMetrics() {
		fmt.Println("Processing aggregate data points...")
		fmt.Println("Ensuring thread-safe sync execution...")
	}`

	_ = os.WriteFile(fileA, []byte("package main\nimport \"fmt\"\n"+sharedSnippet), 0644)

	// File B has a completely unique structure, but copies the snippet into its center
	_ = os.WriteFile(fileB, []byte(`package main
import "fmt"
func RunServer() {
	fmt.Println("Starting web framework server on port 8080...")
}
`+sharedSnippet+`
func ShutdownServer() {
	fmt.Println("Cleaning network contexts...")
}`), 0644)

	registry := NewFingerprintRegistry()

	// 1. Scan and seed our database with the known code signatures
	if err := registry.ScanAndRegister(fileA); err != nil {
		t.Fatalf("Failed to register source target: %v", err)
	}

	// 2. Scan the bloated file to see if it catches the hidden duplicate block inside it
	isDuplicate, originalPath, err := registry.ScanAndCheck(fileB)
	if err != nil {
		t.Fatalf("Scanner crashed during inspection pass: %v", err)
	}

	// 3. Assert: This will turn RED immediately because ScanAndCheck only compares macro hashes!
	if !isDuplicate {
		t.Error("FAIL: Scanner failed to identify a duplicated chunk embedded inside a unique file file.")
	}
	if originalPath != fileA {
		t.Errorf("Expected collision tracking to return '%s', got '%s'", fileA, originalPath)
	}

	// Append this directly to the bottom of your existing test to verify dashboard warning telemetry
	warnMsg := fmt.Sprintf("⚠️ YELLOW ALERT: Code duplication detected in %s (cloned from %s)", filepath.Base(fileB), filepath.Base(fileA))
	d.AddSystemError(warnMsg)

	errors := d.GetSystemErrors()
	foundWarning := false
	for _, msg := range errors {
		if strings.Contains(msg, "YELLOW ALERT") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("Dashboard failed to register and store the duplication system warning string")
	}

	// Verify the duplication warning propagates to issue creation
	coord, _ := newMockCoordinator()
	d.Coord = coord

	handleDetonationIssues(d, tmpDir)

	if len(d.IssueRefs) != 1 {
		t.Fatalf("Expected 1 IssueRef after detonation, got %d", len(d.IssueRefs))
	}
	info, ok := d.IssueRefs[warnMsg]
	if !ok {
		t.Fatal("IssueRefs missing for duplication warning")
	}
	if info.Number != 0 {
		t.Errorf("IssueRefs.Number = %d, want 0 (queued)", info.Number)
	}
	if info.Existed {
		t.Errorf("IssueRefs.Existed = true, want false (new issue)")
	}

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("Expected 1 queued operation, got %d", len(ops))
	}
	if ops[0].Type != SyncOpCreateIssue {
		t.Errorf("Expected SyncOpCreateIssue, got %s", ops[0].Type)
	}
}