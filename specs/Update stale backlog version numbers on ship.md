---
spec_id: "SPEC-1784672660"
status: ship
repo_issue: 550
type: feature
version: "0.9.7"
root_cause: ""



linked_commits: ["7639034", "383248f", "40a0402"]
resolution: fixed in d7412c2
---
# Update Stale Backlog Version Numbers On Ship
# Update Stale Backlog Version Numbers On Ship

When ff ship runs, specs in backlog may carry old version numbers from previous releases. These stale versions create confusion about what is planned for the current cycle. On ship, ff should scan all specs in backlog (and draft) status. Any spec whose version field does not match the incoming ship version gets its version field updated to the ship version. This keeps the backlog clean and ensures version numbers always reflect the current release cycle. Do not change specs already in review, ship, or closed status. The update should happen after the version prompt but before the push, so the version bump rides along in the shipped commit.
