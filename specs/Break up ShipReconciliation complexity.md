---
spec_id: "SPEC-1783223922"
status: in-progress
repo_issue: 468
type: refactor
version: "v0.8.0"
root_cause: ""
resolution: ""
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

- [ ] ShipController extracted with Run() method
- [ ] VersionManager extracted with read/write/prompt/increment/isValid
- [ ] execute.go no longer imports housekeeper types directly
- [ ] All tests pass at baseline
- [ ] No behavioral change in ship flow

## Verification

- go build ./...
- go test ./... -count=1
- go test ./... -v -run TestShipReconciliation (if tests exist)