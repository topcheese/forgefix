---
spec_id: "SPEC-1783975469"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: ["14ae062", "64037cd"]
---
When ff --ai runs and tests fail (DETONATED status), the system should automatically create a local spec file for the failure so it enters the spec lifecycle. Currently handleDetonationIssues (execute.go) only queues SyncOpCreateIssue which calls processCreateIssue -> coord.EnsureIssue, creating a REMOTE issue only — no local specs/ file is created. The auto-spec-on-failure feature appears lost. This spec investigates what happened and restores the code path that creates a local spec from a failed test.
