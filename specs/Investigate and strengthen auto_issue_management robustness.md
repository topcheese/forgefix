---
spec_id: "SPEC-1783983677"
status: ship
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: "Two code paths in sync.go created IssueCoordinator via NewIssueCoordinator directly, bypassing the existing gate in NewCoordinatorFromConfig"
resolution: "Added auto_issue_management gate to RunBackgroundSync (sync.go) and DrainHousekeepingQueueFromConfig (sync.go). Both return early with local ledger reconciliation when aiMode && !AutoIssueManagement. DrainHousekeepingQueueFromConfig gained an aiMode parameter; callers in cmd_commit.go and ship_controller.go updated."
linked_commits: ["3b51389"]
---
The auto_issue_management config flag is supposed to gate automatic remote issue creation/closure in --ai mode. Currently the ONLY enforcement is in NewCoordinatorFromConfig (issue_coordinator.go:70): it returns nil when aiMode && !cfg.AutoIssueManagement, which makes dashboard.Coord nil and skips handleDetonationIssues. Concern: with auto_issue_management:false, the Agent was still able to use --ai or bypass the gate and fake like it was managing issues. The spec must investigate every code path that creates/closes/syncs remote issues (cmd_spec.go:191/242, cmd_commit.go:658, cmd_backlog.go:169, sync.go promoteReviewSpecs, processCreateIssue) and verify each respects the flag. Then strengthen the gate so it captures ALL use cases and cannot be bypassed — including direct ff sync/ff ship invocations, background sync spawns, and any path that constructs an IssueCoordinator outside NewCoordinatorFromConfig.

### Implementation

Investigated all code paths. Two bypasses found in sync.go:

1. **RunBackgroundSync** (sync.go:347) — used by `ff sync --ai` and all background sync spawns. Created coordinator via `NewIssueCoordinator(Owne, Repo, Token, BaseURL)` directly, skipping the gate.

2. **DrainHousekeepingQueueFromConfig** (sync.go:873) — used by housekeeping drains after `ff commit` and `ff ship`. Same bypass pattern: `NewIssueCoordinator(...)` direct.

**Fix**: Added `auto_issue_management` gate to both:
- `RunBackgroundSync`: returns early with ledger reconciliation when `aiMode && !cfg.AutoIssueManagement`
- `DrainHousekeepingQueueFromConfig`: new `aiMode` parameter, same early return
- Callers updated: `cmd_commit.go` passes `flags.AIMode`; `ship_controller.go` passes `sc.aiMode`

**Still properly gated** (no change needed):
- `cmd_spec.go:203/254` — check `loaded.Config.AutoIssueManagement` before `SpawnBackgroundSync`
- `cmd_commit.go:756` — same pattern for bug spec creation
- `cmd_backlog.go:169` — same pattern for backlog creation
- `NewCoordinatorFromConfig` (issue_coordinator.go:70) — the original gate

**Not issue-tracker paths** (no change needed):
- `promoteReviewSpecs` — only modifies local ledger status, doesn't touch remote
- `processCreateIssue` — only called from `handleDetonationIssues` which is already gated via nil coord
