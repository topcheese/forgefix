---
spec_id: "SPEC-1783791504"
status: review
repo_issue: ""
type: bug
version: "0.9.0"
root_cause: "buildReleaseBody uses the numeric spec ID as both the ID and title when LoadSpecByID fails, producing doubled IDs. No clickable links to the remote issue."
resolution: ""
---
# Fix Release Body Doubled Spec IDs And Add Clickable Links

## Objective

The release body created by `ff ship` shows doubled spec IDs for specs that
have been archived or removed from disk. Also, spec IDs are not clickable
links to the remote issue on the Gitea server.

## Requirements

1. Fix doubled IDs: when `LoadSpecByID` fails, show the spec ID once, not
   twice in place of the title.
2. Add clickable links: format spec IDs as markdown links pointing to the
   remote issue on Gitea (`[SPEC-X](.../issues/N)`.
3. Use `repo_issue_id` from the spec or ledger to construct the link URL.

## Implementation

- `ship_controller.go`: `buildReleaseBody` now distinguishes between:
  - Spec found with title → `[SPEC-X](issue_url) Title`
  - Spec not found → `[SPEC-X](issue_url)` (no title, no doubling)
  - No issue URL available → backtick formatting as before
- The issue URL is constructed from `github.base_url`, owner, repo, and
  `repo_issue_id` from the spec file.

## Acceptance Criteria

- Release body shows `[SPEC-3707750](http://nas/.../issues/517) Title` not
  `SPEC-3707750 SPEC-3707750`.
- Archived specs without titles show a single link, not doubled.
- All existing tests pass.
