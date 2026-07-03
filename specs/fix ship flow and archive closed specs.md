---
spec_id: "SPEC-1783113194"
status: ship
repo_issue: 439
type: feature
root_cause: "ShipReconciliation had no spec-based workflow path; ArchiveResolvedSpecs only handled resolved status; PostResolutionComment referenced non-existent fields"
resolution: "Added spec-based ship path in ShipReconciliation with POST_RESOLUTION + CLOSE_ISSUE + SYNC_METADATA tasks, expanded archive to handle closed status, fixed PostResolutionComment template and ResolutionCommentService to use real spec data"
---
# Fix Ship Flow And Archive Closed Specs

## Objective
Fix the `ff ship --ai` workflow for spec-based shipping so it properly closes remote issues with meaningful resolution comments, archives closed specs, and pushes to the remote repository.

## Requirements
- Spec-based ship path that skips audit-log resolution check
- Remote issues get closed with resolution comments containing spec details
- Closed/resolved specs are archived correctly
- Spec title validation works for sync-created issues

## Implementation
- Added spec-based bypass in `ShipReconciliation` (engine/execute.go:604) — skips audit-log resolution check when ship-ready specs exist
- Fixed `ArchiveResolvedSpecs` (engine/archive.go:43) to handle `closed` status alongside `resolved`
- Added `root_cause` and `resolution` field parsing in `SpecFile` + frontmatter parser
- Fixed `PostResolutionComment` in issue_coordinator.go to use actual spec data instead of empty fields
- Fixed `ResolutionCommentService.Execute` in housekeeper to generate meaningful resolution reports
- Added `LoadSpecByID` helper to find spec files by ID
- Ship now enqueues POST_RESOLUTION → CLOSE_ISSUE → SYNC_METADATA tasks for each shipped spec
- Added `isResolvedStatus` test update (ship no longer resolved — issues stay open until post-ship close)
- Spec titles auto-prefixed as `feat/spec: ` during sync to pass IssueTitleValidator

## Acceptance Criteria
1. `ff ship --ai` pushes to remote and enqueues close tasks
2. Closed issues have resolution comments with spec title, ID, version
3. `ff archive` moves closed/resolved specs to archive file
4. `ff sync` creates remote issues for new specs
5. Ship-ready specs keep issues open until ship completes

## Verification
- 314 tests pass
- Force-push to nas succeeded
- 4 housekeeping tasks enqueued after ship
- Ledger cleaned to only track active specs
- 6 closed specs archived to archive_20260703.md
