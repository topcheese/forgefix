---
spec_id: "SPEC-1783113220"
status: ship
repo_issue: 441
type: feature
resolution: "Full spec-based ship workflow implemented: spec gate validation, git push, housekeeping task enqueue, remote issue close"
---
# Ship Flow

## Objective
Implement the end-to-end ship workflow for spec-based development: validate ship-ready specs, push to remote, enqueue housekeeping tasks to close issues and update spec statuses.

## Requirements
- Validate all specs are at `ship` status before proceeding
- Block on specs with `backlog`, `in-progress`, or `review` status
- Push committed changes to remote after validation
- Enqueue POST_RESOLUTION, CLOSE_ISSUE, and SYNC_METADATA tasks for shipped specs
- Support both spec-based and test-audit-based ship paths

## Implementation
- `checkShipGateSpecStatuses` scans specs/ dir for all `ship`-status specs
- `ShipReconciliation` branches on spec-based vs audit-log-based path
- Spec-based path pushes and enqueues housekeeping tasks with ResolutionPayload
- Housekeeping queue processes tasks asynchronously via DrainHousekeepingQueue

## Acceptance Criteria
1. Ship aborts with clear error if any spec is not at `ship` status
2. Git push succeeds to configured remote
3. Housekeeping tasks are persisted to tasks.json
4. Shipped specs get resolution comments and closed issues

## Verification
Full ship cycle tested: 4 specs shipped, remote push succeeded, tasks enqueued, all 314 tests pass.
