---
spec_id: "SPEC-1783162752"
status: closed
repo_issue: 464
type: feature
version: "0.8.1"
root_cause: ""
resolution: "Fixed shipping gate blocking on in-progress specs, audited and fixed error handling across all engine packages, fixed sync queue 404 loop"
---
# Fix Shipping Gate And Error Handling Across Forgefix

## Objective

Fix the shipping gate, error handling, and sync queue in ForgeFix.

## Requirements

- In-progress specs should not block shipping
- Error handling should follow Go best practices (%w over %v, errors.As over type assertions, recover on goroutines, single handling rule)
- Sync queue should not loop on 404 failures

## Implementation

- engine/execute.go: Removed in-progress from shipping gate blocker, fixed bare return err, added errors.As, fixed capitalized errors
- engine/sync.go: Moved lastErr after 404 check, added deduplication in EnqueueSyncOp
- engine/issue_coordinator.go: Fixed log-and-return, removed DEBUG prints, fixed json.Marshal/io.ReadAll/http.NewRequest error checks
- engine/config.go, runner.go, discovery.go, ledger.go, parser.go: %v->%w
- engine/cmd_git.go, runner.go: type assertions -> errors.As
- engine/watcher.go, execute.go, runner.go: added recover() to goroutines
- engine/ship_test.go: Updated tests for new behavior

## Acceptance Criteria

- ff ship passes with in-progress specs
- All tests pass
- go vet clean

## Verification

- Tests: go test ./engine/ -count=1 → PASS
- Lint: go vet ./engine/ → clean

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->