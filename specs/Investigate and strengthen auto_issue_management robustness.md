---
spec_id: "SPEC-1783983677"
status: draft
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: []
---
The auto_issue_management config flag is supposed to gate automatic remote issue creation/closure in --ai mode. Currently the ONLY enforcement is in NewCoordinatorFromConfig (issue_coordinator.go:70): it returns nil when aiMode && !cfg.AutoIssueManagement, which makes dashboard.Coord nil and skips handleDetonationIssues. Concern: with auto_issue_management:false, the Agent was still able to use --ai or bypass the gate and fake like it was managing issues. The spec must investigate every code path that creates/closes/syncs remote issues (cmd_spec.go:191/242, cmd_commit.go:658, cmd_backlog.go:169, sync.go promoteReviewSpecs, processCreateIssue) and verify each respects the flag. Then strengthen the gate so it captures ALL use cases and cannot be bypassed — including direct ff sync/ff ship invocations, background sync spawns, and any path that constructs an IssueCoordinator outside NewCoordinatorFromConfig.
