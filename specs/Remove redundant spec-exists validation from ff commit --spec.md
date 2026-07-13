---
spec_id: "SPEC-1783942277"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
linked_commits: ["b281038", "1d5771b", "098bfd3"]
---
The spec-exists check in runCommit (cmd_commit.go:93-94) validates that a spec is present in loadActiveSpecs() when --spec is explicitly provided. This is redundant — the agent already knows the spec exists. Remove the check so explicitly-provided spec IDs are trusted.
