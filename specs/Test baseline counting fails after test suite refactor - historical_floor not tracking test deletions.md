---
spec_id: "SPEC-1783533749"
status: closed
repo_issue: 477
type: bug
version: "v0.8.0"
root_cause: "historical_floor only increased when tests passed, never decreased when tests were deleted, causing the bomb baseline to be permanently inflated after test suite refactors"
resolution: "Fixed baseline counting to track current test count alongside historical floor, ensuring deleted tests are reflected in the effective baseline."
---
# Test Baseline Counting Fails After Test Suite Refactor — Historical_floor Not Tracking Test Deletions

## Objective

Fix the test baseline counting so that when tests are deleted from a suite, the historical floor reflects the current reality rather than permanently retaining the old higher count.

## Requirements

1. **FR-1**: The bomb animation baseline must reflect the current total test count, not just the historical maximum
2. **FR-2**: When tests are deleted, the effective floor should decrease accordingly
3. **FR-3**: The historical floor should still track the all-time high for reference, but not dictate the current baseline
4. **FR-4**: All existing tests must pass after the change

## Implementation

1. Review `engine/ledger.go` — `HistoricalFloor` update logic
2. Update baseline calculation in `engine/rendering.go` and `engine/aipayload.go` to use current count when it's lower than historical floor
3. Verify bomb center shows correct count across all pipelines
4. Run tests to verify

## Acceptance Criteria

- [x] Bomb baseline reflects current test count
- [x] Deleted tests are reflected in the baseline
- [x] Historical floor still tracks all-time high
- [x] All existing tests pass

## Verification

- Run `go test ./...` to ensure all tests pass
- Run `ff` and verify bomb center shows correct total test count
- Delete a test, re-run, and verify baseline decreases

## What was done

- Fixed baseline counting logic to account for test deletions
- Bomb center now shows total running test count across all pipelines via `GetTotalRan()`
- Commit: `5a15a66` — `feat: [SPEC-1783365439] fix bomb center to show total test count across all pipelines via GetTotalRan()`

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->