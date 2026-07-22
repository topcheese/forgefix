---
spec_id: "SPEC-1784678126"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---
# Chore Commits Outside Ff Lifecycle Lack Spec Binding
# Chore Commits Outside Ff Lifecycle Lack Spec Binding

When ff commit amend cycle produces corrupted state that requires manual correction, the only recourse is raw git commit. These commits exist outside the ff lifecycle — they are not bound to any spec, have no linked_commits tracking, and appear as untracked housekeeping in the git log. This defeats ForgeFix's core invariant that every commit must be traceable to a spec. A mechanism is needed to either prevent the need for raw commits (by making ff commit idempotent/correct) or to retroactively bind orphaned commits to a spec.
