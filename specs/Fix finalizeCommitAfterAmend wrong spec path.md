---
spec_id: "SPEC-1784672632"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---
# Fix Finalizecommitafteramend Wrong Spec Path
# Fix Finalizecommitafteramend Wrong Spec Path

finalizeCommitAfterAmend constructs the spec directory path from configDir (.ff/) instead of the repo root, so findSpecFileByID always fails. Line 312: specDir := filepath.Join(configDir, "specs") should use the working directory. This means linked_commits is never updated after amend, leaving stale hashes in spec files. Fix: pass the working directory (or derive it from configDir via findGitRootWalk) and use that for the specs path.
