---
spec_id: "SPEC-1784262174"
status: draft
repo_issue: 546
type: refactor
version: "0.9.7"
root_cause: "ff backlog was a standalone command for spec status management, duplicating logic that belongs under ff spec. Collapsing it into ff spec --status simplifies the CLI surface and unifies status management under one command."
linked_commits: ["1a79972", "fe63c8e", "c3728c3"]
---
# Problem

The `ff backlog <spec_id>` command is a standalone subcommand that manages spec status. It has its own usage, help text, and dispatch entry in command_dispatcher.go. This is confusing — spec status management should live under `ff spec`, not as a separate command with an unrelated name ('backlog' only describes one possible transition, not what the command does).

## Current Behavior

- `ff backlog <spec_id>` moves a spec to "backlog" status (if not already there)
- If already backlog, it shows an interactive promotion menu: 1. in-progress, 2. review, 3. ship, 4. close, 5. delete
- Both spec file frontmatter and ledger are updated atomically
- `ff spec <name>` only creates new specs — it has no capability to manage existing specs

## Requirements

1. Remove `ff backlog` as a standalone command (remove dispatch entry, remove help text, remove handler code)
2. Add `--status <status>` flag to `ff spec` for non-interactive status changes: `ff spec <spec_id> --status review`
3. When `ff spec <spec_id>` is called on an existing spec without `--status` or `--delete`, show the interactive promotion menu (same logic as current promptAdvancement)
4. Both spec file and ledger must be updated atomically on every status change
5. The interactive menu must support all valid status transitions: in-progress, review, ship, close, delete
6. `ff commit` (human, explicit) sets the bound spec to `review` — the human has tested the code before committing and is intentionally marking it ready for review
7. `ff commit --ai` (AI/agent mode) sets the bound spec to `draft` — the AI ran tests but a human must verify before promotion to `review`
8. No `--review` flag is needed on `ff commit` — the human/AI distinction already determines the post-commit status
9. All valid status values must be accepted by `--status`: backlog, draft, in-progress, review, ship, closed

## Implementation

- Move the interactive promotion menu (currently promptAdvancement in cmd_backlog.go) into cmd_spec.go
- Create a shared setSpecStatus(specID, status, configDir) helper that updates both spec file and ledger atomically
- Rename handleBacklog to handleSpecStatus or inline into handleSpec
- Remove the `backlog` case from command_dispatcher.go
- Update help text accordingly
- Keep `ff backlog` as a backward-compatible alias for one release cycle (optional — we can skip this since 0.8.5 hasn't shipped yet)
- `ff commit`: after resolving the spec and creating the commit, set status to `review` (both file + ledger)
- `ff commit --ai`: after commit, set status to `draft` (both file + ledger) — AI output needs human verification
- No need for a separate `--review` flag; the `--ai` flag is the discriminator
- Update `ff help` and `ff spec --help` text to reflect `--status` flag and the removal of `ff backlog`
- Update any internal docs or man pages that reference `ff backlog` or the old promotion model

## Acceptance Criteria

1. `ff backlog <spec_id>` produces "unknown command" or routes to help
2. `ff spec <spec_id> --status review` promotes the spec to review (both file + ledger)
3. `ff spec <spec_id>` (on existing spec, without flags) shows interactive menu
4. `ff spec <spec_id> --status invalid` shows error with valid statuses
5. `ff spec <new-name>` still creates a new spec (no regression)
6. `ff spec <spec_id> --delete` still deletes a spec (no regression)
7. All existing tests pass
