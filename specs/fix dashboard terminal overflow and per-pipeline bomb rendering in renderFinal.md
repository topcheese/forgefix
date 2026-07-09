---
spec_id: "SPEC-1783363349"
status: ship
repo_issue: 473
type: bug
version: "0.9.3"
root_cause: "renderFinal() called WriteBombFinal inside per-pipeline loop, rendering 11-line ASCII explosion once per pipeline (17x = 187 lines). render() printed all pipeline headers unconditionally regardless of terminal height, causing TUI overflow."
resolution: "Moved WriteBombFinal outside loop to render bomb art once. Added terminal-height cap on pipeline headers with '…and N more' overflow indicator."
---
# Fix Dashboard Terminal Overflow And Per Pipeline Bomb Rendering In Renderfinal

## Objective

Prevent terminal display corruption when `ff` runs in projects with many pipeline entries (> terminal height).

## Requirements

- `renderFinal()` should show bomb explosion/defusal art once, not per pipeline
- `render()` live dashboard should not overflow the terminal with more pipeline headers than visible rows
- Truncated pipelines should show an overflow indicator

## Implementation

Two changes in `engine/dashboard.go`:

1. **renderFinal()**: Moved `r.WriteBombFinal(&sb, d, p)` outside the `for _, p := range config.Pipelines` loop. Bomb art (explosion or defusal ASCII, ~11 lines) now renders once instead of N times. Added guard `if len(config.Pipelines) > 0` and uses `config.Pipelines[0]` for the floor string.

2. **render()**: Added `reservedLines` calculation for bomb (~5 lines), stats (7 lines), and blank line (1 line). Pipeline header loop capped at `termHeight - reservedLines`. When truncation occurs, renders `"… and N more pipeline(s)"` in yellow. Updated `fixedLines` and `availableForTests` calculation to use actual `headersShown` count.

## Acceptance Criteria

- [x] Running `ff` in a project with 17+ pipelines shows bomb ring and stats on screen (not pushed off)
- [x] renderFinal shows ASCII explosion once, not 17 times
- [x] "… and N more pipeline(s)" shown when terminal height exceeded
- [x] Build, tests, vet all pass

## Verification

- `go build ./...` — clean
- `go test ./...` — all 4 packages pass
- `go vet ./...` — clean
- `gofmt -l engine/dashboard.go` — no changes needed

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->