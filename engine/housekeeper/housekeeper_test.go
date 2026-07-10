package housekeeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Mock implementations

type mockCommenter struct {
	PostCommentFn func(issueNumber int, body string) error
}

func (m *mockCommenter) PostComment(issueNumber int, body string) error {
	return m.PostCommentFn(issueNumber, body)
}

type mockIssueCloser struct {
	CloseIssueByNumberFn func(issueNumber int) error
}

func (m *mockIssueCloser) CloseIssueByNumber(issueNumber int) error {
	return m.CloseIssueByNumberFn(issueNumber)
}

func TestNewHousekeepingQueue(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)
	if q == nil {
		t.Fatal("NewHousekeepingQueue returned nil")
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue, got %d tasks", q.Len())
	}
}

func TestEnqueueAndDequeue(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	task := HousekeepingTask{
		Type:        TaskTypeCloseIssue,
		SpecID:      "SPEC-123",
		RepoIssueID: 42,
		Priority:    PriorityHigh,
		Payload:     `{"action":"close"}`,
		Context:     map[string]string{"commit": "abc123"},
	}

	if err := q.Enqueue(task); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if q.Len() != 1 {
		t.Errorf("expected 1 task, got %d", q.Len())
	}

	dequeued, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned false on non-empty queue")
	}

	if dequeued.Type != TaskTypeCloseIssue {
		t.Errorf("expected TaskTypeCloseIssue, got %s", dequeued.Type)
	}
	if dequeued.SpecID != "SPEC-123" {
		t.Errorf("expected SPEC-123, got %s", dequeued.SpecID)
	}
	if dequeued.RepoIssueID != 42 {
		t.Errorf("expected 42, got %d", dequeued.RepoIssueID)
	}
	if dequeued.Priority != PriorityHigh {
		t.Errorf("expected PriorityHigh, got %d", dequeued.Priority)
	}
	if dequeued.ID == "" {
		t.Error("expected ID to be generated")
	}
	if dequeued.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if q.Len() != 0 {
		t.Errorf("expected empty queue after dequeue, got %d", q.Len())
	}
}

func TestDequeueEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	_, ok := q.Dequeue()
	if ok {
		t.Error("expected Dequeue on empty queue to return false")
	}
}

func TestPriorityOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	low := HousekeepingTask{Type: TaskTypeSyncMetadata, SpecID: "SPEC-1", Priority: PriorityLow, CreatedAt: time.Now()}
	medium := HousekeepingTask{Type: TaskTypeSyncMetadata, SpecID: "SPEC-2", Priority: PriorityMedium, CreatedAt: time.Now()}
	high := HousekeepingTask{Type: TaskTypeSyncMetadata, SpecID: "SPEC-3", Priority: PriorityHigh, CreatedAt: time.Now()}

	q.Enqueue(low)
	q.Enqueue(medium)
	q.Enqueue(high)

	first, _ := q.Dequeue()
	if first.Priority != PriorityHigh {
		t.Errorf("expected high priority first, got %d", first.Priority)
	}

	second, _ := q.Dequeue()
	if second.Priority != PriorityMedium {
		t.Errorf("expected medium priority second, got %d", second.Priority)
	}

	third, _ := q.Dequeue()
	if third.Priority != PriorityLow {
		t.Errorf("expected low priority third, got %d", third.Priority)
	}
}

