---
spec_id: "SPEC-1784075091"
status: draft
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: []
---
Investigate where, how, and when the repo_issue value gets set in spec frontmatter. Some specs have it populated (e.g., repo_issue: 42) and others don't (repo_issue: ''). Trace the full lifecycle of repo_issue assignment across spec creation, sync, commit, and ship operations to identify the source of inconsistency.
