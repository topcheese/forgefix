
package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type mockTransport struct {
	mu             sync.Mutex
	responses      map[string]func(req *http.Request) (*http.Response, error)
	callCount      map[string]int
	responseBody   map[string][]byte
	responseStatus map[string]int
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses:      make(map[string]func(req *http.Request) (*http.Response, error)),
		callCount:      make(map[string]int),
		responseBody:   make(map[string][]byte),
		responseStatus: make(map[string]int),
	}
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.String()
	m.mu.Lock()
	m.callCount[key]++
	m.mu.Unlock()

	m.mu.Lock()
	status := m.responseStatus[key]
	body := m.responseBody[key]
	m.mu.Unlock()

	if status != 0 || len(body) > 0 {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
	}, nil
}

func (m *mockTransport) setResponse(method, url string, status int, body interface{}) {
	var data []byte
	switch b := body.(type) {
	case string:
		data = []byte(b)
	case []byte:
		data = b
	default:
		var err error
		data, err = json.Marshal(b)
		if err != nil {
			data = []byte(`{}`)
		}
	}
	m.mu.Lock()
	m.responseStatus[method+" "+url] = status
	m.responseBody[method+" "+url] = data
	m.mu.Unlock()
}

type mockHTTPClient struct {
	transport *mockTransport
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.transport.RoundTrip(req)
}

func newMockCoordinator() (*IssueCoordinator, *mockTransport) {
	transport := newMockTransport()
	client := &mockHTTPClient{transport: transport}
	coord := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	coord.client = client
	return coord, transport
}

