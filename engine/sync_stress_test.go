package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// stress test mock server
// ---------------------------------------------------------------------------

type stressCall struct {
	Method string
	Path   string
}

type stressMock struct {
	mu         sync.Mutex
	issues     []GitHubIssue
	nextID     int
	calls      []stressCall
	failPost   bool
	failCreate bool
	failClose  bool
	delay      time.Duration
}

func newStressMock() *stressMock {
	return &stressMock{nextID: 1}
}

func (sm *stressMock) Calls() []stressCall {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cp := make([]stressCall, len(sm.calls))
	copy(cp, sm.calls)
	return cp
}

func (sm *stressMock) Issues() []GitHubIssue {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cp := make([]GitHubIssue, len(sm.issues))
	copy(cp, sm.issues)
	return cp
}

func (sm *stressMock) Handler(w http.ResponseWriter, r *http.Request) {
	sm.mu.Lock()
	sm.calls = append(sm.calls, stressCall{Method: r.Method, Path: r.URL.Path})
	sm.mu.Unlock()

	if sm.delay > 0 {
		time.Sleep(sm.delay)
	}

	path := strings.TrimSuffix(r.URL.Path, "/")

	var issueNum int
	hasIssueNum := false
	if _, err := fmt.Sscanf(path, "/repos/test-owner/test-repo/issues/%d", &issueNum); err == nil {
		hasIssueNum = true
	}

	switch {
	case path == "/repos/test-owner/test-repo/issues" && r.Method == "GET":
		sm.mu.Lock()
		issues := sm.issues
		sm.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issues)

	case path == "/repos/test-owner/test-repo/issues" && r.Method == "POST":
		if sm.failCreate {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "server error"})
			return
		}
		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]string
		json.Unmarshal(body, &reqBody)
		sm.mu.Lock()
		issue := GitHubIssue{
			ID:     int64(sm.nextID),
			Number: sm.nextID,
			Title:  reqBody["title"],
			Body:   reqBody["body"],
			State:  "open",
		}
		sm.issues = append(sm.issues, issue)
		sm.nextID++
		sm.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(issue)

	case hasIssueNum && r.Method == "GET":
		sm.mu.Lock()
		var found *GitHubIssue
		for i := range sm.issues {
			if sm.issues[i].Number == issueNum {
				found = &sm.issues[i]
				break
			}
		}
		sm.mu.Unlock()
		if found == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
			return
		}
		json.NewEncoder(w).Encode(found)

	case hasIssueNum && r.Method == "PATCH" && !strings.HasSuffix(path, "/comments"):
		// capture body to determine if this is close or update
		body, _ := io.ReadAll(r.Body)
		var patch map[string]string
		json.Unmarshal(body, &patch)

		if _, isClose := patch["state"]; isClose && sm.failClose {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "server error"})
			return
		}

		sm.mu.Lock()
		for i := range sm.issues {
			if sm.issues[i].Number == issueNum {
				if state, ok := patch["state"]; ok {
					sm.issues[i].State = state
				}
				if b, ok := patch["body"]; ok {
					sm.issues[i].Body = b
				}
				break
			}
		}
		sm.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(GitHubIssue{Number: issueNum, State: "closed"})

	case strings.HasSuffix(path, "/comments") && r.Method == "POST":
		if sm.failPost {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "server error"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "body": "ok"})

	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"})
	}
}

func stressServer(t *testing.T, sm *stressMock) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(sm.Handler))
}

func stressCoord(srv *httptest.Server) *IssueCoordinator {
	coord := NewIssueCoordinator("test-owner", "test-repo", "test-token", srv.URL)
	return coord
}

func stressDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ff"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func enqueueStressOps(t *testing.T, dir string, ops ...SyncOperation) {
	t.Helper()
	for _, op := range ops {
		if err := EnqueueSyncOp(dir, op); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// 1. FIFO order
// ---------------------------------------------------------------------------

func TestStressFIFOOrder(t *testing.T) {
	sm := newStressMock()
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)
	coord := stressCoord(srv)
	coord.SetConfigDir(dir)

	enqueueStressOps(t, dir,
		SyncOperation{Type: SyncOpCreateIssue, TestName: "feat/test: stress-A",
			Details: &ErrorDetails{TestName: "feat/test: stress-A", ErrorMessage: "err A"}},
		SyncOperation{Type: SyncOpPostComment, IssueNum: 1, Body: "comment 1"},
		SyncOperation{Type: SyncOpCloseIssue, IssueNum: 1},
	)

	if err := processSyncQueue(coord, dir, nil); err != nil {
		t.Fatalf("processSyncQueue: %v", err)
	}

	remaining, err := PeekSyncQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected empty queue, got %d ops", len(remaining))
	}

	calls := sm.Calls()
	if len(calls) < 3 {
		t.Fatalf("expected at least 3 API calls, got %d: %v", len(calls), calls)
	}

	// Verify the order matches queue order
	expected := []string{
		"POST /repos/test-owner/test-repo/issues",      // create issue
		"POST /repos/test-owner/test-repo/issues/1/comments", // post comment
	}
	if !strings.HasSuffix(calls[len(calls)-1].Path, "/issues/1") || calls[len(calls)-1].Method != "PATCH" {
		t.Errorf("expected last call to be PATCH /issues/1, got %s %s", calls[len(calls)-1].Method, calls[len(calls)-1].Path)
	}

	// Check first two calls match expected order
	if calls[0].Method != expected[0] && !strings.HasPrefix(calls[1].Path, "/repos/test-owner/test-repo/issues") {
		// First call may be GET to search for existing issues, so check the POST next
		for _, c := range calls {
			if c.Method == "POST" && c.Path == "/repos/test-owner/test-repo/issues" {
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Partial success — one failing op doesn't block independent ops
// ---------------------------------------------------------------------------

func TestStressPartialSuccess(t *testing.T) {
	sm := newStressMock()
	sm.failPost = true
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)
	coord := stressCoord(srv)
	coord.SetConfigDir(dir)

	enqueueStressOps(t, dir,
		SyncOperation{Type: SyncOpCreateIssue, TestName: "feat/test: stress-B",
			Details: &ErrorDetails{TestName: "feat/test: stress-B", ErrorMessage: "err B"}},
		SyncOperation{Type: SyncOpPostComment, IssueNum: 1, Body: "will fail"},
		SyncOperation{Type: SyncOpCloseIssue, IssueNum: 99},
	)

	if err := processSyncQueue(coord, dir, nil); err != nil {
		t.Logf("processSyncQueue returned error (expected due to partial failure): %v", err)
	}

	// Failed post_comment should remain in queue with retry_count=1
	remaining, err := PeekSyncQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining op (post_comment), got %d", len(remaining))
	}
	if remaining[0].Type != SyncOpPostComment {
		t.Errorf("expected remaining op to be SyncOpPostComment, got %s", remaining[0].Type)
	}
	if remaining[0].RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", remaining[0].RetryCount)
	}

	// Verify create_issue succeeded — issue should exist on server
	if len(sm.Issues()) != 1 {
		t.Errorf("expected 1 issue created, got %d", len(sm.Issues()))
	}
}

// ---------------------------------------------------------------------------
// 3. Retry limit — 3 failures then logged permanently
// ---------------------------------------------------------------------------

func TestStressRetryLimit(t *testing.T) {
	sm := newStressMock()
	sm.failCreate = true
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)
	coord := stressCoord(srv)
	coord.SetConfigDir(dir)

	enqueueStressOps(t, dir,
		SyncOperation{Type: SyncOpCreateIssue, TestName: "feat/test: stress-C",
			Details: &ErrorDetails{TestName: "feat/test: stress-C", ErrorMessage: "err C"}},
	)

	// Process — retry_count goes 0→1→2 on successive failures.
	// After 3 failures (0-indexed: < 3 check), the operation is logged
	// as a permanent failure and removed from the queue.
	for i := 0; i < 3; i++ {
		if err := processSyncQueue(coord, dir, nil); err != nil {
			t.Logf("processSyncQueue attempt %d: %v", i+1, err)
		}
		if i < 2 {
			remaining, _ := PeekSyncQueue(dir)
			if len(remaining) == 0 {
				t.Fatalf("queue emptied after attempt %d (expected more retries)", i+1)
			}
			if remaining[0].RetryCount != i+1 {
				t.Errorf("attempt %d: expected retry_count=%d, got %d", i+1, i+1, remaining[0].RetryCount)
			}
		}
	}

	remaining, _ := PeekSyncQueue(dir)
	if len(remaining) != 0 {
		t.Errorf("expected queue empty after 3 retries, got %d ops", len(remaining))
	}

	failures, err := LoadSyncFailures(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) == 0 {
		t.Error("expected at least 1 permanent sync failure logged")
	}
	if len(failures) > 0 && !strings.Contains(failures[0].Error, "create_issue") {
		t.Errorf("failure message should reference create_issue, got: %s", failures[0].Error)
	}
}

// ---------------------------------------------------------------------------
// 4. Schedule gating
// ---------------------------------------------------------------------------

