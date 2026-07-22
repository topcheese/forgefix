---
spec_id: "SPEC-1784742157"
status: draft
repo_issue: 558
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# Fix Ff Sync Not Promoting Review Specs To Ship
## Objective
ff sync is supposed to promote specs from 'review' status to 'ship' status after successfully synchronizing with the remote issue tracker. Currently, specs are remaining in 'review' status even after a successful sync.

## Root Cause
Investigation needed in engine/cmd_sync.go and engine/sync.go to ensure the status transition logic is correctly triggered and persisted to both the spec file and the database after remote synchronization.

## Requirements
1. ff sync must promote all successfully synced 'review' specs to 'ship' status.
2. The status change must be persisted to the spec's frontmatter.
3. The status change must be persisted to the database/ledger.
4. Verify via unit or integration tests.