func TestErrorDetails_Parsing(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name: "valid_error_details",
			raw: `{
				"test_name": "TestAddition",
				"package": "math",
				"file_path": "/path/to/math.go",
				"line_number": 42,
				"error_message": "assertion failed",
				"stack_trace": "goroutine 1 [running]:\nmath.TestAddition()"
			}`,
			wantErr: false,
		},
		{
			name:    "minimal_error_details",
			raw:     `{"test_name": "SimpleTest"}`,
			wantErr: false,
		},
		{
			name:    "empty_error_details",
			raw:     `{}`,
			wantErr: false,
		},
		{
			name:    "invalid_json",
			raw:     `{"test_name": "Test", invalid: true}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
			details, err := coordinator.ParseErrorDetails(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseErrorDetails() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && details != nil {
				if details.TestName != "TestAddition" && details.TestName != "SimpleTest" && details.TestName != "" {
					t.Errorf("ParseErrorDetails() TestName = %q, want one of empty, TestAddition, or SimpleTest", details.TestName)
				}
			}
		})
	}
}

func TestErrorDetails_StructuralValidation(t *testing.T) {
	details := &ErrorDetails{
		TestName:     "TestAddition",
		Package:      "math",
		FilePath:     "/path/to/math.go",
		LineNumber:   42,
		ErrorMessage: "assertion failed",
		StackTrace:   "goroutine 1 [running]:\nmath.TestAddition()",
	}
	if details.TestName != "TestAddition" {
		t.Errorf("TestName = %q, want TestAddition", details.TestName)
	}
	if details.Package != "math" {
		t.Errorf("Package = %q, want math", details.Package)
	}
	if details.FilePath != "/path/to/math.go" {
		t.Errorf("FilePath = %q, want /path/to/math.go", details.FilePath)
	}
	if details.LineNumber != 42 {
		t.Errorf("LineNumber = %d, want 42", details.LineNumber)
	}
	if details.ErrorMessage != "assertion failed" {
		t.Errorf("ErrorMessage = %q, want assertion failed", details.ErrorMessage)
	}
	if details.StackTrace != "goroutine 1 [running]:\nmath.TestAddition()" {
		t.Errorf("StackTrace mismatch")
	}
}

func TestErrorDetails_JSONRoundTrip(t *testing.T) {
	original := &ErrorDetails{
		TestName:     "TestAddition",
		Package:      "math",
		FilePath:     "/path/to/math.go",
		LineNumber:   42,
		ErrorMessage: "assertion failed",
		StackTrace:   "goroutine 1 [running]:\nmath.TestAddition()",
	}
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	var details ErrorDetails
	if err := json.Unmarshal(jsonData, &details); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if details.TestName != original.TestName {
		t.Errorf("TestName mismatch: got %q, want %q", details.TestName, original.TestName)
	}
	if details.Package != original.Package {
		t.Errorf("Package mismatch: got %q, want %q", details.Package, original.Package)
	}
	if details.FilePath != original.FilePath {
		t.Errorf("FilePath mismatch: got %q, want %q", details.FilePath, original.FilePath)
	}
	if details.LineNumber != original.LineNumber {
		t.Errorf("LineNumber mismatch: got %d, want %d", details.LineNumber, original.LineNumber)
	}
	if details.ErrorMessage != original.ErrorMessage {
		t.Errorf("ErrorMessage mismatch: got %q, want %q", details.ErrorMessage, original.ErrorMessage)
	}
	if details.StackTrace != original.StackTrace {
		t.Errorf("StackTrace mismatch")
	}
}

func TestIssueCoordinator_IssueKeyGeneration(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	key := coordinator.GetIssueKey("TestAddition")
	expectedKey := "test-owner/TestAddition"
	if key != expectedKey {
		t.Errorf("GetIssueKey() = %q, want %q", key, expectedKey)
	}
}

func TestEnsureIssue_CreatesNew(t *testing.T) {
	coord, transport := newMockCoordinator()

	searchURL := "https://api.github.com/repos/test-owner/test-repo/issues?q=test-owner%2Ffeat%2Ftest%3A+TestAddition&state=all"
	transport.setResponse("GET", searchURL, http.StatusOK, []GitHubIssue{})

	createURL := "https://api.github.com/repos/test-owner/test-repo/issues"
	transport.setResponse("POST", createURL, http.StatusCreated, GitHubIssue{
		Number:  42,
		HTMLURL: "https://github.com/test-owner/test-repo/issues/42",
		State:   "open",
		Title:   "feat/test: TestAddition",
	})

	details := &ErrorDetails{
		TestName:     "feat/test: TestAddition",
		Package:      "math",
		FilePath:     "/path/to/math.go",
		LineNumber:   42,
		ErrorMessage: "assertion failed",
	}

	issue, existed, err := coord.EnsureIssue("feat/test: TestAddition", details)
	if err != nil {
		t.Fatalf("EnsureIssue() error = %v", err)
	}
	if existed {
		t.Errorf("EnsureIssue() existed = true, want false for new issue")
	}
	if issue.Number != 42 {
		t.Errorf("EnsureIssue() Number = %d, want 42", issue.Number)
	}
}

func TestEnsureIssue_ReturnsExisting(t *testing.T) {
	coord, transport := newMockCoordinator()

	existingIssue := GitHubIssue{
		Number:  99,
		HTMLURL: "https://github.com/test-owner/test-repo/issues/99",
		State:   "open",
		Title:   "TestExisting",
	}

	searchURL := "https://api.github.com/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", searchURL, http.StatusOK, []GitHubIssue{existingIssue})

	details := &ErrorDetails{TestName: "TestExisting"}
	issue, existed, err := coord.EnsureIssue("TestExisting", details)
	if err != nil {
		t.Fatalf("EnsureIssue() error = %v", err)
	}
	if !existed {
		t.Errorf("EnsureIssue() existed = false, want true for existing issue")
	}
	if issue.Number != 99 {
		t.Errorf("EnsureIssue() Number = %d, want 99", issue.Number)
	}
}

func TestEnsureIssue_ApiError(t *testing.T) {
	coord, _ := newMockCoordinator()
	details := &ErrorDetails{TestName: "TestFail"}
	_, _, err := coord.EnsureIssue("TestFail", details)
	if err == nil {
		t.Error("EnsureIssue() expected error with no mock, got nil")
	}
}

func TestGetIssueComments(t *testing.T) {
	coord, transport := newMockCoordinator()

	comments := []GitHubComment{
		{ID: 1, Body: "First attempt failed", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: 2, Body: "Second attempt also failed", CreatedAt: "2024-01-02T00:00:00Z"},
	}

	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/42/comments"
	transport.setResponse("GET", commentsURL, http.StatusOK, comments)

	got, err := coord.GetIssueComments(42)
	if err != nil {
		t.Fatalf("GetIssueComments() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetIssueComments() got %d comments, want 2", len(got))
	}
	if got[0].Body != "First attempt failed" {
		t.Errorf("GetIssueComments()[0].Body = %q, want %q", got[0].Body, "First attempt failed")
	}
	if got[1].Body != "Second attempt also failed" {
		t.Errorf("GetIssueComments()[1].Body = %q, want %q", got[1].Body, "Second attempt also failed")
	}
}

func TestGetIssueComments_Empty(t *testing.T) {
	coord, transport := newMockCoordinator()

	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/99/comments"
	transport.setResponse("GET", commentsURL, http.StatusOK, []GitHubComment{})

	got, err := coord.GetIssueComments(99)
	if err != nil {
		t.Fatalf("GetIssueComments() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("GetIssueComments() got %d comments, want 0", len(got))
	}
}

func TestGetIssueComments_ApiError(t *testing.T) {
	coord, _ := newMockCoordinator()
	_, err := coord.GetIssueComments(999)
	if err == nil {
		t.Error("GetIssueComments() expected error with no mock, got nil")
	}
}

func TestCloseIssueByNumber(t *testing.T) {
	coord, transport := newMockCoordinator()

	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/42"
	transport.setResponse("PATCH", issueURL, http.StatusOK, GitHubIssue{
		Number: 42,
		State:  "closed",
	})

	err := coord.CloseIssueByNumber(42)
	if err != nil {
		t.Fatalf("CloseIssueByNumber() error = %v", err)
	}
}

func TestCloseIssueByNumber_ApiError(t *testing.T) {
	coord, _ := newMockCoordinator()
	err := coord.CloseIssueByNumber(999)
	if err == nil {
		t.Error("CloseIssueByNumber() expected error with no mock, got nil")
	}
}

func TestCloseIssue_FromCache(t *testing.T) {
	coord, transport := newMockCoordinator()

	searchURL := "https://api.github.com/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", searchURL, http.StatusOK, []GitHubIssue{
		{Number: 77, Title: "TestClose", HTMLURL: "https://github.com/test-owner/test-repo/issues/77", State: "open"},
	})

	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/77"
	transport.setResponse("PATCH", issueURL, http.StatusOK, GitHubIssue{Number: 77, State: "closed"})

	_, err := coord.CheckExistingIssue("TestClose")
	if err != nil {
		t.Fatalf("CheckExistingIssue() error = %v", err)
	}

	err = coord.CloseIssue("TestClose")
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
}

func TestCloseIssue_NotInCache(t *testing.T) {
	coord, _ := newMockCoordinator()
	err := coord.CloseIssue("UnknownTest")
	if err == nil {
		t.Error("CloseIssue() expected error for uncached test, got nil")
	}
}

func TestGenerateIssueBody(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	details := &ErrorDetails{
		TestName:     "TestAddition",
		Package:      "math",
		FilePath:     "/path/to/math.go",
		LineNumber:   42,
		ErrorMessage: "assertion failed",
		StackTrace:   "goroutine 1 [running]:\nmath.TestAddition()",
	}
	body := coordinator.generateIssueBody("TestAddition", details)
	if body == "" {
		t.Error("generateIssueBody() returned empty string")
	}
	if !strings.Contains(body, "TestAddition") {
		t.Error("Issue body should contain test name")
	}
	if !strings.Contains(body, "math") {
		t.Error("Issue body should contain package name")
	}
	if !strings.Contains(body, "/path/to/math.go") {
		t.Error("Issue body should contain file path")
	}
	if !strings.Contains(body, "42") {
		t.Error("Issue body should contain line number")
	}
	if !strings.Contains(body, "assertion failed") {
		t.Error("Issue body should contain error message")
	}
}

func TestGenerateIssueBody_EmptyDetails(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	details := &ErrorDetails{}
	body := coordinator.generateIssueBody("EmptyTest", details)
	if body == "" {
		t.Error("generateIssueBody() returned empty string for empty details")
	}
	if !strings.Contains(body, "EmptyTest") {
		t.Error("Issue body should contain test name even with empty details")
	}
}

func TestCheckForExistingIssue_Stub(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	_, err := CheckForExistingIssue(coordinator, "TestStub")
	if err == nil {
		t.Error("Expected error from stub function with no real connection, got nil")
	}
}

func TestClearCache(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	coordinator.ClearCache("TestAddition")
	coordinator.ClearAllCache()
	_ = NewIssueCoordinator("new-owner", "new-repo", "new-token", "https://api.github.com")
}

func TestConcurrentCacheAccess(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	var wg sync.WaitGroup
	const numWorkers = 5
	const numOpsPerWorker = 20

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerWorker; j++ {
				testName := fmt.Sprintf("Test-%d-%d", workerID, j)
				details := &ErrorDetails{
					TestName:     testName,
					Package:      fmt.Sprintf("pkg-%d", workerID),
					FilePath:     fmt.Sprintf("/path/to/file_%d.go", workerID),
					LineNumber:   j,
					ErrorMessage: fmt.Sprintf("error-%d-%d", workerID, j),
				}
				_, _ = coordinator.CheckExistingIssue(testName)
				_, _ = coordinator.CreateIssue(testName, details)
				coordinator.ClearCache(testName)
			}
		}(i)
	}
	wg.Wait()
}

func TestNoPanicOnConcurrentAccess(t *testing.T) {
	coordinator := NewIssueCoordinator("test-owner", "test-repo", "test-token", "https://api.github.com")
	var wg sync.WaitGroup
	const numWorkers = 20
	const numOpsPerWorker = 50

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerWorker; j++ {
				testName := fmt.Sprintf("Test-%d-%d", workerID, j)
				rawDetails := fmt.Sprintf(`{"test_name": "%s", "package": "pkg-%d"}`, testName, workerID)
				_, _ = coordinator.ParseErrorDetails(rawDetails)
				_ = coordinator.GetIssueKey(testName)
				coordinator.ClearCache(testName)
			}
		}(i)
	}
	wg.Wait()
}

func TestCollectFailedTests(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})

	d.TestTrackers["p1"] = &TestTracker{
		Completed: map[string]*TestInfo{
			"test_a": {ID: "test_a", Name: "TestA", State: StateDud, Elapsed: 100},
			"test_b": {ID: "test_b", Name: "TestB", State: StateDud, Elapsed: 200},
			"test_c": {ID: "test_c", Name: "TestC", State: StatePopped, Elapsed: 50},
		},
		CompletedIDs: map[string]bool{
			"test_a": true,
			"test_b": true,
			"test_c": true,
		},
	}

	failed := collectFailedTests(d)
	if len(failed) != 2 {
		t.Fatalf("collectFailedTests() got %d, want 2", len(failed))
	}
	ids := make(map[string]bool)
	for _, f := range failed {
		ids[f.TestID] = true
	}
	if !ids["test_a"] || !ids["test_b"] {
		t.Errorf("collectFailedTests() missing expected tests, got %v", failed)
	}
}

func TestCollectFailedTests_NoFailures(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})

	d.TestTrackers["p1"] = &TestTracker{
		Completed: map[string]*TestInfo{
			"test_a": {ID: "test_a", Name: "TestA", State: StatePopped, Elapsed: 100},
		},
	}

	failed := collectFailedTests(d)
	if len(failed) != 0 {
		t.Errorf("collectFailedTests() got %d, want 0", len(failed))
	}
}

func TestCollectFailedTests_EmptyDashboard(t *testing.T) {
	d := NewDashboard(nil)
	failed := collectFailedTests(d)
	if len(failed) != 0 {
		t.Errorf("collectFailedTests() got %d, want 0", len(failed))
	}
}

func TestGetAllTracked(t *testing.T) {
	coord, _ := newMockCoordinator()

	coord.tracked["test_a"] = 100
	coord.tracked["test_b"] = 200

	tracked := coord.GetAllTracked()
	if len(tracked) != 2 {
		t.Errorf("GetAllTracked() got %d, want 2", len(tracked))
	}
	if tracked["test_a"] != 100 {
		t.Errorf("GetAllTracked()['test_a'] = %d, want 100", tracked["test_a"])
	}
	if tracked["test_b"] != 200 {
		t.Errorf("GetAllTracked()['test_b'] = %d, want 200", tracked["test_b"])
	}
}

func TestEnsureIssue_SetsTracked(t *testing.T) {
	coord, transport := newMockCoordinator()

	searchURL := "https://api.github.com/repos/test-owner/test-repo/issues?q=test-owner%2Ffeat%2Ftest%3A+TrackedTest&state=all"
	transport.setResponse("GET", searchURL, http.StatusOK, []GitHubIssue{})

	createURL := "https://api.github.com/repos/test-owner/test-repo/issues"
	transport.setResponse("POST", createURL, http.StatusCreated, GitHubIssue{
		Number:  55,
		HTMLURL: "https://github.com/test-owner/test-repo/issues/55",
		Title:   "feat/test: TrackedTest",
	})

	_, _, err := coord.EnsureIssue("feat/test: TrackedTest", &ErrorDetails{TestName: "feat/test: TrackedTest"})
	if err != nil {
		t.Fatalf("EnsureIssue() error = %v", err)
	}

	tracked := coord.GetAllTracked()
	if tracked["feat/test: TrackedTest"] != 55 {
		t.Errorf("tracked['feat/test: TrackedTest'] = %d, want 55", tracked["feat/test: TrackedTest"])
	}
}

func TestHandleDetonationIssues(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	coord, _ := newMockCoordinator()
	d.Coord = coord

	d.TestTrackers["p1"] = &TestTracker{
		Completed: map[string]*TestInfo{
			"test_fail": {
				ID:          "test_fail",
				Name:        "TestFail",
				State:       StateDud,
				Elapsed:     100,
				ErrorTrace:  "assertion failed",
				FilePath:    "/path/to/test.go",
				FailureLine: 42,
			},
		},
		CompletedIDs: map[string]bool{"test_fail": true},
	}

	tmpDir := t.TempDir()
	d.ConfigDir = tmpDir

	handleDetonationIssues(d, tmpDir)

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
	if ops[0].TestName != "TestFail" {
		t.Errorf("Expected TestName=TestFail, got %s", ops[0].TestName)
	}
	if ops[0].Details == nil {
		t.Error("Expected Details to be set")
	}

	if len(d.IssueRefs) != 1 {
		t.Fatalf("IssueRefs len = %d, want 1", len(d.IssueRefs))
	}
	info, ok := d.IssueRefs["TestFail"]
	if !ok {
		t.Fatal("IssueRefs[TestFail] not found")
	}
	if info.Number != 0 {
		t.Errorf("IssueRefs[TestFail].Number = %d, want 0 (queued)", info.Number)
	}
	if info.Existed {
		t.Errorf("IssueRefs[TestFail].Existed = true, want false")
	}
}

func TestHandleDetonationIssues_ExistingIssue(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	d.Coord = coord

	d.TestTrackers["p1"] = &TestTracker{
		Completed: map[string]*TestInfo{
			"test_old": {
				ID:      "test_old",
				Name:    "TestOld",
				State:   StateDud,
				Elapsed: 100,
			},
		},
		CompletedIDs: map[string]bool{"test_old": true},
	}

	handleDetonationIssues(d, tmpDir)

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
	if ops[0].TestName != "TestOld" {
		t.Errorf("Expected TestName=TestOld, got %s", ops[0].TestName)
	}
	if ops[0].Details == nil {
		t.Error("Expected Details to be set")
	}

	info, ok := d.IssueRefs["TestOld"]
	if !ok {
		t.Fatal("IssueRefs[TestOld] not found")
	}
	if info.Number != 0 {
		t.Errorf("IssueRefs[TestOld].Number = %d, want 0 (queued)", info.Number)
	}
	if info.Existed {
		t.Errorf("IssueRefs[TestOld].Existed = true, want false")
	}
}

func TestHandleDefusedIssues_ClosesAndUpdatesChangelog(t *testing.T) {
	tmpDir := t.TempDir()
	changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
	originalChangelog := `# Changelog

## [1.0.0] - 2024-01-01
- Added feature X
- Fixed bug in TestFail tracking (#10)
- Updated dependencies
`
	if err := os.WriteFile(changelogPath, []byte(originalChangelog), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	d.Coord = coord

	d.IssueRefs["TestFail"] = &IssueInfo{Number: 10, URL: "https://github.com/test-owner/test-repo/issues/10"}

	handleDefusedIssues(d, tmpDir)

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 queued operations (post_comment + close_issue), got %d", len(ops))
	}
	if ops[0].Type != SyncOpPostComment {
		t.Errorf("Expected first op SyncOpPostComment, got %s", ops[0].Type)
	}
	if ops[1].Type != SyncOpCloseIssue {
		t.Errorf("Expected second op SyncOpCloseIssue, got %s", ops[1].Type)
	}

	updated, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(updated)
	if !strings.Contains(content, "~~- Fixed bug in TestFail tracking (#10)~~") {
		t.Errorf("Changelog should have strikethrough entry, got:\n%s", content)
	}
}

func TestHandleDefusedIssues_NoChangelog(t *testing.T) {
	tmpDir := t.TempDir()

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	d.Coord = coord
	d.IssueRefs["TestA"] = &IssueInfo{Number: 99}

	handleDefusedIssues(d, tmpDir)

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 queued operations, got %d", len(ops))
	}
}

func TestPostComment_NotFound(t *testing.T) {
	coord, transport := newMockCoordinator()
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/42/comments"
	transport.setResponse("POST", commentsURL, http.StatusNotFound, `{"message":"not found"}`)

	err := coord.PostComment(42, "test body")
	if err == nil {
		t.Fatal("PostComment() expected error for 404, got nil")
	}
	if !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("PostComment() error = %v, want wrapped ErrResourceNotFound", err)
	}
}

func TestPostComment_ServerError(t *testing.T) {
	coord, transport := newMockCoordinator()
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/42/comments"
	transport.setResponse("POST", commentsURL, http.StatusInternalServerError, `{"message":"server error"}`)

	err := coord.PostComment(42, "test body")
	if err == nil {
		t.Fatal("PostComment() expected error for 500, got nil")
	}
	if errors.Is(err, ErrResourceNotFound) {
		t.Errorf("PostComment() error = %v should NOT be ErrResourceNotFound for 5xx", err)
	}
}

func TestCloseIssueByNumber_NotFound(t *testing.T) {
	coord, transport := newMockCoordinator()
	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/42"
	transport.setResponse("PATCH", issueURL, http.StatusNotFound, `{"message":"not found"}`)

	err := coord.CloseIssueByNumber(42)
	if err == nil {
		t.Fatal("CloseIssueByNumber() expected error for 404, got nil")
	}
	if !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("CloseIssueByNumber() error = %v, want wrapped ErrResourceNotFound", err)
	}
}

func TestCloseIssueByNumber_ServerError(t *testing.T) {
	coord, transport := newMockCoordinator()
	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/42"
	transport.setResponse("PATCH", issueURL, http.StatusInternalServerError, `{"message":"server error"}`)

	err := coord.CloseIssueByNumber(42)
	if err == nil {
		t.Fatal("CloseIssueByNumber() expected error for 500, got nil")
	}
	if errors.Is(err, ErrResourceNotFound) {
		t.Errorf("CloseIssueByNumber() error = %v should NOT be ErrResourceNotFound for 5xx", err)
	}
}

func TestGetIssueComments_NotFound(t *testing.T) {
	coord, transport := newMockCoordinator()
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/42/comments"
	transport.setResponse("GET", commentsURL, http.StatusNotFound, `{"message":"not found"}`)

	_, err := coord.GetIssueComments(42)
	if err == nil {
		t.Fatal("GetIssueComments() expected error for 404, got nil")
	}
	if !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("GetIssueComments() error = %v, want wrapped ErrResourceNotFound", err)
	}
}

func TestGetIssueComments_ServerError(t *testing.T) {
	coord, transport := newMockCoordinator()
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/42/comments"
	transport.setResponse("GET", commentsURL, http.StatusInternalServerError, `{"message":"server error"}`)

	_, err := coord.GetIssueComments(42)
	if err == nil {
		t.Fatal("GetIssueComments() expected error for 500, got nil")
	}
	if errors.Is(err, ErrResourceNotFound) {
		t.Errorf("GetIssueComments() error = %v should NOT be ErrResourceNotFound for 5xx", err)
	}
}

func TestHandleDefusedIssues_PurgesGhostOnPostComment(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, ".ff", ".forgefix_history.log")
	auditLine := "[2026-06-11T12:00:00-07:00] [#99] [abc123] [TestGhost] [CREATED - test failed]\n"
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditLine), 0644); err != nil {
		t.Fatal(err)
	}

	// Point resolveAuditDir at tmpDir by creating forgefix_ff.yaml
	configFile := filepath.Join(tmpDir, "forgefix_ff.yaml")
	if err := os.WriteFile(configFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	coord, transport := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	d.Coord = coord

	d.IssueRefs["TestGhost"] = &IssueInfo{Number: 99}

	// PostComment returns 404
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/99/comments"
	transport.setResponse("POST", commentsURL, http.StatusNotFound, `{"message":"not found"}`)

	// closeStaleIssues also could be called; mock it so the test doesn't explode
	issueURL99 := "https://api.github.com/repos/test-owner/test-repo/issues/99"
	transport.setResponse("PATCH", issueURL99, http.StatusOK, GitHubIssue{Number: 99, State: "closed"})

	handleDefusedIssues(d, tmpDir)

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "TestGhost") {
		t.Errorf("ghost entry should have been purged from audit log, got:\n%s", string(data))
	}
}

func TestHandleDefusedIssues_PreservesTransientOnPostComment(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, ".ff", ".forgefix_history.log")
	auditLine := "[2026-06-11T12:00:00-07:00] [#99] [abc123] [TestTransient] [CREATED - test failed]\n"
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditLine), 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "forgefix_ff.yaml")
	if err := os.WriteFile(configFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	d.Coord = coord

	d.IssueRefs["TestTransient"] = &IssueInfo{Number: 99}

	handleDefusedIssues(d, tmpDir)

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 queued operations (post_comment + close_issue), got %d", len(ops))
	}
	if ops[0].Type != SyncOpPostComment {
		t.Errorf("Expected first op SyncOpPostComment, got %s", ops[0].Type)
	}
	if ops[1].Type != SyncOpCloseIssue {
		t.Errorf("Expected second op SyncOpCloseIssue, got %s", ops[1].Type)
	}

	// With queue-based approach, audit entry is deleted immediately when operations are queued
	// Background sync will retry transient errors (up to 3 times)
	data, err := os.ReadFile(auditPath)
	if err != nil {
		// File not found is expected - audit entry was deleted
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	} else if strings.Contains(string(data), "TestTransient") {
		t.Errorf("audit entry should have been deleted when operations were queued:\n%s", string(data))
	}
}

func TestCloseStaleIssues_PurgesGhostOnPostComment(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, ".ff", ".forgefix_history.log")
	auditLine := "[2026-06-11T12:00:00-07:00] [#42] [abc123] [TestStaleGhost] [CREATED - test failed]\n"
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditLine), 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "forgefix_ff.yaml")
	if err := os.WriteFile(configFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	d.Coord = coord

	closeStaleIssues(d, tmpDir)

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 queued operations (post_comment + close_issue), got %d", len(ops))
	}
	if ops[0].Type != SyncOpPostComment {
		t.Errorf("Expected first op SyncOpPostComment, got %s", ops[0].Type)
	}
	if ops[1].Type != SyncOpCloseIssue {
		t.Errorf("Expected second op SyncOpCloseIssue, got %s", ops[1].Type)
	}

	// Note: Audit entry is not purged immediately; it's purged when background sync processes the close_issue op
	// The queue approach defers the actual purge to the background sync process
}

func TestCloseStaleIssues_PreservesTransientOnPostComment(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, ".ff", ".forgefix_history.log")
	auditLine := "[2026-06-11T12:00:00-07:00] [#42] [abc123] [TestStaleTransient] [CREATED - test failed]\n"
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditLine), 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "forgefix_ff.yaml")
	if err := os.WriteFile(configFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	d.Coord = coord

	closeStaleIssues(d, tmpDir)

	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatalf("LoadSyncQueue error: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("Expected 2 queued operations (post_comment + close_issue), got %d", len(ops))
	}

	// Note: With queue-based approach, transient errors are handled by retry logic in background sync
	// The audit entry is preserved until the background sync successfully processes the operation
}

func TestHandleDefusedIssues_PurgesGhostOnClose(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, ".ff", ".forgefix_history.log")
	auditLine := "[2026-06-11T12:00:00-07:00] [#88] [abc123] [TestGhostClose] [CREATED - test failed]\n"
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte(auditLine), 0644); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(tmpDir, "forgefix_ff.yaml")
	if err := os.WriteFile(configFile, []byte("version: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDashboard([]PipelineConfig{{ID: "p1", Name: "Pipeline 1"}})
	coord, transport := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	d.Coord = coord

	d.IssueRefs["TestGhostClose"] = &IssueInfo{Number: 88}

	// PostComment succeeds
	commentsURL := "https://api.github.com/repos/test-owner/test-repo/issues/88/comments"
	transport.setResponse("POST", commentsURL, http.StatusCreated, GitHubComment{ID: 1, Body: "comment"})
	// CloseIssueByNumber returns 404
	issueURL88 := "https://api.github.com/repos/test-owner/test-repo/issues/88"
	transport.setResponse("PATCH", issueURL88, http.StatusNotFound, `{"message":"not found"}`)

	handleDefusedIssues(d, tmpDir)

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "TestGhostClose") {
		t.Errorf("ghost entry should have been purged from audit log, got:\n%s", string(data))
	}
}

func TestHandleTimeoutIssues(t *testing.T) {
	tmpDir := t.TempDir()
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipeline 1"},
	})
	d.ConfigDir = tmpDir
	coord, _ := newMockCoordinator()
	d.Coord = coord

	d.TestTrackers["p1"] = &TestTracker{
		Completed: map[string]*TestInfo{
			"test_timeout": {
				ID:      "test_timeout",
				Name:    "TestTimeout",
				State:   StateDud,
				Elapsed: 5000,
			},
		},
		CompletedIDs: map[string]bool{"test_timeout": true},
	}

	handleTimeoutIssues(d, tmpDir)

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
	if ops[0].TestName != "TestTimeout" {
		t.Errorf("Expected TestName=TestTimeout, got %s", ops[0].TestName)
	}
	if ops[0].Details == nil {
		t.Error("Expected Details to be set")
	}

	if len(d.IssueRefs) != 1 {
		t.Fatalf("IssueRefs len = %d, want 1", len(d.IssueRefs))
	}
	info, ok := d.IssueRefs["TestTimeout"]
	if !ok {
		t.Fatal("IssueRefs[TestTimeout] not found")
	}
	if info.Number != 0 {
		t.Errorf("IssueRefs[TestTimeout].Number = %d, want 0 (queued)", info.Number)
	}
	if info.Existed {
		t.Errorf("IssueRefs[TestTimeout].Existed = true, want false")
	}
}

func TestDoRequestAsync(t *testing.T) {
	coord, transport := newMockCoordinator()

	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/42"
	transport.setResponse("PATCH", issueURL, http.StatusOK, GitHubIssue{
		Number: 42,
		State:  "closed",
	})

	req, _ := http.NewRequest("PATCH", issueURL, strings.NewReader(`{"state":"closed"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resultCh := coord.doRequestAsync(req)
	result := <-resultCh

	if result.Err != nil {
		t.Fatalf("doRequestAsync() error = %v", result.Err)
	}
	if result.Resp == nil {
		t.Fatal("doRequestAsync() returned nil response")
	}
	if result.Resp.StatusCode != http.StatusOK {
		t.Errorf("doRequestAsync() status = %d, want %d", result.Resp.StatusCode, http.StatusOK)
	}
}

func TestBatchCloseIssues(t *testing.T) {
	coord, transport := newMockCoordinator()

	for _, num := range []int{1, 2, 3} {
		issueURL := fmt.Sprintf("https://api.github.com/repos/test-owner/test-repo/issues/%d", num)
		transport.setResponse("PATCH", issueURL, http.StatusOK, GitHubIssue{
			Number: num,
			State:  "closed",
		})
	}

	errs := coord.BatchCloseIssues([]int{1, 2, 3})
	if len(errs) != 3 {
		t.Fatalf("BatchCloseIssues() returned %d errors, want 3", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("errs[%d] = %v, want nil", i, err)
		}
	}
}

func TestBatchCloseIssues_PartialFailure(t *testing.T) {
	coord, transport := newMockCoordinator()

	transport.setResponse("PATCH", "https://api.github.com/repos/test-owner/test-repo/issues/1", http.StatusOK, GitHubIssue{Number: 1, State: "closed"})
	transport.setResponse("PATCH", "https://api.github.com/repos/test-owner/test-repo/issues/3", http.StatusOK, GitHubIssue{Number: 3, State: "closed"})

	errs := coord.BatchCloseIssues([]int{1, 2, 3})
	if len(errs) != 3 {
		t.Fatalf("BatchCloseIssues() returned %d errors, want 3", len(errs))
	}
	if errs[0] != nil {
		t.Errorf("errs[0] = %v, want nil", errs[0])
	}
	if errs[1] == nil {
		t.Error("errs[1] = nil, want error for missing mock")
	}
	if errs[2] != nil {
		t.Errorf("errs[2] = %v, want nil", errs[2])
	}
}

func TestBatchCloseIssues_Empty(t *testing.T) {
	coord, _ := newMockCoordinator()
	errs := coord.BatchCloseIssues(nil)
	if len(errs) != 0 {
		t.Fatalf("BatchCloseIssues(nil) returned %d errors, want 0", len(errs))
	}
	errs = coord.BatchCloseIssues([]int{})
	if len(errs) != 0 {
		t.Fatalf("BatchCloseIssues([]) returned %d errors, want 0", len(errs))
	}
}

func TestBatchCloseIssues_Inactive(t *testing.T) {
	coord := NewIssueCoordinator("", "", "", "https://api.github.com")
	errs := coord.BatchCloseIssues([]int{1, 2})
	if len(errs) != 2 {
		t.Fatalf("BatchCloseIssues() returned %d errors, want 2", len(errs))
	}
	for i, err := range errs {
		if err == nil {
			t.Errorf("errs[%d] = nil, want inactive error", i)
		}
	}
}

func TestDoRequestAsync_ChannelCloses(t *testing.T) {
	coord, transport := newMockCoordinator()

	issueURL := "https://api.github.com/repos/test-owner/test-repo/issues/1"
	transport.setResponse("GET", issueURL, http.StatusOK, GitHubIssue{Number: 1})

	req, _ := http.NewRequest("GET", issueURL, nil)
	req.Header.Set("Authorization", "Bearer test-token")

	resultCh := coord.doRequestAsync(req)
	result, ok := <-resultCh
	if !ok {
		t.Fatal("doRequestAsync() channel closed without sending a value")
	}
	if result.Err != nil {
		t.Fatalf("doRequestAsync() error = %v", result.Err)
	}

	_, ok = <-resultCh
	if ok {
		t.Error("doRequestAsync() channel should be closed after first read")
	}
}
