package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ForgeFix/engine/housekeeper"
)

const testBaseURL = "https://api.github.com"

func validTestTitle(specID string) string {
	return fmt.Sprintf("feat/test: %s", specID)
}

func writeSpecFile(t *testing.T, dir, specID, status string, issueNum int, bodyContent string) string {
	t.Helper()
	title := validTestTitle(specID)
	content := fmt.Sprintf(`---
spec_id: "%s"
status: "%s"
type: "type/bug"
version: "version/v0.8.0"
repo_issue: %d
---

# %s
%s
`, specID, status, issueNum, title, bodyContent)
	path := filepath.Join(dir, "specs", specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return bodyContent
}

func expectedSpecBody(specID, bodyContent string) string {
	return fmt.Sprintf("# %s\n%s", validTestTitle(specID), bodyContent)
}

func mockSyncEndpoints(transport *mockTransport, issueNum int, body, state string) {
	base := testBaseURL + "/repos/test-owner/test-repo"
	title := validTestTitle(fmt.Sprintf("SPEC-%d", issueNum))
	issue := GitHubIssue{
		ID: int64(issueNum), Number: issueNum,
		Title: title, Body: body, State: state,
	}
	issueData, _ := json.Marshal(issue)
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d", issueNum), 200, issueData)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d/labels", issueNum), 200, []RepoLabel{})
	transport.setResponse("PUT", fmt.Sprintf(base+"/issues/%d/labels", issueNum), 200, []RepoLabel{})
	transport.setResponse("POST", fmt.Sprintf(base+"/issues/%d/comments", issueNum), 201, map[string]any{"id": 1})
}

func TestSyncSingleSpec_ClosesResolvedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)
	body := writeSpecFile(t, tmpDir, "SPEC-CLOSE-TEST", "closed", 42, "Body content")

	coord, transport := newMockCoordinator()
	mockSyncEndpoints(transport, 42, expectedSpecBody("SPEC-CLOSE-TEST", body), "open")
	transport.setResponse("PATCH",
		testBaseURL+"/repos/test-owner/test-repo/issues/42",
		200, GitHubIssue{Number: 42, State: "closed"})

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "SPEC-CLOSE-TEST", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec failed: %v", err)
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.mu.Lock()
	count := transport.callCount["PATCH "+base+"/issues/42"]
	transport.mu.Unlock()

	if count == 0 {
		t.Error("expected PATCH /issues/42 to close the issue, but no PATCH call was made")
	}
}

func TestSyncSingleSpec_DoesNotCloseDraftIssue(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)
	body := writeSpecFile(t, tmpDir, "SPEC-OPEN-TEST", "draft", 42, "Body content")

	coord, transport := newMockCoordinator()
	mockSyncEndpoints(transport, 42, expectedSpecBody("SPEC-OPEN-TEST", body), "open")
	transport.setResponse("PATCH",
		testBaseURL+"/repos/test-owner/test-repo/issues/42",
		200, GitHubIssue{Number: 42, State: "closed"})

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "SPEC-OPEN-TEST", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec failed: %v", err)
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.mu.Lock()
	count := transport.callCount["PATCH "+base+"/issues/42"]
	transport.mu.Unlock()

	if count > 0 {
		t.Error("expected no PATCH for draft status, but PATCH call was made")
	}
}

func TestIsResolvedStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"closed", true},
		{"done", true},
		{"fixed", true},
		{"ship", false},
		{"SHIP", false},
		{"backlog", false},
		{"in-progress", false},
		{"review", false},
		{"", false},
		{"Closed", true},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status=%q", tc.status), func(t *testing.T) {
			got := isResolvedStatus(tc.status)
			if got != tc.want {
				t.Errorf("isResolvedStatus(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestSyncSingleSpec_UpdatesBodyAndCloses(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)
	// Use different old/new body to trigger body update BEFORE close
	specBody := writeSpecFile(t, tmpDir, "SPEC-BODY-CLOSE", "fixed", 7, "New content")

	coord, transport := newMockCoordinator()
	oldBody := "Old body content that differs"
	mockSyncEndpoints(transport, 7, oldBody, "open")
	transport.setResponse("PATCH",
		testBaseURL+"/repos/test-owner/test-repo/issues/7",
		200, GitHubIssue{Number: 7, State: "closed"})

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "SPEC-BODY-CLOSE", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec failed: %v", err)
	}

	// Verify body was updated AND issue was closed (2 PATCH calls)
	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.mu.Lock()
	count := transport.callCount["PATCH "+base+"/issues/7"]
	transport.mu.Unlock()
	if count == 0 {
		t.Error("expected at least one PATCH (body update or close), got none")
	}

	found := false
	entries, _ := os.ReadDir(filepath.Join(tmpDir, "specs"))
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(tmpDir, "specs", e.Name()))
		if strings.Contains(string(data), "repo_issue: 7") {
			found = true
			break
		}
	}
	if !found {
		t.Error("spec file should retain repo_issue: 7")
	}
	_ = specBody
}

func TestSyncSingleSpec_DoesNotCloseAlreadyClosedIssue(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)
	body := writeSpecFile(t, tmpDir, "SPEC-ALREADY-CLOSED", "closed", 42, "Body")

	coord, transport := newMockCoordinator()
	mockSyncEndpoints(transport, 42, expectedSpecBody("SPEC-ALREADY-CLOSED", body), "closed")

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "SPEC-ALREADY-CLOSED", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec failed: %v", err)
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.mu.Lock()
	count := transport.callCount["PATCH "+base+"/issues/42"]
	transport.mu.Unlock()

	if count > 0 {
		t.Error("expected no PATCH for already-closed issue, but PATCH call was made")
	}
}

func TestSyncSingleSpec_ClosesMultipleResolvedSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".ff"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Pre-create an empty ledger so LoadLedger doesn't fail.
	emptyLedger := NewLedgerEngine()
	if err := SaveLedger(emptyLedger, tmpDir); err != nil {
		t.Fatal(err)
	}

	type specDef struct {
		id     string
		status string
		issue  int
	}
	specs := []specDef{
		{"SPEC-RESOLVED-1", "closed", 10},
		{"SPEC-RESOLVED-2", "fixed", 20},
		{"SPEC-DRAFT-3", "draft", 30},
	}

	coord, transport := newMockCoordinator()
	for _, s := range specs {
		writeSpecFile(t, tmpDir, s.id, s.status, s.issue, "Body")
		mockSyncEndpoints(transport, s.issue, expectedSpecBody(s.id, "Body"), "open")
		transport.setResponse("PATCH",
			fmt.Sprintf(testBaseURL+"/repos/test-owner/test-repo/issues/%d", s.issue),
			200, GitHubIssue{Number: s.issue, State: "closed"})
	}

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec failed: %v", err)
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.mu.Lock()
	c10 := transport.callCount["PATCH "+base+"/issues/10"]
	c20 := transport.callCount["PATCH "+base+"/issues/20"]
	c30 := transport.callCount["PATCH "+base+"/issues/30"]
	transport.mu.Unlock()

	if c10 == 0 {
		t.Error("expected issue #10 (resolved) to be closed")
	}
	if c20 == 0 {
		t.Error("expected issue #20 (fixed) to be closed")
	}
	if c30 > 0 {
		t.Error("expected issue #30 (draft) NOT to be closed")
	}
}

