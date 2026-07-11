---
spec_id: "SPEC-1783723663"
status: review
repo_issue: 522
type: bug
version: "0.9.0"
root_cause: "During a refactor, the [type][status] prefix was stripped from remote issue titles created by SyncSpecs. Issues now show only the plain spec title with no type/status context."
resolution: "Implemented in issue_coordinator.go: prefixedTitle() helper, used in creation and rebind paths; title sync on status change during SyncSpecs."
---
# Restore Type And Status Prefix In Remote Issue Titles

## Objective

Remote issue titles created by `ff sync` should include `[type][status]` prefix
so the issue tracker shows the spec's category and lifecycle state at a glance.
For example: `[feature][review] My Spec Title` instead of just `My Spec Title`.

## Requirements

1. When `SyncSpecs` creates a new issue via `CreateIssueWithBody`, prefix the
   title with `[<type>][<status>]` derived from the spec's frontmatter.
2. When `SyncSpecs` updates an existing issue and the spec's status has changed,
   update the remote issue title via `UpdateIssueTitle` to reflect the new
   status.
3. The prefix uses the short status name (e.g. `draft`, `review`, `ship`,
   `closed`) not the internal YAML key.
4. The spec's human-readable title follows the prefix without the numeric spec
   ID (e.g. `[feature][review] Add User Authentication`).
5. Backward compatible — existing issues without the prefix are not broken;
   re-running `ff sync` updates their titles.

## Implementation

- In `SyncSpecs` (around line 877-890 in `issue_coordinator.go`), when building
  the title for a new issue, use:
  ```go
  title = fmt.Sprintf("[%s][%s] %s", spec.Type, spec.Status, title)
  ```
- After the remote issue is fetched and the spec status has changed, call
  `UpdateIssueTitle` with the new prefixed title.
- The `IsValidIssueTitle` check runs on the result — adjust the validator if
  the bracket format fails validation.

## Acceptance Criteria

- `ff sync` on a draft spec creates an issue titled `[feature][draft] My Spec`.
- `ff sync` on a review spec updates the issue title to `[feature][review] My Spec`.
- Existing issues without the prefix are updated on next sync.
- `IsValidIssueTitle` accepts the bracketed prefix format.

## Verification

- Run `ff sync --ai` and check the remote issue title on the Gitea server.
- Unit test in `issue_coordinator_test.go` for title formatting.
- `go test ./... -count=1` green.
