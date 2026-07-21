---
spec_id: "SPEC-1784672632"
status: review
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: ["5e6c3cf", "5429d22"]
---
# Fix finalizeCommitAfterAmend spec path and duplicate linked_commits

Two bugs in finalizeCommitAfterAmend (cmd_commit.go):

1. **Wrong spec directory path.** Line 312 constructs `specDir` from `configDir` (.ff/) instead of the repo root, so `findSpecFileByID` always fails and linked_commits is never updated after amend.

2. **Duplicate linked_commits on re-amend.** When the last linked_commit already equals the final hash, the code skips replacement but falls through to append, creating a duplicate entry. Fix: check if finalHash exists anywhere in the list before modifying.