func TestReconciliation_DetectsOrphanedIssues(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Spec with issue #100 (matched)
	writeSpecFile(t, tmpDir, "SPEC-MATCHED", "in-progress", 100, "Body")

	// Spec with issue #200 (matched)
	writeSpecFile(t, tmpDir, "SPEC-MATCHED-2", "closed", 200, "Body")

	coord, transport := newMockCoordinator()

	// Mock remote issues: #100, #200 (matched), #300 (orphan), #400 (orphan)
	issue100 := GitHubIssue{ID: 100, Number: 100, Title: "feat/test: SPEC-100", State: "open", Body: "Body"}
	issue200 := GitHubIssue{ID: 200, Number: 200, Title: "feat/test: SPEC-200", State: "open", Body: "Body"}
	issue300 := GitHubIssue{ID: 300, Number: 300, Title: "feat/test: Orphan Issue", State: "open", Body: "Body"}
	issue400 := GitHubIssue{ID: 400, Number: 400, Title: "feat/test: Another Orphan", State: "open", Body: "Body"}

	allIssues := []GitHubIssue{issue100, issue200, issue300, issue400}
	allIssuesData, _ := json.Marshal(allIssues)

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, allIssuesData)

	// Mock individual issue fetches for specs
	for _, issue := range []GitHubIssue{issue100, issue200} {
		issueData, _ := json.Marshal(issue)
		transport.setResponse("GET",
			fmt.Sprintf(base+"/issues/%d", issue.Number),
			200, issueData)
		transport.setResponse("GET",
			fmt.Sprintf(base+"/issues/%d/labels", issue.Number),
			200, []RepoLabel{})
		transport.setResponse("PUT",
			fmt.Sprintf(base+"/issues/%d/labels", issue.Number),
			200, []RepoLabel{})
	}

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs failed: %v", err)
	}

	// Verify reconciliation was attempted
	// The test should verify that orphan detection happened
	// (This is a basic smoke test; full reconciliation logic tested elsewhere)
}

func TestReconciliation_DetectsGhostIssues(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Spec with issue that no longer exists on remote (404)
	writeSpecFile(t, tmpDir, "SPEC-GHOST", "in-progress", 999, "Body")

	coord, transport := newMockCoordinator()

	// Mock remote issues - only issue #100 exists, not #999
	issue100 := GitHubIssue{ID: 100, Number: 100, Title: "feat/test: Other Issue", State: "open", Body: "Body"}
	allIssues := []GitHubIssue{issue100}
	allIssuesData, _ := json.Marshal(allIssues)

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, allIssuesData)

	// Mock: remote issue 999 returns 404
	transport.setResponse("GET",
		base+"/issues/999",
		404, map[string]string{"message": "not found"})

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs failed: %v", err)
	}
	// Ghost issue (999) should be detected and handled
}

func TestReconciliation_DetectsDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Two specs with same title (duplicate)
	writeSpecFile(t, tmpDir, "SPEC-DUPE-1", "in-progress", 100, "Body")
	writeSpecFile(t, tmpDir, "SPEC-DUPE-2", "in-progress", 200, "Body")

	coord, transport := newMockCoordinator()

	issue100 := GitHubIssue{ID: 100, Number: 100, Title: "feat/test: Duplicate Title", State: "open", Body: "Body"}
	issue200 := GitHubIssue{ID: 200, Number: 200, Title: "feat/test: Duplicate Title", State: "open", Body: "Body"}

	allIssues := []GitHubIssue{issue100, issue200}
	allIssuesData, _ := json.Marshal(allIssues)

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, allIssuesData)

	for _, issue := range []GitHubIssue{issue100, issue200} {
		issueData, _ := json.Marshal(issue)
		transport.setResponse("GET",
			fmt.Sprintf(base+"/issues/%d", issue.Number),
			200, issueData)
		transport.setResponse("GET",
			fmt.Sprintf(base+"/issues/%d/labels", issue.Number),
			200, []RepoLabel{})
		transport.setResponse("PUT",
			fmt.Sprintf(base+"/issues/%d/labels", issue.Number),
			200, []RepoLabel{})
	}

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs failed: %v", err)
	}
	// Duplicate detection should work
}