func TestFIFOWithinSamePriority(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	first := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-1", Priority: PriorityHigh, CreatedAt: time.Now().Add(-time.Hour)}
	second := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-2", Priority: PriorityHigh, CreatedAt: time.Now()}

	q.Enqueue(first)
	q.Enqueue(second)

	d1, _ := q.Dequeue()
	d2, _ := q.Dequeue()

	if d1.SpecID != "SPEC-1" {
		t.Errorf("expected SPEC-1 first (older), got %s", d1.SpecID)
	}
	if d2.SpecID != "SPEC-2" {
		t.Errorf("expected SPEC-2 second (newer), got %s", d2.SpecID)
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	q1 := NewHousekeepingQueue(tmpDir)
	task := HousekeepingTask{
		Type:        TaskTypePostResolution,
		SpecID:      "SPEC-999",
		RepoIssueID: 100,
		Priority:    PriorityMedium,
		Payload:     "test",
	}
	q1.Enqueue(task)
	q1.Save()

	q2 := NewHousekeepingQueue(tmpDir)
	if err := q2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if q2.Len() != 1 {
		t.Errorf("expected 1 task after reload, got %d", q2.Len())
	}

	task2, _ := q2.Dequeue()
	if task2.SpecID != "SPEC-999" {
		t.Errorf("expected SPEC-999, got %s", task2.SpecID)
	}
	if task2.RepoIssueID != 100 {
		t.Errorf("expected 100, got %d", task2.RepoIssueID)
	}
}

func TestUpdateTask(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	task := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-1", Priority: PriorityHigh}
	q.Enqueue(task)

	all := q.GetAll()
	if len(all) != 1 {
		t.Fatal("expected 1 task")
	}
	taskID := all[0].ID

	if err := q.UpdateTask(taskID, func(t *HousekeepingTask) {
		t.Attempts++
		t.LastError = "test error"
	}); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	updated, _ := q.Dequeue()
	if updated.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", updated.Attempts)
	}
	if updated.LastError != "test error" {
		t.Errorf("expected LastError='test error', got %s", updated.LastError)
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	err := q.UpdateTask("nonexistent", func(t *HousekeepingTask) {})
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestProcessSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	executed := make(chan string, 1)
	registry := map[TaskType]IssueAction{
		TaskTypeCloseIssue: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			executed <- task.SpecID
			return nil
		}},
	}

	task := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-X", Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	select {
	case specID := <-executed:
		if specID != "SPEC-X" {
			t.Errorf("expected SPEC-X, got %s", specID)
		}
	case <-time.After(time.Second):
		t.Fatal("action not executed")
	}

	if q.Len() != 0 {
		t.Errorf("expected empty queue after successful process, got %d", q.Len())
	}
}

func TestProcessRequeueOnError(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	attempts := 0
	registry := map[TaskType]IssueAction{
		TaskTypeCloseIssue: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			attempts++
			if attempts < 2 {
				return errors.New("transient error")
			}
			return nil
		}},
	}

	task := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-Y", Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue after eventual success, got %d", q.Len())
	}
}

func TestProcessMissingHandler(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	registry := map[TaskType]IssueAction{}

	task := HousekeepingTask{Type: TaskTypeUpdateMilestone, SpecID: "SPEC-Z", Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	all := q.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 task preserved after MaxAttempts, got %d", len(all))
	}
	if all[0].Status != StatusFailed {
		t.Errorf("expected task status FAILED, got %s", all[0].Status)
	}
	if all[0].Attempts != MaxAttempts {
		t.Errorf("expected Attempts=%d, got %d", MaxAttempts, all[0].Attempts)
	}
	if all[0].LastError == "" {
		t.Error("expected LastError to be set")
	}
}

func TestProcessMaxAttemptsPreservesFailedTask(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	callCount := 0
	registry := map[TaskType]IssueAction{
		TaskTypeCloseIssue: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			callCount++
			return errors.New("persistent error")
		}},
	}

	task := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-FAIL", Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if callCount != MaxAttempts {
		t.Errorf("expected %d execution attempts, got %d", MaxAttempts, callCount)
	}

	// Verify the task is still in the queue with FAILED status
	all := q.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 failed task preserved, got %d", len(all))
	}
	if all[0].Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", all[0].Status)
	}
	if all[0].Attempts != MaxAttempts {
		t.Errorf("expected Attempts=%d, got %d", MaxAttempts, all[0].Attempts)
	}
	if all[0].LastError != "persistent error" {
		t.Errorf("expected LastError='persistent error', got %s", all[0].LastError)
	}

	// Subsequent Dequeue should skip FAILED task
	_, ok := q.Dequeue()
	if ok {
		t.Error("expected Dequeue to return false when only FAILED tasks remain")
	}

	// Process on already-failed task should be a no-op (not retry)
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("second Process failed: %v", err)
	}
	if callCount != MaxAttempts {
		t.Errorf("expected no additional executions after FAILED, got %d", callCount)
	}
}

