---
spec_id: "SPEC-1783150591"
status: in-progress
repo_issue: ""
type: bug
root_cause: "TestTracker struct defines mu sync.RWMutex but it was never used by any code path. The render loop (renderTestList) and the per-test-timeout checker (GetTimeoutTests) iterated over tracker.ActiveTests/Completed without any lock. Meanwhile UpdateMetrics wrote to those same maps while holding TestTrackerService.mu — a different lock entirely. Since s.mu and tracker.mu are independent, the reader and writer could access the maps concurrently, causing fatal 'concurrent map iteration and map write' panics. My initial fix only added tracker.mu to the readers but left the writer unlocked, so the race remained."}
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
- `engine/testtracker.go` UpdateMetrics: release s.mu after getting/creating tracker, then acquire tracker.mu.Lock() for all map mutations.
- `engine/testtracker.go` ResetTrackers: collect tracker pointers under s.mu, then lock each tracker.mu individually during reset.
- `engine/testtracker.go` GetMetrics: release s.mu after getting tracker pointer, then acquire tracker.mu.RLock() before iterating maps.

## Acceptance Criteria
- [ ] No concurrent map iteration/write panics during `ff` test runs.
- [ ] All existing tests continue to pass.

## Verification
