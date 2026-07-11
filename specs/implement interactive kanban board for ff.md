---
spec_id: "SPEC-1783707750"
status: review
repo_issue: 517
type: feature
version: "0.9.0"
root_cause: "No tracking UI exists for spec workflow — users must check the ledger or spec files manually to see what's in progress, review, or done"
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

### Completed
- **DB schema** (migration 001, `engine/db.go`): Tables `kanban_boards`,
  `kanban_columns`, and `kanban_cards` with foreign keys and cascade deletes.

### Not yet implemented
- CLI command `ff --kanban` (new `engine/cmd_kanban.go`).
- Data model types and business logic (`engine/kanban/` or inline).
- Persistence uses the existing SQLite DB tables (`kanban_*` in
  `.ff/forgefix.db`) — the original `.ff/kanban.json` approach is superseded.
- Sync: on load and every refresh, diff spec files and git state against cards.
- UI: simple text renderer first; Bubble Tea variant as opt-in (`--kanban-ui`).
- Card → spec linking: when a card moves to Done, update spec status via
  `UpdateLedgerAfterCommit`.—>

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