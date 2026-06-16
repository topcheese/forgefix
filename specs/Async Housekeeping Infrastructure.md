---
spec_id: "SPEC-1781317046"
status: "closed"
type: "type/feature"
version: "version/0.8.0"
repo_issue: ""
---

# TITLE: engine/housekeeper: implement asynchronous task orchestration

## Description
Refactor the current monolithic `ff sync` loop into a two-phase execution model: **Discovery** (identifying needs) and **Housekeeping** (asynchronous state mutation). This improves sync performance by offloading non-critical API operations to a priority-managed background queue.

## Proposed Components

### 1. Task Definition
Implement the `HousekeepingTask` struct to serve as the unit of work for all background maintenance.

```go
type Priority int

const (
    PriorityLow    Priority = iota
    PriorityMedium
    PriorityHigh
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
    CreatedAt   time.Time         `json:"created_at"`
}
```

### 2. Service-Oriented Execution
Implement an `IssueAction` interface for decoupled task execution.

```go
type IssueAction interface {
    Execute(ctx context.Context, task HousekeepingTask) error
}
```

**Registry mapping:**
- `CLOSE_ISSUE` -> `IssueCloserService`
- `POST_RESOLUTION` -> `ResolutionCommentService`
- `SYNC_METADATA` -> `MetadataSyncService`
- `UPDATE_MILESTONE` -> `MilestoneUpdateService`

### 3. Workflow Strategy
- **Registration**: `SyncSpecs` identifies actions (e.g., "close issue X") and writes a `HousekeepingTask` to the local `tasks.json`.
- **Orchestration**: `ff commit`, `ff archive`, and `ff sync` trigger the `Housekeeper.Process()` method.
- **Execution**: The `Housekeeper` sorts tasks by `Priority`, executes them via registered `IssueAction` services using worker goroutines, and updates/clears the `tasks.json` file.
- **Resiliency**: Failed tasks retain their state (with an `Attempts` increment and `LastError` log) for retry during the next command trigger.

## Acceptance Criteria
- [ ] `housekeeper` package implemented with support for priority-based execution.
- [ ] `SyncSpecs` modified to enqueue resolution/closure tasks instead of direct API POST/PATCH.
- [ ] `ff commit` triggers a synchronous drain of the `HousekeepingQueue` before exit.
- [ ] Task persistence via `tasks.json` verified across process restarts.
- [ ] Zero loss of resolution audits; background tasks maintain full context metadata.
