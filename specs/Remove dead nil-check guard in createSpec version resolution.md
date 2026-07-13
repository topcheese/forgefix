---
spec_id: "SPEC-1783975440"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: ""
resolution: ""
linked_commits: []
---
In engine/cmd_spec.go createSpec(), the version resolution used 'if vm := NewVersionManager(configDir); vm != nil' but NewVersionManager always returns a non-nil pointer, making the nil check dead code. Simplify to call CurrentVersion() directly.
