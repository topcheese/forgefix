---
spec_id: "SPEC-1783237790"
status: in-progress
repo_issue: 471
type: chore
version: "0.8.2"
root_cause: ""
resolution: ""
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

- [ ] All implemented specs have root_cause and resolution filled in
- [ ] Ledger reflects correct version, counts, and spec statuses
- [ ] CHANGELOG.md has accurate v0.8.2 release notes
- [ ] `ff specs` shows correct statuses