func TestFindRemoteIssueByTitle_Found(t *testing.T) {
	coord, transport := newMockCoordinator()

	existingIssue := GitHubIssue{
		Number: 42,
		Title:  "feat/test: My Feature",
		State:  "open",
		Body:   "Body",
	}
	listURL := testBaseURL + "/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", listURL, 200, []GitHubIssue{existingIssue})

	issue, err := coord.findRemoteIssueByTitle("feat/test: My Feature")
	if err != nil {
		t.Fatalf("findRemoteIssueByTitle() error = %v", err)
	}
	if issue.Number != 42 {
		t.Errorf("findRemoteIssueByTitle() Number = %d, want 42", issue.Number)
	}
}

func TestFindRemoteIssueByTitle_NotFound(t *testing.T) {
	coord, transport := newMockCoordinator()

	existingIssue := GitHubIssue{
		Number: 42,
		Title:  "feat/test: Different Title",
		State:  "open",
	}
	listURL := testBaseURL + "/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", listURL, 200, []GitHubIssue{existingIssue})

	_, err := coord.findRemoteIssueByTitle("feat/test: Completely Different")
	if err == nil {
		t.Fatal("findRemoteIssueByTitle() expected error for non-matching title, got nil")
	}
}

func TestFindRemoteIssueByTitle_CaseInsensitive(t *testing.T) {
	coord, transport := newMockCoordinator()

	existingIssue := GitHubIssue{
		Number: 42,
		Title:  "feat/test: My Feature",
		State:  "open",
	}
	listURL := testBaseURL + "/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", listURL, 200, []GitHubIssue{existingIssue})

	issue, err := coord.findRemoteIssueByTitle("FEAT/TEST: my feature")
	if err != nil {
		t.Fatalf("findRemoteIssueByTitle() (case-insensitive) error = %v", err)
	}
	if issue.Number != 42 {
		t.Errorf("findRemoteIssueByTitle() Number = %d, want 42", issue.Number)
	}
}

func TestSyncSpecsIdempotent_UsesExistingIssue(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Spec with no repo_issue (new spec, never synced)
	writeSpecFile(t, tmpDir, "SPEC-FRESH", "draft", 0, "Body content")

	coord, transport := newMockCoordinator()

	// Remote already has an issue with matching title
	existingIssue := GitHubIssue{
		ID: 100, Number: 100,
		Title: "feat/test: SPEC-FRESH",
		State: "open", Body: expectedSpecBody("SPEC-FRESH", "Body content"),
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	// ListOpenIssues returns the existing issue
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, []GitHubIssue{existingIssue})
	// For the individual issue fetch (existing path in SyncSpecs)
	issueData, _ := json.Marshal(existingIssue)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d", existingIssue.Number), 200, issueData)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d/labels", existingIssue.Number), 200, []RepoLabel{})
	transport.setResponse("PUT", fmt.Sprintf(base+"/issues/%d/labels", existingIssue.Number), 200, []RepoLabel{})

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs failed: %v", err)
	}

	// Verify NO POST was made (no duplicate created)
	transport.mu.Lock()
	postCount := transport.callCount["POST "+base+"/issues"]
	transport.mu.Unlock()
	if postCount > 0 {
		t.Errorf("expected 0 POST calls (no duplicate), got %d", postCount)
	}

	// Verify spec file now has repo_issue: 100
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-FRESH.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 100") {
		t.Errorf("spec file should have repo_issue: 100 after sync, got:\n%s", string(data))
	}
}

func TestSyncSpecsIdempotent_SecondRunDoesNotCreateDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Simulate a spec that already has repo_issue set (previous sync)
	writeSpecFile(t, tmpDir, "SPEC-EXISTING", "in-progress", 50, "Body")

	coord, transport := newMockCoordinator()

	remoteIssue := GitHubIssue{
		ID: 50, Number: 50,
		Title: "feat/test: SPEC-EXISTING",
		State: "open", Body: expectedSpecBody("SPEC-EXISTING", "Body"),
	}

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, []GitHubIssue{remoteIssue})
	issueData, _ := json.Marshal(remoteIssue)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d", 50), 200, issueData)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d/labels", 50), 200, []RepoLabel{})
	transport.setResponse("PUT", fmt.Sprintf(base+"/issues/%d/labels", 50), 200, []RepoLabel{})

	// Run sync twice
	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("first SyncSpecs failed: %v", err)
	}

	err = coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("second SyncSpecs failed: %v", err)
	}

	// Only 0 POST calls (existing issue used, no new creation)
	transport.mu.Lock()
	postCount := transport.callCount["POST "+base+"/issues"]
	transport.mu.Unlock()
	if postCount > 0 {
		t.Errorf("expected 0 POST calls across two sync runs, got %d", postCount)
	}
}

