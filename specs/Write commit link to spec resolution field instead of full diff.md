---
spec_id: "SPEC-1784703448"
status: draft
repo_issue: 557
type: refactor
version: "0.9.8"
root_cause: ""





linked_commits: ["84c4aa9", "3f38c4a", "4e17751", "8d9d2d5", "9d8cd85"]
resolution: fixed in 7bdcdd7
---
# Write Commit Link To Spec Resolution Field Instead Of Full Diff
## Objective

After each ff commit, write a lightweight commit link (e.g. "fixed in abc123") to the spec file's resolution frontmatter field and DB entry, instead of dumping the full git diff. The diff lives in git history via linked_commits; the resolution field provides a human-readable pointer.

## Root Cause

The previous implementation (SPEC-1784264619) removed git diff capture entirely from cmd_commit.go, leaving the resolution field empty after commits. The resolution field should contain a traceable link to the implementation commit as ground truth of what was changed.

## Requirements

1. After finalizeCommitAfterAmend, write "fixed in <commit_hash>" to the spec file's resolution frontmatter and DB SpecEntry.Resolution
2. Restore spec.Resolution in PostResolutionComment so the commit link appears in the remote issue resolution comment
3. No full diff capture — just the commit hash link
