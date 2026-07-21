---spec_id: "SPEC-1784105008"
status: review
repo_issue: 539
type: bug
version: "0.9.6"
root_cause: "ff commit --ai calls autoDetectSpecFromWorkingTree (cmd_commit.go:824) which picks the most-recently-modified ACTIVE spec and binds the commit to it, then unconditionally sets that spec to status='review' (cmd_commit.go:247/251). When multiple specs were touched (e.g. a cleanup commit touching many files, or the user editing a different spec just before committing), the auto-detect binds to the wrong spec and force-promotes it to review — creating more problems than the convenience solves. The --spec flag exists to bind explicitly, but the default auto-detect overrides intent."
linked_commits: ["8c75ea9"]
---

# Fix `ff commit --ai` Auto-Detect Forcing Wrong Spec To `review`

## Objective

`ff commit --ai` auto-detects the most-recently-modified active spec, binds
the commit to it, and **force-sets it to `review`**. This frequently binds to
the wrong spec and prematurely promotes it to `review`, causing more problems
than the convenience solves. This spec changes that behavior.

## Root Cause (from code reading)

- `autoDetectSpecFromWorkingTree` (cmd_commit.go:824) prefers active specs
  and picks the most recently modified one (lines 819-849). Comment says it
  exists "so that `ff commit --ai` can bind to it without an interactive
  prompt."
- After detection, `ff commit --ai` calls `UpdateSpecFileStatus(specFile,
  "review")` (cmd_commit.go:247) and sets `entry.Status = "review"`
  (cmd_commit.go:251) — **unconditionally**, regardless of whether the user
  intended that spec.
- The `--spec <id>` flag already allows explicit binding, but the default
  path ignores intent and force-promotes whatever file was last touched.

## Problems This Causes

- A cleanup/multi-file commit (e.g. deleting rogue specs) gets bound to the
  cleanup spec and forces it to `review` even when the user may not want it
  promoted yet.
- Editing a spec file moments before committing an unrelated change steals
  the binding to the edited spec.
- Force-promotion to `review` bypasses deliberate status workflow.

## Requirements

### 1. Do not force `review` on auto-detect
- When `ff commit --ai` auto-detects a spec (no `--spec` given), bind the
  commit to it but **do NOT auto-promote to `review`** unless the spec is
  already in a state that implies readiness, or the user explicitly opts in.
- Prefer leaving the spec at its current status, or require an explicit
  `--review` flag to promote.

### 2. Prefer explicit binding
- Make `--spec <id>` the recommended path; when provided, bind and promote
  only per the user's explicit intent (e.g. `--review` to promote).
- When auto-detect is used and ambiguous (multiple recently-modified active
  specs), either (a) prompt, or (b) refuse and require `--spec`, rather than
  guessing.

### 3. Keep convenience for the simple case
- Single-active-spec repos should still work without `--spec` (bind + no
  forced promotion, or promote only with `--review`).

## Acceptance Criteria

- `ff commit --ai` without `--spec` binds the commit but does NOT force the
  detected spec to `review` unless `--review` is passed.
- `ff commit --ai --spec <id>` binds explicitly and only promotes when
  `--review` is given.
- Ambiguous auto-detect (multiple recent active specs) prompts or requires
  `--spec` instead of guessing.
- No regression to the 13 pre-existing failing integration tests (additive
  change; those remain tracked separately).
