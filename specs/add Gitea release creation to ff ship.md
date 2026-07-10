---
spec_id: "SPEC-1783612709"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: "ff ship pushes commits but never creates a git tag before calling the Gitea releases API — CreateRelease references a tag_name that doesn't exist yet, so the API call fails silently"
resolution: "Implementation complete (CreateRelease, buildReleaseBody, coordinator call) but needs a git tag creation step before the API call. Fixed in commit XXXXXX once the tag issue is addressed."
---
# Add Gitea Release Creation To Ff Ship

## Objective

After `ff ship --ai` pushes commits to the remote, create a corresponding
release on the Gitea/GitHub server via API so the version tag is visible in the
remote's release UI. Currently the push succeeds but no release artifact is created.

## Requirements

1. Add `CreateRelease` method to the Gitea client (or issue coordinator).
2. Call it from the ship controller after a successful push and version prompt.
3. The release should use the version from `HandleShipVersion` as the tag.
4. The release body should summarize the shipped specs.
5. Must handle API errors gracefully (warning, not fatal).

## Implementation

- `engine/github_client.go`: Added `CreateRelease(version, body string) error`
  that POSTs to `/api/v1/repos/{owner}/{repo}/releases` with tag_name, name,
  body, draft, and prerelease fields.
- `engine/issue_coordinator.go:526`: `CreateRelease` delegates to the client.
- `engine/ship_controller.go:96-97`: Calls `coord.CreateRelease` after
  housekeeping drain, with body from `buildReleaseBody`.
- `engine/ship_controller.go:126`: `buildReleaseBody` formats shipped specs.
- **Known gap**: `CreateRelease` is called with a version tag
  that doesn't exist as a git tag yet. `ff ship` must create
  `git tag v0.9.x` before calling the API, or the release creation fails
  because the tag reference is missing.

## Verification

- Manual: run `ff ship --ai`, check Gitea releases page.
- Unit test: mock the Gitea API and verify `CreateRelease` is called with correct
  version during ship.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->