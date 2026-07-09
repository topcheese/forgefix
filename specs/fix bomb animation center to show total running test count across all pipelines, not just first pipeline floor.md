---
spec_id: "SPEC-1783365439"
status: ship
repo_issue: 474
type: bug
version: "0.9.3"
root_cause: "floorString() returned LedgerFloor of first pipeline only — bomb ring showed 0 when first pipeline had no floor configured"
resolution: "Changed floorString to return d.GetTotalRan() — total test count across all pipelines"
---
# Fix Bomb Animation Center To Show Total Running Test Count Across All Pipelines, Not Just First Pipeline Floor

## Objective

Make the bomb animation center display the total number of tests running across all pipelines, not just the first pipeline's ledger floor.

## Requirements

- Bomb ring center shows total test count across all active pipelines
- Bomb defused art shows total test count across all pipelines
- Tests updated to verify the new behavior

## Implementation

Changed `floorString()` in `engine/rendering.go` to return `d.GetTotalRan()` instead of computing a single pipeline's ledger floor. This makes the bomb ring center (`│ X│`) display the cumulative test count across all pipelines.

## Acceptance Criteria

- [x] Bomb ring shows total running test count instead of pipeline floor
- [x] Bomb defused art shows total count
- [x] All 14 render tests pass

## Verification

- `go build ./...` — clean
- `go test ./engine/ -run TestDashboardRenderer -v` — 14/14 pass
- `go test ./...` — all 4 packages pass

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->