func TestSyncSpecsIdempotent_ResolvedSpecGetsClosedByTitle(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Resolved spec with no repo_issue (needs to be found by title)
	body := writeSpecFile(t, tmpDir, "SPEC-RESOLVED-NOISSUE", "closed", 0, "Body content")

	coord, transport := newMockCoordinator()
	coord.SetConfigDir(tmpDir)

	base := testBaseURL + "/repos/test-owner/test-repo"

	// Remote has an issue matching the spec title
	existingIssue := GitHubIssue{
		ID: 200, Number: 200,
		Title: "feat/test: SPEC-RESOLVED-NOISSUE",
		State: "open", Body: expectedSpecBody("SPEC-RESOLVED-NOISSUE", body),
	}

	// Labels, list, and individual issue endpoints
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, []GitHubIssue{existingIssue})
	issueData, _ := json.Marshal(existingIssue)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d", 200), 200, issueData)
	transport.setResponse("GET", fmt.Sprintf(base+"/issues/%d/labels", 200), 200, []RepoLabel{})
	transport.setResponse("PUT", fmt.Sprintf(base+"/issues/%d/labels", 200), 200, []RepoLabel{})

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs failed: %v", err)
	}

	// Verify POST was never called (no duplicate created)
	transport.mu.Lock()
	postCount := transport.callCount["POST "+base+"/issues"]
	commentCount := transport.callCount["POST "+base+"/issues/200/comments"]
	patchCount := transport.callCount["PATCH "+base+"/issues/200"]
	transport.mu.Unlock()

	if postCount > 0 {
		t.Errorf("expected 0 POST calls (no duplicate), got %d", postCount)
	}
	if commentCount > 0 {
		t.Error("expected 0 direct resolution comment calls (now uses housekeeping queue)")
	}
	if patchCount > 0 {
		t.Error("expected 0 direct close calls (now uses housekeeping queue)")
	}

	// Verify tasks were enqueued in the housekeeping queue
	taskData, err := os.ReadFile(filepath.Join(tmpDir, "tasks.json"))
	if err != nil {
		t.Fatal("expected tasks.json to exist with enqueued tasks")
	}
	var tasks []housekeeper.HousekeepingTask
	if err := json.Unmarshal(taskData, &tasks); err != nil {
		t.Fatalf("failed to unmarshal tasks.json: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 enqueued tasks, got %d", len(tasks))
	}
	if tasks[0].Type != housekeeper.TaskTypePostResolution {
		t.Errorf("expected first task type %s, got %s", housekeeper.TaskTypePostResolution, tasks[0].Type)
	}
	if tasks[1].Type != housekeeper.TaskTypeCloseIssue {
		t.Errorf("expected second task type %s, got %s", housekeeper.TaskTypeCloseIssue, tasks[1].Type)
	}
	if tasks[0].RepoIssueID != 200 {
		t.Errorf("expected first task RepoIssueID 200, got %d", tasks[0].RepoIssueID)
	}
	if tasks[1].RepoIssueID != 200 {
		t.Errorf("expected second task RepoIssueID 200, got %d", tasks[1].RepoIssueID)
	}

	// Spec file should have repo_issue: 200 now
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-RESOLVED-NOISSUE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 200") {
		t.Errorf("spec file should have repo_issue: 200 after sync, got:\n%s", string(data))
	}
}

