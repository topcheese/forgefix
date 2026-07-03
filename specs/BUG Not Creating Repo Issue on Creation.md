---
spec_id: "SPEC-1783079687"
status: ship
repo_issue: 440
type: bug
root_cause: "createNewBugSpec in main.go was missing QueueSyncSpec and SpawnBackgroundSync calls that createSpec already had"
resolution: "Added QueueSyncSpec + SpawnBackgroundSync to createNewBugSpec so bug specs trigger remote issue creation on sync"
---
# Bug Not Creating Repo Issue On Creation

## Objective
Bug specs created via the interactive prompt were not generating remote issues in Gitea, while feature specs created via `ff spec --ai` were working correctly. This caused bugs to be untracked in the issue tracker.

## Requirements
- Bug specs must auto-create a remote issue in Gitea on creation (same as feature specs)
- The sync queue must be populated and a background sync process spawned

## Implementation
Added `QueueSyncSpec` and `SpawnBackgroundSync` calls to `createNewBugSpec` in `main.go:1020-1032`, matching the existing pattern in `createSpec`.

## Acceptance Criteria
1. Creating a bug spec via the interactive prompt queues a sync operation
2. A background sync process is spawned immediately
3. A remote issue is created in Gitea with the bug spec title and body

## Verification
Confirmed via `ff sync` that bug specs now create remote issues with proper titles.
