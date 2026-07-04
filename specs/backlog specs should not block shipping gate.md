---
spec_id: "SPEC-1783138695"
status: ship
repo_issue: ""
type: bug
root_cause: "checkShipGateSpecStatuses included 'backlog' in the blocking switch case, treating planned future work as a ship blocker"
resolution: "Removed 'backlog' from the blocking status list in engine/execute.go:584. Backlog specs represent deferred future work and must never block shipping."
---
# Backlog Specs Should Not Block Shipping Gate

## Objective

The `ff ship` Strict Shipping Gate currently treats `backlog`-status specs as blockers, preventing shipping even when the backlog items are planned future work (e.g., v0.9.0 features deferred to a later release). Backlog specs should never block shipping — only actively-being-worked-on (`in-progress`) or under-review (`review`) specs should gate the ship.

## Requirements

1. `backlog`-status specs must not block `ff ship`
2. `draft`-status specs must not block `ff ship` (already works)
3. `in-progress` and `review` specs must continue to block
4. All existing ship gate tests must pass with updated expectations

## Implementation

- `engine/execute.go:584` — Removed `"backlog"` from blocking switch case; only `"in-progress"` and `"review"` now block
- `engine/ship_test.go` — Updated `TestCheckShipGateSpecStatuses_BacklogBlocks` → `BacklogPasses`; removed backlog from `TestCheckShipGateSpecStatuses_MultipleBlocking`

## Acceptance Criteria

- [x] `ff ship` passes when the only non-ship specs are `backlog` or `draft`
- [x] `ff ship` still blocks when `in-progress` or `review` specs exist
- [x] All tests pass

## Verification

```bash
go test ./engine/ -run TestCheckShipGate -v
# 7 tests pass
```