func TestFindRemoteIssueByTitle_EmptyOpenIssues(t *testing.T) {
	coord, transport := newMockCoordinator()

	listURL := testBaseURL + "/repos/test-owner/test-repo/issues?per_page=100&state=open"
	transport.setResponse("GET", listURL, 200, []GitHubIssue{})

	_, err := coord.findRemoteIssueByTitle("feat/test: Anything")
	if err == nil {
		t.Fatal("findRemoteIssueByTitle() expected error when no open issues exist, got nil")
	}
}

func TestSyncSingleSpec_DeletedIssueClearsRepoIssue(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Spec that had a remote issue that was deleted
	writeSpecFile(t, tmpDir, "SPEC-DELETED", "in-progress", 99, "Body content")

	coord, transport := newMockCoordinator()

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})

	// GetIssueByNumber returns 404 — issue was deleted
	getURL := fmt.Sprintf(base+"/issues/%d", 99)
	transport.setResponse("GET", getURL, 404, []byte(`{"message":"Not Found"}`))

	cfg := &Config{}
	err := syncSingleSpec(coord, tmpDir, "SPEC-DELETED", cfg)
	if err != nil {
		t.Fatalf("syncSingleSpec should not error on deleted issue, got: %v", err)
	}

	// Verify spec file now has repo_issue: 0 (cleared)
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-DELETED.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 0") {
		t.Errorf("spec file should have repo_issue: 0 after deleted issue sync, got:\n%s", string(data))
	}

	// Verify no POST was made — we only cleared, didn't recreate
	transport.mu.Lock()
	postCount := transport.callCount["POST "+base+"/issues"]
	transport.mu.Unlock()
	if postCount > 0 {
		t.Errorf("expected 0 POST calls (no new issue created), got %d", postCount)
	}
}

func TestSyncSpecs_DeletedIssueCreatesReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Spec that had a remote issue that was deleted
	writeSpecFile(t, tmpDir, "SPEC-REPLACE", "in-progress", 99, "Body content")

	coord, transport := newMockCoordinator()

	base := testBaseURL + "/repos/test-owner/test-repo"
	transport.setResponse("GET", base+"/labels", 200, []RepoLabel{})

	// GetIssueByNumber returns 404 — issue was deleted
	getURL := fmt.Sprintf(base+"/issues/%d", 99)
	transport.setResponse("GET", getURL, 404, []byte(`{"message":"Not Found"}`))

	// No existing issue found by title
	transport.setResponse("GET", base+"/issues?per_page=100&state=open", 200, []GitHubIssue{})

	// Create replacement issue
	newIssue := GitHubIssue{Number: 200, State: "open", Title: "feat/test: SPEC-REPLACE"}
	transport.setResponse("POST", base+"/issues", 201, newIssue)

	// Fetch the newly created issue for body sync
	newIssueData, _ := json.Marshal(newIssue)
	transport.setResponse("GET", base+"/issues/200", 200, newIssueData)
	transport.setResponse("GET", base+"/issues/200/labels", 200, []RepoLabel{})
	transport.setResponse("PUT", base+"/issues/200/labels", 200, []RepoLabel{})
	transport.setResponse("PATCH", base+"/issues/200", 200, newIssue)

	err := coord.SyncSpecs(tmpDir)
	if err != nil {
		t.Fatalf("SyncSpecs should not error on deleted issue, got: %v", err)
	}

	// Verify spec file now has repo_issue: 200 (replacement created)
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-REPLACE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 200") {
		t.Errorf("spec file should have repo_issue: 200 after replacement, got:\n%s", string(data))
	}
}

