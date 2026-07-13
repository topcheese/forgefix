---
spec_id: "SPEC-1783899571"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
linked_commits: ["20a6e10"]
---
## Goal

Agents (and ad-hoc edits) are mutating ForgeFix-managed files directly instead of going through the `ff` CLI. Observed symptoms: CHANGELOG.md goes stale / is edited by hand, forgefix_ledger.json and spec frontmatter are written to directly, and `ff ship` pushes to the public GitHub remote (`origin`) instead of the configured local NAS remote. The result is drift between what ff believes the project state is and what is actually on disk / pushed.

This spec covers the enforcement gap: tracked ForgeFix artifacts must only be mutated through `ff` commands, and `ff ship` must validate its push target before touching a remote.

## Technical Requirements

- Catalog every file ff treats as authoritative (CHANGELOG.md, .ff/forgefix_ledger.json, specs/*.md frontmatter, forgefix.yaml config) and ensure each has a single ff-owned mutation path.
- Detect / block direct writes to these artifacts outside the ff CLI (pre-commit check or watcher) so hand-edits are flagged rather than silently drifting.
- `ff ship` must resolve its git push target from configuration, not hardcode `origin`. Add a safeguard that refuses to push (or push tags/releases) to a remote whose URL matches a public host (e.g. github.com) unless that remote is explicitly the configured ship target.
- Credentials for remote issue tracking must load from the gitignored project yaml (`forgefix_ff.yaml`) / env, never be required inline in any synced file, and must not be scrubbed by local secret scanners that ignore .gitignore.

## Acceptance Criteria

- A direct edit to CHANGELOG.md or forgefix_ledger.json outside `ff` is detected and reported (and ideally blocked) rather than committed silently.
- `ff ship` pushes only to the configured NAS remote; attempting to push unreviewed code to the public GitHub origin is aborted with a clear safeguard error.
- `ff sync`/`ff ship` obtain credentials from the gitignored config and function with no inline secrets in version-controlled files.
