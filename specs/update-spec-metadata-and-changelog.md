---
spec_id: "SPEC-1783237790"
status: ship
repo_issue: 471
type: chore
version: "0.8.2"
root_cause: "Multiple extraction specs were implemented but their root_cause and resolution fields were never filled in. The ledger version was stale at 0.8.1 and test counts showed 0. Several specs lacked repo_issue_ids from remote sync. CHANGELOG.md needed updating for the upcoming v0.8.2 release."
resolution: "Updated root_cause and resolution fields for SPEC-1783222605 (GitHubClient extraction), SPEC-1783222864 (AuditLog extraction), SPEC-1783225332 (BinaryManager), SPEC-1783224084 (410 Gone handling). Bumped ledger version 0.8.1→0.8.2, updated test counts to 349/349. Added repo_issue_ids for all synced specs. Wrote CHANGELOG.md v0.8.2 release notes. Promoted completed specs to ship. Committed as c378b40."
---

# Update Spec Metadata and Changelog

## Objective

Update spec root_cause and resolution fields for all completed extraction specs, bump ledger version to 0.8.2, and update CHANGELOG.md to reflect all changes shipped since 0.8.1.

## Requirements

- Fill in root_cause and resolution fields for specs that have been implemented but are missing metadata: SPEC-1783222605 (GitHubClient extraction), SPEC-1783222864 (AuditLog extraction), SPEC-1783225332 (BinaryManager), SPEC-1783224084 (410 Gone handling)
- Bump ledger version from 0.8.1 to 0.8.2
- Update test counts in ledger (0→349 ran, 0→349 passed)
- Add repo_issue_ids for all specs that were synced
- Update CHANGELOG.md with v0.8.2 release notes
- Promote completed specs from in-progress/draft to ship

## Implementation

Spec and ledger metadata already updated (unstaged changes). This spec documents the commit that will stage and commit those changes.

## Verification

- `git status` shows only the expected metadata files modified
- Ledger JSON is valid
- CHANGELOG.md is valid
- All tests pass after commit

## Acceptance Criteria

- [x] All implemented specs have root_cause and resolution filled in
- [x] Ledger reflects correct version, counts, and spec statuses
- [x] CHANGELOG.md has accurate v0.8.2 release notes
- [x] `ff specs` shows correct statuses
