---
spec_id: "SPEC-1783707750"
status: ship
repo_issue: 517
type: feature
version: "0.9.0"
root_cause: "No tracking UI exists for spec workflow — users must check the ledger or spec files manually to see what's in progress, review, or done"
resolution: "Implemented across multiple commits: DB schema, CLI CRUD, spec linking, pipeline stats, and interactive watch mode. See git log for SPEC-1783707750."
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

### Implemented
- **DB schema** (migration 001): Tables `kanban_boards`, `kanban_columns`,
  `kanban_cards` with foreign keys and cascade deletes.
- **Basic CLI** (`engine/cmd_kanban.go`): `ff --kanban init`, `column new`,
  `column ls`, `card add`, `card move`, `card delete`, `ls`.
- **Spec linking**: Card titles starting with `SPEC-` show the spec's ledger
  status. `SyncCards` auto-moves cards to the matching column on every render.
- **Live pipeline stats**: The "In Progress" column shows `[tests: X/Y pass, Z fail]`
  from `pipeline_stats` in the DB.
- **Interactive watch mode**: `ff --kanban ui` renders the board, redraws every
  2s, auto-syncs cards, `q` to quit.
- **State persistence**: All board data lives in the SQLite `kanban_*` tables.

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