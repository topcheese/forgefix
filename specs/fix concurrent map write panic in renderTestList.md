---
spec_id: "SPEC-1783150591"
status: in-progress
repo_issue: ""
type: bug
root_cause: "TestTracker struct defines mu sync.RWMutex but it was never used. The render loop (renderTestList) iterated over tracker.ActiveTests and tracker.Completed without any lock, while the test parser goroutine (UpdateMetrics) wrote to those same maps while holding TestTrackerService.mu. This caused fatal 'concurrent map iteration and map write' panics during test execution."
resolution: ""
---
# Fix Concurrent Map Write Panic In RenderTestList

## Objective
Eliminate the data race between the render loop and test parser that causes fatal panics during test execution.

## Requirements
1. All reads from TestTracker.ActiveTests, Completed, CompletedIDs, and History must hold tracker.mu.RLock().
2. All writes to TestTracker internal maps must hold tracker.mu.Lock().
3. No deadlock can be introduced by nested locking with DashboardFacade.mu.

## Implementation
- `engine/rendering.go` renderTestList: acquire tracker.mu.RLock() before iterating tracker.Completed and tracker.ActiveTests.
- `engine/dashboard.go` GetTimeoutTests: acquire tracker.mu.RLock() before iterating tracker.ActiveTests.
- `engine/dashboard.go` drainOrphanedTests: acquire tracker.mu.Lock() before reading/writing tracker maps, unlock after mutations complete.

## Acceptance Criteria
- [ ] No concurrent map iteration/write panics during `ff` test runs.
- [ ] All existing tests continue to pass.

## Verification