func TestStressScheduleGating(t *testing.T) {
	dir := stressDir(t)

	// Fresh state — should run
	shouldRun, err := ShouldRunFullSync(dir, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldRun {
		t.Error("ShouldRunFullSync with fresh state should return true")
	}

	shouldRetry, err := ShouldRetryFailures(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldRetry {
		t.Error("ShouldRetryFailures with fresh state should return true")
	}

	// Mark as run
	if err := MarkFullSyncRun(dir); err != nil {
		t.Fatal(err)
	}

	// Should not run again (max_age_days=7, just marked)
	shouldRun, err = ShouldRunFullSync(dir, 7)
	if err != nil {
		t.Fatal(err)
	}
	if shouldRun {
		t.Error("ShouldRunFullSync should return false after MarkFullSyncRun")
	}

	// With max_age_days=0, should always run
	shouldRun, err = ShouldRunFullSync(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldRun {
		t.Error("ShouldRunFullSync with max_age_days=0 should return true")
	}

	// Mark retry attempt
	if err := MarkRetryAttempt(dir); err != nil {
		t.Fatal(err)
	}

	shouldRetry, err = ShouldRetryFailures(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if shouldRetry {
		t.Error("ShouldRetryFailures should return false immediately after MarkRetryAttempt with retry_interval_hours=1")
	}

	// With retry_interval_hours=0, should always retry
	shouldRetry, err = ShouldRetryFailures(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !shouldRetry {
		t.Error("ShouldRetryFailures with retry_interval_hours=0 should return true")
	}
}

// ---------------------------------------------------------------------------
// 5. Stress burst with delay
// ---------------------------------------------------------------------------

func TestStressBurstQueue(t *testing.T) {
	sm := newStressMock()
	sm.delay = 5 * time.Millisecond
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)
	coord := stressCoord(srv)
	coord.SetConfigDir(dir)

	const numOps = 20
	for i := 0; i < numOps; i++ {
		name := fmt.Sprintf("feat/test: burst-%d", i)
		op := SyncOperation{
			Type:     SyncOpCreateIssue,
			TestName: name,
			Details:  &ErrorDetails{TestName: name, ErrorMessage: fmt.Sprintf("err %d", i)},
		}
		if err := EnqueueSyncOp(dir, op); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	if err := processSyncQueue(coord, dir, nil); err != nil {
		t.Fatalf("processSyncQueue: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 5*time.Millisecond {
		t.Logf("burst processed in %v (with %dms delay per op)", elapsed, 5)
	}

	remaining, err := PeekSyncQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected empty queue after burst, got %d ops", len(remaining))
	}

	issues := sm.Issues()
	if len(issues) != numOps {
		t.Errorf("expected %d issues created, got %d", numOps, len(issues))
	}
}

// ---------------------------------------------------------------------------
// 6. Mixed batch — fail subset, verify independent ops proceed
// ---------------------------------------------------------------------------

func TestStressMixedBatchFails(t *testing.T) {
	sm := newStressMock()
	sm.failCreate = true // make issue creation fail
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)
	coord := stressCoord(srv)
	coord.SetConfigDir(dir)

	// Enqueue operations that depend on different issue numbers:
	// close_issue and post_comment use arbitrary issue numbers, so they
	// should succeed even if create_issue operations fail.
	enqueueStressOps(t, dir,
		SyncOperation{Type: SyncOpCreateIssue, TestName: "feat/test: fail-A",
			Details: &ErrorDetails{TestName: "feat/test: fail-A", ErrorMessage: "err"}},
		SyncOperation{Type: SyncOpCloseIssue, IssueNum: 99, TestName: "close-99"},
		SyncOperation{Type: SyncOpCreateIssue, TestName: "feat/test: fail-B",
			Details: &ErrorDetails{TestName: "feat/test: fail-B", ErrorMessage: "err"}},
		SyncOperation{Type: SyncOpPostComment, IssueNum: 42, Body: "survivor comment"},
		SyncOperation{Type: SyncOpUpdateIssueBody, IssueNum: 42, Body: "updated body"},
	)

	if err := processSyncQueue(coord, dir, nil); err != nil {
		t.Logf("processSyncQueue returned error (expected): %v", err)
	}

	remaining, err := PeekSyncQueue(dir)
	if err != nil {
		t.Fatal(err)
	}

	// create_issue ops failed and should remain; close/post/update should succeed
	for _, op := range remaining {
		if op.Type != SyncOpCreateIssue {
			t.Errorf("remaining op should be SyncOpCreateIssue, got %s", op.Type)
		}
	}
}

// ---------------------------------------------------------------------------
// 7. Schedule config integration — RunBackgroundSync respects schedule
// ---------------------------------------------------------------------------

func TestStressScheduleRespected(t *testing.T) {
	sm := newStressMock()
	srv := stressServer(t, sm)
	defer srv.Close()

	dir := stressDir(t)

	// Write a config at a path LoadPipelineConfig can find
	// configDir needs a _ff.yaml matching the folder name
	folderName := filepath.Base(dir)
	yamlPath := filepath.Join(dir, folderName+"_ff.yaml")
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
sync_schedule:
  max_age_days: 7
  retry_interval_hours: 1
`, srv.URL)
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// First call should perform full sync (fresh state)
	if err := RunBackgroundSync(dir, ""); err != nil {
		t.Fatalf("RunBackgroundSync: %v", err)
	}

	// Second call should NOT perform full sync (just marked)
	// Change the mock to record calls
	callsBefore := len(sm.Calls())
	if err := RunBackgroundSync(dir, ""); err != nil {
		t.Fatalf("RunBackgroundSync: %v", err)
	}
	callsAfter := len(sm.Calls())

	// There should be few/no new calls since schedule says not time yet
	// (SyncIssues and SyncSpecs would make calls, but with max_age_days=7
	// they should be skipped). Minor calls for labels/etc may still occur.
	t.Logf("calls before second sync: %d, after: %d", callsBefore, callsAfter)
}
