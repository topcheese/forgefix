---
spec_id: "SPEC-1783569007"
status: review
repo_issue: 485
type: feature
version: "0.9.0"
root_cause: "Specs could only be moved forward (draft→review→ship) with no way to move them backward to backlog"
resolution: "Implemented in 4247be6"
---
# Backlog Command

## Objective

Add `ff backlog <spec_id>` command to move a spec to "backlog" status, the
only direction that allows moving a spec backward in the lifecycle (from draft
or in-progress back to backlog). Also allows advancing from backlog to
in-progress.

## Requirements

1. `ff backlog <spec_id>` moves the spec to "backlog" status in both spec file
   and ledger.
2. Only allowed from "draft" or "in-progress" — reject if already "review" or
   higher.
3. If already "backlog", prompt to advance to "in-progress".
4. Updates both spec file frontmatter and ledger entry.

## Implementation

- `engine/cmd_backlog.go`: `handleBacklog` dispatcher, `runBacklog` core logic.
- Reads current status from spec file, validates transition is allowed.
- Calls `UpdateSpecFileStatus(specFile, "backlog")` and updates ledger.
- If already backlog, prompts for advancement to in-progress.

## Acceptance Criteria

- `ff backlog SPEC-X` moves a draft spec to backlog.
- `ff backlog SPEC-X` on a review spec returns an error.
- `ff backlog SPEC-X` on an already-backlog spec offers to advance.
- Ledger and spec file stay consistent.

## Verification

- `go test ./engine -run TestBacklog -v`
- Full suite: `go test ./... -count=1`

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->