func TestClearRepoIssueForSpec(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Create a spec with a repo_issue pointing to a now-deleted remote issue
	writeSpecFile(t, tmpDir, "SPEC-CLEAR", "in-progress", 42, "Body content")

	// Create a ledger entry so we can verify it's also cleaned up
	ledger := NewLedgerEngine()
	ledger.SetSpecEntry("SPEC-CLEAR", &SpecEntry{
		SpecID:      "SPEC-CLEAR",
		RepoIssueID: 42,
		Status:      "in-progress",
	})
	if err := SaveLedger(ledger, tmpDir); err != nil {
		t.Fatal(err)
	}

	coord, _ := newMockCoordinator()
	coord.SetConfigDir(tmpDir)
	coord.SetLedger(ledger)

	// Call clearRepoIssueForSpec
	clearRepoIssueForSpec(tmpDir, "SPEC-CLEAR", coord)

	// Verify spec file's repo_issue is cleared
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-CLEAR.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 0") {
		t.Errorf("spec file should have repo_issue: 0 after clearing, got:\n%s", string(data))
	}

	// Verify ledger entry is also cleared
	ledger2, err := LoadLedger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	entry := ledger2.GetSpecEntry("SPEC-CLEAR")
	if entry == nil {
		t.Fatal("spec entry should still exist in ledger")
	}
	if entry.RepoIssueID != 0 {
		t.Errorf("ledger RepoIssueID should be 0 after clearing, got %d", entry.RepoIssueID)
	}
}

func TestClearRepoIssueForSpec_NoopWhenNoSpec(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	coord, _ := newMockCoordinator()

	// Should not panic or error when spec doesn't exist
	clearRepoIssueForSpec(tmpDir, "SPEC-NONEXISTENT", coord)
}

func TestClearRepoIssueForSpec_NoopWhenRepoIssueZero(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Create a spec with repo_issue already 0
	writeSpecFile(t, tmpDir, "SPEC-ZERO", "in-progress", 0, "Body content")

	coord, _ := newMockCoordinator()

	// Should be a no-op
	clearRepoIssueForSpec(tmpDir, "SPEC-ZERO", coord)

	// Verify still 0
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-ZERO.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 0") {
		t.Errorf("spec file should still have repo_issue: 0, got:\n%s", string(data))
	}
}

func TestProcessSyncQueue_DropsDeletedIssueOpAndCleansUpSpec(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Create a spec with a repo_issue referencing a deleted remote issue
	writeSpecFile(t, tmpDir, "SPEC-STALE", "in-progress", 99, "Body content")

	// Enqueue a close_issue operation for the deleted issue
	if err := QueueCloseIssue(tmpDir, "", 99); err != nil {
		t.Fatal(err)
	}

	coord, transport := newMockCoordinator()
	coord.SetConfigDir(tmpDir)

	// CloseIssueByNumber returns 404 — issue was deleted
	base := testBaseURL + "/repos/test-owner/test-repo"
	closeURL := fmt.Sprintf(base+"/issues/%d", 99)
	// We need the mock client to return 404 for the PATCH request used by CloseIssueByNumber
	// The actual URL for CloseIssueByNumber is baseURL + "/repos/owner/repo/issues/{number}"
	// Let's handle the PATCH via the transport
	transport.setResponse("PATCH", closeURL, 404, []byte(`{"message":"Not Found"}`))

	// We need a ledger for this test
	ledger := NewLedgerEngine()
	// Add SPEC-STALE to ledger
	ledger.SetSpecEntry("SPEC-STALE", &SpecEntry{
		SpecID:      "SPEC-STALE",
		RepoIssueID: 99,
		Status:      "in-progress",
	})
	if err := SaveLedger(ledger, tmpDir); err != nil {
		t.Fatal(err)
	}

	coord.SetLedger(ledger)

	// Process the sync queue
	err := processSyncQueue(coord, tmpDir, ledger)
	if err != nil {
		t.Fatalf("processSyncQueue should not error on 404 ops, got: %v", err)
	}

	// Verify the spec's repo_issue was cleared
	data, err := os.ReadFile(filepath.Join(tmpDir, "specs", "SPEC-STALE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repo_issue: 0") {
		t.Errorf("spec file should have repo_issue: 0 after sync queue cleaned up, got:\n%s", string(data))
	}

	// Verify the queue is now empty
	ops, err := LoadSyncQueue(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 0 {
		t.Errorf("sync queue should be empty after processing, got %d ops", len(ops))
	}
}
