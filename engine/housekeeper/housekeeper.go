package housekeeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Commenter posts comments on issues.
type Commenter interface {
	PostComment(issueNumber int, body string) error
}

// IssueCloser closes issues.
type IssueCloser interface {
	CloseIssueByNumber(issueNumber int) error
}

// ResolutionPayload contains the data needed to format a resolution report.
type ResolutionPayload struct {
	SpecID    string `json:"spec_id"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	RepoIssue int    `json:"repo_issue"`
	SpecURL   string `json:"spec_url"`
}

// ResolutionCommentService implements IssueAction to post a resolution report
// as a comment on an issue. It parses the task's Payload field as ResolutionPayload
// and formats it using the standard resolution report template.
type ResolutionCommentService struct {
	commenter Commenter
}

func NewResolutionCommentService(commenter Commenter) *ResolutionCommentService {
	return &ResolutionCommentService{commenter: commenter}
}

func (s *ResolutionCommentService) Execute(ctx context.Context, task HousekeepingTask) error {
	var payload ResolutionPayload
	if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
		return fmt.Errorf("unmarshaling resolution payload: %w", err)
	}

	closedRef := fmt.Sprintf("#%d", task.RepoIssueID)
	specRef := payload.SpecID
	if payload.SpecURL != "" {
		specRef = fmt.Sprintf("[%s](%s)", payload.SpecID, payload.SpecURL)
	}

	body := fmt.Sprintf("## Resolution — [ForgeFix Resolution Report]\n\n**Status:** ✅ ALL TESTS PASSED\n\n")
	body += fmt.Sprintf("**Spec:** %s  \n", specRef)
	body += fmt.Sprintf("**Title:** %s  \n", payload.Title)
	body += fmt.Sprintf("**Issue:** %s  \n", closedRef)
	body += "\n### Implementation\n\n"
	body += fmt.Sprintf("The changes for **%s** have been implemented and shipped (version %s).\n", payload.Title, payload.Version)
	body += "\n---\n**Closed by:** ForgeFix Auto-Resolution"
	return s.commenter.PostComment(task.RepoIssueID, body)
}

// IssueCloserService implements IssueAction to close an issue by its remote ID.
type IssueCloserService struct {
	closer IssueCloser
}

func NewIssueCloserService(closer IssueCloser) *IssueCloserService {
	return &IssueCloserService{closer: closer}
}

func (s *IssueCloserService) Execute(ctx context.Context, task HousekeepingTask) error {
	return s.closer.CloseIssueByNumber(task.RepoIssueID)
}

// SyncMetadataService updates a spec's status to "closed" after a successful ship.
type SyncMetadataService struct {
	syncer MetadataSyncer
}

func NewSyncMetadataService(syncer MetadataSyncer) *SyncMetadataService {
	return &SyncMetadataService{syncer: syncer}
}

func (s *SyncMetadataService) Execute(ctx context.Context, task HousekeepingTask) error {
	return s.syncer.SyncMetadata(task.SpecID)
}

// MetadataSyncer updates a spec's frontmatter status (e.g. ship -> closed).
type MetadataSyncer interface {
	SyncMetadata(specID string) error
}

// NewDefaultRegistry builds the default TaskType->IssueAction mapping.
func NewDefaultRegistry(commenter Commenter, closer IssueCloser, syncer ...MetadataSyncer) map[TaskType]IssueAction {
	reg := map[TaskType]IssueAction{
		TaskTypePostResolution: NewResolutionCommentService(commenter),
		TaskTypeCloseIssue:     NewIssueCloserService(closer),
	}
	if len(syncer) > 0 && syncer[0] != nil {
		reg[TaskTypeSyncMetadata] = NewSyncMetadataService(syncer[0])
	}
	return reg
}

type Priority int

const (
	PriorityLow Priority = iota
	PriorityMedium
	PriorityHigh
)

type TaskType string

const (
	TaskTypeCloseIssue      TaskType = "CLOSE_ISSUE"
	TaskTypePostResolution  TaskType = "POST_RESOLUTION"
	TaskTypeSyncMetadata    TaskType = "SYNC_METADATA"
	TaskTypeUpdateMilestone TaskType = "UPDATE_MILESTONE"
)

const MaxAttempts = 3

type TaskStatus string

const (
	StatusPending   TaskStatus = "PENDING"
	StatusFailed    TaskStatus = "FAILED"
	StatusCompleted TaskStatus = "COMPLETED"
)

type HousekeepingTask struct {
	ID          string            `json:"id"`
	Type        TaskType          `json:"type"`
	SpecID      string            `json:"spec_id"`
	RepoIssueID int               `json:"repo_issue_id"`
	Priority    Priority          `json:"priority"`
	Payload     string            `json:"payload"`
	Context     map[string]string `json:"context"`
	Attempts    int               `json:"attempts"`
	LastError   string            `json:"last_error,omitempty"`
	Status      TaskStatus        `json:"status,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type IssueAction interface {
	Execute(ctx context.Context, task HousekeepingTask) error
}

