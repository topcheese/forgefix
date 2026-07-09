---
spec_id: "SPEC-1783612709"
status: review
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
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

- Add `func (c *IssueCoordinator) CreateRelease(version, body string) error`
  that POSTs to `/api/v1/repos/{owner}/{repo}/releases`.
- In `ShipController.Run()`, after `DrainHousekeepingQueueFromConfig`, call
  `coord.CreateRelease(shipVersion, releaseBody)`.
- Build `releaseBody` from the list of shipped spec IDs and titles.
- Non-fatal: if the release API call fails, log a warning and continue.

## Acceptance Criteria

- `ff ship --ai` creates a release on Gitea visible at `/releases`.
- Release tag matches the ship version (e.g., `v0.9.2`).
- Release body lists shipped spec IDs.
- Failure to create release does not block the ship.

## Verification

- Manual: run `ff ship --ai`, check Gitea releases page.
- Unit test: mock the Gitea API and verify `CreateRelease` is called with correct
  version during ship.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->