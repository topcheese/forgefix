---
spec_id: "SPEC-1783791504"
status: review
repo_issue: 528
type: bug
version: "0.9.5"
root_cause: "buildReleaseBody uses the numeric spec ID as both the ID and title when LoadSpecByID fails, producing doubled IDs. No clickable links to the remote issue."
resolution: ""
---
# Fix Release Body Doubled Spec IDs And Add Clickable Links

## Objective

The release body created by `ff ship` shows doubled spec IDs for specs that
have been archived or removed from disk. Also, spec IDs are not clickable
links to the remote issue on the Gitea server.

## Requirements

### Already implemented (51c6e74)
1. Fix doubled IDs: when `LoadSpecByID` fails, show the spec ID once, not
   twice in place of the title.
2. Add clickable issue links: format spec IDs as markdown links pointing to
   the remote issue on Gitea.
3. Use `repo_issue_id` from the spec or ledger to construct the link URL.

### Link all internal references
4. Every reference to a spec (`SPEC-XXXX`), commit hash, or remote issue in
   release bodies, spec file bodies, and CLI output should be a clickable link
   to the corresponding resource on the remote Gitea instance.
5. This applies to:
   - Release body: spec IDs → issue links, commit hashes → commit links
   - Spec file body: any `[SPEC-XXXX]` or commit hash → linked automatically
   - CLI output (`ff specs`, `ff --kanban ls`): spec IDs → clickable if terminal supports

### Commit tracking in spec template
6. `templates/spec_template.md` still has no `linked_commits` field. Add it to
   the frontmatter so each spec file tracks which commits implemented it.
   (Existing spec `SPEC-1783788768` covers this — not yet implemented.)

### One commit → one spec
7. `ff commit --ai` strips only the matching spec's tag from the message but
   preserves other spec references (e.g. `[SPEC-111] integrate with [SPEC-456]`
   preserves `[SPEC-456]`). The commit is bound to only `SPEC-111` in the
   ledger. This is correct — references to other specs are informational, not
   binding. A commit belongs to one spec.

## Implementation

### Done (51c6e74)
- `ship_controller.go`: `buildReleaseBody` distinguishes between specs found
  with a title, specs not found (archived), and specs with repo_issue > 0.
- Issue URL derived from config (`github.base_url` stripped of `/api/v1`).
- No hardcoded URLs or paths.

### Not yet implemented
- Auto-link `SPEC-XXXX` and commit hashes in release body, spec body, and CLI.
- `linked_commits` field in spec template (see `SPEC-1783788768`).

## Acceptance Criteria

- Release body shows `[SPEC-3707750]({web}/issues/517) Title` not doubled IDs.
- Archived specs without titles show a single link.
- `SPEC-XXXX` references in release body and spec body are clickable links.
- Spec template includes `linked_commits: []` field.
- All existing tests pass.
