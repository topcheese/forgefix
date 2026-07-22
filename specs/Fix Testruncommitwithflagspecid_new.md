---
spec_id: "SPEC-1784255528"
status: ship
repo_issue: 545
type: bug
version: "0.9.6"
root_cause: "TestRunCommitWithFlagSpecID asserted that a plain ff commit auto-promotes a spec to status: review. The intended design (established in commit db296db) is that a plain commit does NOT auto-promote. The test's assertions were stale."
resolution: "Updated TestRunCommitWithFlagSpecID to match intended design: plain commit leaves status unchanged (in-progress), only --review flag promotes to review."
linked_commits: ["3a3d43f"]
---

# Fix TestRunCommitWithFlagSpecID

## Objective

Automatically created from failing test TestRunCommitWithFlagSpecID during ff --ai run. The test verifies that ff commit with --spec flag correctly binds and updates spec status.
