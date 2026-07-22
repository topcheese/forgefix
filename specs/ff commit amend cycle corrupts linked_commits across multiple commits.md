---
spec_id: "SPEC-1784678104"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---
# Ff Commit Amend Cycle Corrupts Linked_commits Across Multiple Commits
# Ff Commit Amend Cycle Corrupts Linked_commits Across Multiple Commits

The ff commit amend cycle overwrites linked_commits in both the ledger and spec file during each commit-amend pair. Each ff commit creates a commit, amends it (changing the hash), then finalizeCommitAfterAmend tries to update linked_commits — but the amend produces a new hash that differs from what was just written, causing repeated overwrites and lost entries. This was observed during a session where three consecutive ff commit calls each corrupted the linked_commits list, requiring manual raw git commits to correct. The fix in SPEC-1784672632 addresses the root causes (wrong path, duplicate append) but the amend-then-replace cycle itself remains a design issue that can still produce incorrect hashes on rapid successive commits.
