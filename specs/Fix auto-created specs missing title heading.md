---
spec_id: "SPEC-1784225300"
status: ship
repo_issue: 540
type: bug
version: "0.9.6"
root_cause: ""
resolution: ""
linked_commits: ["9068fa7"]
---
# Fix auto-created specs missing title heading

## Objective
When `ff` auto-creates a spec (e.g. when a test fails during `ff --ai`), the generated file has no `# Title` H1 heading. `LoadLedger` / `SyncFromSpecsDir` therefore cannot read a title from the `# …` H1 and substitutes the `spec_id` as the display title — the ledger `title` column ends up equal to the `spec_id` instead of a human-readable name.

This spec also investigates how the existing specs in this repository were actually created. They should have been produced through `ff spec` (which runs the pre-creation duplicate detector and generates a unique `SPEC-<unix>` id), but multiple files share colliding `spec_id`s, a strong signal that some were hand-written by agents rather than created via `ff`.

## Root Cause
`engine/cmd_spec.go` — `writeSpecFromTemplate` builds the file as `---<frontmatter>---\n<body>`, where `body` is just the `## Objective / ## Root Cause / ## Failure Details` text. No `# <title>` H1 line is ever written. `parseSpecFile` (spec_manager.go) extracts the title from the `# …` H1; when absent it falls back to the `spec_id`, so the ledger `title` column is set to the spec_id rather than a real title.

## Investigation: How were the existing specs created?
1. Record each spec's `spec_id`, `created` date, `status`, and whether the body contains a `# Title` H1 heading.
2. Inspect `git log` for each spec file to determine if it was created by `ff spec` (committed through the `ff` lifecycle) or hand-written directly (raw file write / agent edit outside `ff`).
3. Query the ledger (`specs` table in `.ff/forgefix.db`): for each `spec_id`, check whether `title` equals the human title or the `spec_id` itself — the latter is a tell-tale sign of the missing-heading bug.
4. Note the spec_id collisions: at least three files (`Fix Test404reconciliation.md`, `Fix Testruncommitwithflagspecid.md`, `Fix Testspeclifecycle.md`) all carry `SPEC-1784146853`, and several others share `SPEC-1784143269` / `SPEC-1784143268`. `writeSpecFromTemplate` uses `fmt.Sprintf("SPEC-%d", time.Now().Unix())`, so genuine `ff` creation yields a unique id per file; identical ids across files indicate same-second batch creation or manual authoring that hard-coded / copied an id.
5. Determine whether agents are invoking `ff spec` or writing `specs/*.md` directly. If the latter, document the gap: manual authoring bypasses the pre-creation duplicate detector (`FindDuplicateSpec`, called inside `createSpec`), so duplicates and id collisions go uncaught.
6. Report findings: classify every existing spec as ff-created vs manually-authored, with cited evidence (git log, ledger title value, heading presence, id collisions).

## Requirements
1. `writeSpecFromTemplate` must emit a `# <title>` H1 heading using the `title` argument it already receives, so every auto-created spec carries a real title in its body.
2. Complete the investigation above and record the evidence for how each existing spec was created.
3. Confirm whether manually-authored specs bypass the pre-creation duplicate detector, and if so, decide whether `ff` should reject/gate specs whose `spec_id` collides with an existing one at write time.

## Acceptance Criteria
- Auto-created specs contain a `# Title` H1 heading and the ledger `title` equals the human title, not the `spec_id`.
- The Investigation section is complete: every existing spec is classified as ff-created or manually-authored, with cited evidence.
- No spec files share a colliding `spec_id` that should have been caught by the pre-creation duplicate detector.
- This spec is additive and does not alter the Test404Reconciliation fix (SPEC-1784146853).
