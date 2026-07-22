---
spec_id: "SPEC-1784744083"
status: draft
repo_issue: 651
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# [bug][draft] Fix Background Sync Ledger Race Condition
## Objective
The background sync process frequently modifies .ff/forgefix_ledger.json immediately after commits, creating a race condition that leaves the git tree dirty and forces repetitive 'ledger sync' commits.

## Requirements
1. Investigate the background sync trigger and timing in engine/cmd_commit.go and engine/sync.go.
2. Implement a mechanism to ensure the ledger is in a stable, committed state after all metadata updates are finalized.
3. Prevent asynchronous ledger modifications from dirtying the tree during the implementation/commit loop.
