---
spec_id: "SPEC-1783223922"
status: ship
repo_issue: 468
type: refactor
version: "0.8.3"
root_cause: "ShipReconciliation() in execute.go was a ~250-line function with ~36 branches, mixing git validation, spec gate checks, version prompting, housekeeping queue enqueuing, and housekeeper type exposure. ship_controller.go and version_manager.go were created as extraction targets but ShipReconciliation() was never converted to use them — both implementations existed independently."
resolution: "ShipReconciliation() in execute.go refactored to a thin wrapper delegating to NewShipController(config, configDir, aiMode).Run(). housekeeper import removed from execute.go (only used in ShipReconciliation, now handled by ShipController internally). All ~250 lines of branching logic consolidated into ShipController with injected collaborators (Config, VersionManager, housekeeping queue). Tests pass without behavioral change."
---
# Break up ShipReconciliation complexity

## Objective

Refactor ShipReconciliation() in engine/execute.go — a 250-line function with ~36 branches — into a ShipController thin facade with injected collaborators and a VersionManager for version read/write/prompt logic. Stop housekeeper types leaking into execute.go.

## Requirements

- Extract VersionManager struct (readProjectVersion, writeProjectVersion, promptForVersion, incrementPatchVersion, isValidSemver)
- Extract ShipController struct orchestrating ship flow with injected config
- ShipReconciliation in execute.go becomes a thin wrapper delegating to ShipController
- No direct housekeeper imports in execute.go; housekeeping queue logic lives in ShipController
- All existing tests continue to pass (no behavioral change)
- ADR compatibility: no public API changes

## Implementation

Create engine/ship_controller.go:
- ShipController struct with Config, configDir, aiMode, VersionManager fields
- Run() method orchestrating the full ship flow

Create engine/version_manager.go:
- VersionManager struct with configDir
- CurrentVersion(), PromptForVersion(), WriteVersion() methods

Update engine/execute.go:
- ShipReconciliation becomes a thin wrapper: new ShipController(...).Run()

## Acceptance Criteria

- [x] ShipController extracted with Run() method
- [x] VersionManager extracted with read/write/prompt/increment/isValid
- [x] execute.go no longer imports housekeeper types directly
- [x] All tests pass at baseline
- [x] No behavioral change in ship flow

## Verification

- [x] go build ./...
- [x] go test ./... -count=1 — all 340+ tests pass
- ShipReconciliation now delegates to NewShipController(config, configDir, aiMode).Run()
- housekeeper import removed from execute.go (only used in ShipReconciliation)
- ship_controller.go enqueueHousekeeping uses shipVersion (consistent) instead of old spec.Version