---
spec_id: "SPEC-1783645711"
status: closed
repo_issue: 490
type: feature
version: "0.9.0"
root_cause: "Housekeeping tasks for archived or deleted specs would retry forever, piling up zombie FAILED messages with no way to clear them"
resolution: "Implemented alongside SPEC-1783709991 (ba67e72). droppableError in housekeeper.go detects permanent failures (404, missing spec files) and drops tasks from the queue instead of retrying."
---
# Make Background Housekeeping Drop Missing Specs And Report Progress

## Objective

Housekeeping tasks for specs that have been archived, deleted, or otherwise
removed from disk should be dropped from the queue immediately rather than
retried to exhaustion. The queue should also report progress so users can see
which tasks succeeded, failed, or were dropped.

## Requirements

1. Detect permanent failures (404 from remote, missing spec file on disk) and
   drop the task from the queue without retrying.
2. Print clear status for each task: running, succeeded, dropped, or failed
   with retry count.
3. Max retry attempts should be bounded so a single failing task doesn't block
   the queue forever.

## Implementation

- `engine/housekeeper/housekeeper.go`: Added `droppableError()` which checks
  for "resource not found" and "not found on disk" error messages — these
  indicate the spec was archived or the remote issue was deleted.
- In the task execution loop, after each attempt, check `droppableError(err)`.
  If true, drop the task (skip retry) and print:
  `[HOUSEKEEPER] ✗ <type> spec <id> dropped (permanent failure, not retrying)`.
- Bounded retries via `MaxAttempts` constant.
- Progress is reported per-task with attempt counters.

## Acceptance Criteria

- Archived specs produce "dropped" messages instead of retrying forever.
- Missing remote issues (404) drop the task immediately.
- Live tasks still retry up to `MaxAttempts` times before giving up.

## Verification

- `go test ./engine/housekeeper -run TestHousekeeper -v` — covers droppable
  error paths.
- `go test ./... -count=1` — full suite green.