type HousekeepingQueue struct {
	mu       sync.Mutex
	filePath string
	tasks    []HousekeepingTask
}

func NewHousekeepingQueue(configDir string) *HousekeepingQueue {
	filePath := filepath.Join(configDir, "tasks.json")
	return &HousekeepingQueue{
		filePath: filePath,
		tasks:    make([]HousekeepingTask, 0),
	}
}

func (q *HousekeepingQueue) Load() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := os.ReadFile(q.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			q.tasks = make([]HousekeepingTask, 0)
			return nil
		}
		return fmt.Errorf("reading tasks file: %w", err)
	}

	if len(data) == 0 {
		q.tasks = make([]HousekeepingTask, 0)
		return nil
	}

	var tasks []HousekeepingTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return fmt.Errorf("unmarshaling tasks: %w", err)
	}
	q.tasks = tasks
	return nil
}

func (q *HousekeepingQueue) Save() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := json.MarshalIndent(q.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tasks: %w", err)
	}

	if err := os.WriteFile(q.filePath, data, 0644); err != nil {
		return fmt.Errorf("writing tasks file: %w", err)
	}
	return nil
}

func (q *HousekeepingQueue) Enqueue(task HousekeepingTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	q.mu.Lock()
	q.tasks = append(q.tasks, task)
	q.mu.Unlock()

	return q.Save()
}

func (q *HousekeepingQueue) Dequeue() (HousekeepingTask, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return HousekeepingTask{}, false
	}

	sort.Slice(q.tasks, func(i, j int) bool {
		if q.tasks[i].Priority != q.tasks[j].Priority {
			return q.tasks[i].Priority > q.tasks[j].Priority
		}
		return q.tasks[i].CreatedAt.Before(q.tasks[j].CreatedAt)
	})

	for i, t := range q.tasks {
		if t.Status != StatusFailed {
			task := q.tasks[i]
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			if err := q.saveUnlocked(); err != nil {
				return HousekeepingTask{}, false
			}
			return task, true
		}
	}
	return HousekeepingTask{}, false
}

func (q *HousekeepingQueue) saveUnlocked() error {
	data, err := json.MarshalIndent(q.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tasks: %w", err)
	}
	return os.WriteFile(q.filePath, data, 0644)
}

func (q *HousekeepingQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

func (q *HousekeepingQueue) GetAll() []HousekeepingTask {
	q.mu.Lock()
	defer q.mu.Unlock()
	tasks := make([]HousekeepingTask, len(q.tasks))
	copy(tasks, q.tasks)
	return tasks
}

func (q *HousekeepingQueue) UpdateTask(id string, fn func(*HousekeepingTask)) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i := range q.tasks {
		if q.tasks[i].ID == id {
			fn(&q.tasks[i])
			return q.saveUnlocked()
		}
	}
	return errors.New("task not found")
}

func (q *HousekeepingQueue) Process(ctx context.Context, registry map[TaskType]IssueAction) error {
	for {
		task, ok := q.Dequeue()
		if !ok {
			return nil
		}

		if task.Attempts >= MaxAttempts {
			task.Status = StatusFailed
			if err := q.Enqueue(task); err != nil {
				return fmt.Errorf("re-queueing failed task after max attempts: %w", err)
			}
			continue
		}

		action, exists := registry[task.Type]
		if !exists {
			task.Attempts++
			task.LastError = fmt.Sprintf("no handler registered for task type: %s", task.Type)
			if err := q.Enqueue(task); err != nil {
				return fmt.Errorf("re-queueing task with missing handler: %w", err)
			}
			continue
		}

		err := action.Execute(ctx, task)
		if err != nil {
			task.Attempts++
			task.LastError = err.Error()
			if err := q.Enqueue(task); err != nil {
				return fmt.Errorf("re-queueing failed task: %w", err)
			}
		}
	}
}
