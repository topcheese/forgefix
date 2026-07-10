---
spec_id: "SPEC-1783707750"
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Implement Interactive Kanban Board For Ff

## Objective

Add a lightweight interactive Kanban board to `ff` (`ff --kanban`) that runs
alongside the existing dashboard. Cards represent ForgeFix specs, PRs, or
commits. Columns are user-definable (To Do, In Progress, Review, Done).
The board syncs with local spec files and git state, and automatically updates
when spec statuses change.

## Requirements

1. `ff --kanban` spawns an interactive board view in the terminal.
2. Board operations: create/rename/delete columns.
3. Card operations: add, move between columns, delete, edit.
4. Cards link to ForgeFix specs (spec_id), PRs, or commit hashes.
5. Live sync — board polls local spec and git state every 15s (configurable).
6. Persistence — board state stored in `.ff/kanban.json`.
7. Non-intrusive — does not disrupt the existing dashboard or test runner.
8. Optional Bubble Tea UI via `--kanban-ui=bubbletea` flag.

## Implementation

- New package: `engine/kanban/` (or `engine/kanban_board.go`).
- Data model: `KanbanBoard` → `KanbanColumn` → `KanbanCard` with Spec/PR/Commit
  type discrimination.
- CLI integration: `ff --kanban` routes to `handleKanban` in a new
  `engine/cmd_kanban.go`.
- Parsing: reuse existing `ParseFlags` pattern.
- Persistence: read/write `.ff/kanban.json` on every mutation.
- Sync: on load and every refresh, diff spec files and git state against cards.
- UI: ship with simple text renderer first; Bubble Tea variant as opt-in.
- Card → spec linking: when a card moves to Done, update the spec status from
  `review` to `ship`.

## Acceptance Criteria

- `ff --kanban` opens and renders the board.
- `column new/add/delete` works.
- `card add/move/delete/view` works with spec, PR, and commit types.
- Board refreshes when spec file statuses change.
- State persists across `ff --kanban` sessions.
- All existing tests, dashboard, and ship gate remain unaffected.

## Verification

- `go test ./engine/kanban/... -count=1` passes.
- Manual: `ff --kanban` → create columns → add cards → move → refresh → quit.
- Board state file `.ff/kanban.json` is valid JSON on disk after quit.
- Full suite: `go test ./... -count=1` green.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->