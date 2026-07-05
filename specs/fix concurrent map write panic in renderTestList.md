---
spec_id: "SPEC-1783150591"
status: closed
repo_issue: 452
type: bug
root_cause: "Two independent bugs: (1) TestTracker.mu was defined but never used; UpdateMetrics locked TestTrackerService.mu while renderTestList locked nothing, causing concurrent map iteration/write panics. (2) DashboardFacade.GetTimeoutFired() returned f.TestCommandCompleted with a comment 'repurposed' — so whenever tests completed (TestCommandCompleted=true), GetTimeoutFired() also returned true. This caused the TUI to render the timeout section alongside the success section, producing the contradictory 'BOMB DEFUSED + TIMEOUT' output."
resolution: "Fixed concurrent map write panic by acquiring TestTracker locks (RLock for reads, Lock for writes) in renderTestList, GetTimeoutTests, drainOrphanedTests, UpdateMetrics, ResetTrackers, and GetMetrics. Fixed contradictory TUI output by adding dedicated TimeoutFired field to DashboardFacade. Added archive-file skip and title truncation in syncSingleSpec. See commits fb57910, 0c448d7, f5287a8, 2b4f5ca."
version: "0.8.1"
---
# Fix Concurrent Map Write Panic In RenderTestList

## Objective
Eliminate the data race between the render loop and test parser that causes fatal panics during test execution, and fix the contradictory timeout/success TUI output.

## Requirements
1. All reads from TestTracker.ActiveTests, Completed, CompletedIDs, and History must hold tracker.mu.RLock().
2. All writes to TestTracker internal maps must hold tracker.mu.Lock().
3. No deadlock can be introduced by nested locking with DashboardFacade.mu.
4. GetTimeoutFired() must use a dedicated TimeoutFired field, not TestCommandCompleted.

## Implementation
- `engine/rendering.go` renderTestList: acquire tracker.mu.RLock() before iterating tracker.Completed and tracker.ActiveTests.
- `engine/dashboard.go` GetTimeoutTests: acquire tracker.mu.RLock() before iterating tracker.ActiveTests.
- `engine/dashboard.go` drainOrphanedTests: acquire tracker.mu.Lock() before reading/writing tracker maps, unlock after mutations complete.
- `engine/testtracker.go` UpdateMetrics: release s.mu after getting/creating tracker, then acquire tracker.mu.Lock() for all map mutations.
- `engine/testtracker.go` ResetTrackers: collect tracker pointers under s.mu, then lock each tracker.mu individually during reset.
- `engine/testtracker.go` GetMetrics: release s.mu after getting tracker pointer, then acquire tracker.mu.RLock() before iterating maps.
- `engine/dashboard_facade.go`: Add dedicated TimeoutFired field; fix GetTimeoutFired() and SetTimeoutFired() to use it instead of TestCommandCompleted.
- `engine/sync.go` syncSingleSpec: Skip archive files (names starting with `archive_`) when scanning the specs directory.
- `engine/sync.go` syncSingleSpec: Truncate issue titles that exceed `maxTitleLength` after prefixing with `feat/spec:`.

## Acceptance Criteria
- [ ] No concurrent map iteration/write panics during `ff` test runs.
- [ ] No 'TIMEOUT + BOMB DEFUSED' contradictory output when tests complete successfully.
- [ ] All existing tests continue to pass.

## Verification