---
spec_id: "SPEC-1784690583"
status: draft
repo_issue: 556
type: bug
version: "0.9.8"
root_cause: ""
resolution: ""
linked_commits: []
---
# Ff Commit Should Not Set Spec Status By Default
ff commit --ai unconditionally sets spec status (draft for feature/bug/refactor, review for chore) even when the spec is already at that status or higher. This causes regressions — e.g. a spec at review gets downgraded to draft on the next commit. Status should only change when --review is explicitly passed. The resolveTargetStatus function should be stripped to only honor --review.
