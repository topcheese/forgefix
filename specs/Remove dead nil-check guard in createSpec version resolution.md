---
spec_id: "SPEC-1783975440"
status: review
repo_issue: ""
type: feature
version: "0.9.5"
root_cause: "createSpec() guarded NewVersionManager() with a nil check, but NewVersionManager always returns a non-nil *VersionManager — the nil branch was dead code that added unnecessary nesting noise"
resolution: "Removed the nil-check guard. The version resolution now calls NewVersionManager(configDir).CurrentVersion() directly, eliminating 3 lines of dead branching code without changing behavior"
linked_commits: ["7971f9d", "3fedae1"]
---
In engine/cmd_spec.go createSpec(), the version resolution used 'if vm := NewVersionManager(configDir); vm != nil' but NewVersionManager always returns a non-nil pointer, making the nil check dead code. Simplify to call CurrentVersion() directly.