// ResolutionCommentService tests

func TestResolutionCommentService_Execute_Success(t *testing.T) {
	var (
		gotIssueNumber int
		gotBody        string
	)
	commenter := &mockCommenter{
		PostCommentFn: func(issueNumber int, body string) error {
			gotIssueNumber = issueNumber
			gotBody = body
			return nil
		},
	}

	service := NewResolutionCommentService(commenter)
	payload := ResolutionPayload{
		SpecID:  "SPEC-42",
		Title:   "Fix widget rendering",
		Version: "0.9.0",
	}
	payloadRaw, _ := json.Marshal(payload)
	task := HousekeepingTask{
		RepoIssueID: 101,
		Payload:     string(payloadRaw),
	}

	ctx := context.Background()
	if err := service.Execute(ctx, task); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if gotIssueNumber != 101 {
		t.Errorf("expected issue 101, got %d", gotIssueNumber)
	}
	if gotBody == "" {
		t.Fatal("expected non-empty comment body")
	}
	if !contains(gotBody, "SPEC-42") {
		t.Errorf("expected body to contain SPEC-42, got:\n%s", gotBody)
	}
	if !contains(gotBody, "Fix widget rendering") {
		t.Errorf("expected body to contain 'Fix widget rendering', got:\n%s", gotBody)
	}
	if !contains(gotBody, "0.9.0") {
		t.Errorf("expected body to contain 0.9.0, got:\n%s", gotBody)
	}
}

func TestResolutionCommentService_Execute_InvalidPayload(t *testing.T) {
	commenter := &mockCommenter{
		PostCommentFn: func(issueNumber int, body string) error {
			return nil
		},
	}

	service := NewResolutionCommentService(commenter)
	task := HousekeepingTask{
		RepoIssueID: 101,
		Payload:     "not-valid-json",
	}

	ctx := context.Background()
	err := service.Execute(ctx, task)
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestResolutionCommentService_Execute_CommenterError(t *testing.T) {
	commenter := &mockCommenter{
		PostCommentFn: func(issueNumber int, body string) error {
			return errors.New("network error")
		},
	}

	service := NewResolutionCommentService(commenter)
	payload := ResolutionPayload{SpecID: "SPEC-1", Title: "t", Version: "1.0"}
	payloadRaw, _ := json.Marshal(payload)
	task := HousekeepingTask{
		RepoIssueID: 101,
		Payload:     string(payloadRaw),
	}

	ctx := context.Background()
	err := service.Execute(ctx, task)
	if err == nil {
		t.Fatal("expected error when commenter fails")
	}
}

// IssueCloserService tests

func TestIssueCloserService_Execute_Success(t *testing.T) {
	var gotIssueNumber int
	closer := &mockIssueCloser{
		CloseIssueByNumberFn: func(issueNumber int) error {
			gotIssueNumber = issueNumber
			return nil
		},
	}

	service := NewIssueCloserService(closer)
	task := HousekeepingTask{
		RepoIssueID: 202,
		SpecID:      "SPEC-99",
	}

	ctx := context.Background()
	if err := service.Execute(ctx, task); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if gotIssueNumber != 202 {
		t.Errorf("expected issue 202, got %d", gotIssueNumber)
	}
}

func TestIssueCloserService_Execute_CloserError(t *testing.T) {
	closer := &mockIssueCloser{
		CloseIssueByNumberFn: func(issueNumber int) error {
			return errors.New("close failed")
		},
	}

	service := NewIssueCloserService(closer)
	task := HousekeepingTask{RepoIssueID: 202}

	ctx := context.Background()
	err := service.Execute(ctx, task)
	if err == nil {
		t.Fatal("expected error when closer fails")
	}
}

func TestProcessDropsOnSpecNotFoundOnDisk(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	// SYNC_METADATA for a spec that was archived/deleted returns
	// "spec <id> not found on disk". That is a permanent condition and the
	// task must be dropped on the first attempt, not retried forever.
	callCount := 0
	registry := map[TaskType]IssueAction{
		TaskTypeSyncMetadata: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			callCount++
			return fmt.Errorf("spec %s not found on disk", task.SpecID)
		}},
	}

	task := HousekeepingTask{Type: TaskTypeSyncMetadata, SpecID: "SPEC-GONE", Priority: PriorityMedium}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 attempt (dropped on missing spec), got %d", callCount)
	}
	if q.Len() != 0 {
		t.Errorf("expected empty queue after dropping missing-spec task, got %d", q.Len())
	}
}

func TestProcessDropsTaskOnResourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	// Registry with an action that returns "resource not found" error
	notFoundErr := errors.New("close on issue #42: resource not found")
	callCount := 0
	registry := map[TaskType]IssueAction{
		TaskTypeCloseIssue: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			callCount++
			return notFoundErr
		}},
	}

	task := HousekeepingTask{Type: TaskTypeCloseIssue, SpecID: "SPEC-RNF", RepoIssueID: 42, Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Task should be dropped (not retried) on first 404
	if callCount != 1 {
		t.Errorf("expected 1 execution call (dropped on 404), got %d", callCount)
	}

	// Queue should be empty
	if q.Len() != 0 {
		t.Errorf("expected empty queue after 404 drop, got %d tasks", q.Len())
	}
}

func TestProcessDropsOnResourceNotFound_RetriesOtherErrors(t *testing.T) {
	tmpDir := t.TempDir()
	q := NewHousekeepingQueue(tmpDir)

	attempts := 0
	registry := map[TaskType]IssueAction{
		TaskTypePostResolution: &mockAction{fn: func(ctx context.Context, task HousekeepingTask) error {
			attempts++
			if attempts == 1 {
				return errors.New("resource not found goes first")
			}
			return errors.New("transient network error")
		}},
	}

	task := HousekeepingTask{Type: TaskTypePostResolution, SpecID: "SPEC-RNF2", RepoIssueID: 99, Priority: PriorityHigh}
	q.Enqueue(task)

	ctx := context.Background()
	if err := q.Process(ctx, registry); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// First attempt returns "resource not found" which is DROPPED (not retried).
	// So attempts == 1 and the queue should be empty.
	if attempts != 1 {
		t.Errorf("expected 1 attempt (dropped on 404), got %d", attempts)
	}

	// Queue should be empty — the dropped task is gone
	if q.Len() != 0 {
		t.Errorf("expected empty queue after 404 drop, got %d tasks", q.Len())
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	commenter := &mockCommenter{
		PostCommentFn: func(issueNumber int, body string) error { return nil },
	}
	closer := &mockIssueCloser{
		CloseIssueByNumberFn: func(issueNumber int) error { return nil },
	}

	reg := NewDefaultRegistry(commenter, closer)

	if _, ok := reg[TaskTypePostResolution]; !ok {
		t.Error("expected TaskTypePostResolution in registry")
	}
	if _, ok := reg[TaskTypeCloseIssue]; !ok {
		t.Error("expected TaskTypeCloseIssue in registry")
	}
	if len(reg) != 2 {
		t.Errorf("expected 2 entries, got %d", len(reg))
	}
}

// Helper

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockAction struct {
	fn func(ctx context.Context, task HousekeepingTask) error
}

func (m *mockAction) Execute(ctx context.Context, task HousekeepingTask) error {
	return m.fn(ctx, task)
}
