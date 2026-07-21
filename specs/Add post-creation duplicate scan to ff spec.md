---spec_id: "SPEC-1784101811"
status: review
repo_issue: 534
type: feature
version: "0.9.6"
root_cause: ""
linked_commits: ["2d29242"]
---
   spec_id: "SPEC-1784101811"
  -status: draft
  +status: review
   repo_issue: ""
   type: feature
   version: "0.9.6"
  diff --git a/specs/Fix ff commit auto-detect forcing review on wrong spec.md b/specs/Fix ff commit auto-detect forcing review on wrong spec.md
  new file mode 100644
  index 0000000..7f74e90
  --- /dev/null
  +++ b/specs/Fix ff commit auto-detect forcing review on wrong spec.md	
  @@ -0,0 +1,73 @@
  +---
  +spec_id: "SPEC-1784105008"
  +status: draft
  +repo_issue: ""
  +type: bug
  +version: "0.9.6"
  +root_cause: "ff commit --ai calls autoDetectSpecFromWorkingTree (cmd_commit.go:824) which picks the most-recently-modified ACTIVE spec and binds the commit to it, then unconditionally sets that spec to status='review' (cmd_commit.go:247/251). When multiple specs were touched (e.g. a cleanup commit touching many files, or the user editing a different spec just before committing), the auto-detect binds to the wrong spec and force-promotes it to review — creating more problems than the convenience solves. The --spec flag exists to bind explicitly, but the default auto-detect overrides intent."
  +resolution: ""
  +linked_commits: []
  +---
  +
  +# Fix `ff commit --ai` Auto-Detect Forcing Wrong Spec To `review`
  +
  +## Objective
  +
  +`ff commit --ai` auto-detects the most-recently-modified active spec, binds
  +the commit to it, and **force-sets it to `review`**. This frequently binds to
  +the wrong spec and prematurely promotes it to `review`, causing more problems
  +than the convenience solves. This spec changes that behavior.
  +
  +## Root Cause (from code reading)
  +
  +- `autoDetectSpecFromWorkingTree` (cmd_commit.go:824) prefers active specs
  +  and picks the most recently modified one (lines 819-849). Comment says it
  +  exists "so that `ff commit --ai` can bind to it without an interactive
  +  prompt."
  +- After detection, `ff commit --ai` calls `UpdateSpecFileStatus(specFile,
  +  "review")` (cmd_commit.go:247) and sets `entry.Status = "review"`
  +  (cmd_commit.go:251) — **unconditionally**, regardless of whether the user
  +  intended that spec.
  +- The `--spec <id>` flag already allows explicit binding, but the default
  +  path ignores intent and force-promotes whatever file was last touched.
  +
  +## Problems This Causes
  +
  +- A cleanup/multi-file commit (e.g. deleting rogue specs) gets bound to the
  +  cleanup spec and forces it to `review` even when the user may not want it
  +  promoted yet.
  +- Editing a spec file moments before committing an unrelated change steals
  +  the binding to the edited spec.
  +- Force-promotion to `review` bypasses deliberate status workflow.
  +
  +## Requirements
  +
  +### 1. Do not force `review` on auto-detect
  +- When `ff commit --ai` auto-detects a spec (no `--spec` given), bind the
  +  commit to it but **do NOT auto-promote to `review`** unless the spec is
  +  already in a state that implies readiness, or the user explicitly opts in.
  +- Prefer leaving the spec at its current status, or require an explicit
  +  `--review` flag to promote.
  +
  +### 2. Prefer explicit binding
  +- Make `--spec <id>` the recommended path; when provided, bind and promote
  +  only per the user's explicit intent (e.g. `--review` to promote).
  +- When auto-detect is used and ambiguous (multiple recently-modified active
  +  specs), either (a) prompt, or (b) refuse and require `--spec`, rather than
  +  guessing.
  +
  +### 3. Keep convenience for the simple case
  +- Single-active-spec repos should still work without `--spec` (bind + no
  +  forced promotion, or promote only with `--review`).
  +
  +## Acceptance Criteria
  +
  +- `ff commit --ai` without `--spec` binds the commit but does NOT force the
  +  detected spec to `review` unless `--review` is passed.
  +- `ff commit --ai --spec <id>` binds explicitly and only promotes when
  +  `--review` is given.
  +- Ambiguous auto-detect (multiple recent active specs) prompts or requires
  +  `--spec` instead of guessing.
  +- No regression to the 13 pre-existing failing integration tests (additive
  +  change; those remain tracked separately).
  +
  diff --git a/test-specs/templates/spec_template.md b/test-specs/templates/spec_template.md
  new file mode 100644
  index 0000000..f956ba1
  --- /dev/null
  +++ b/test-specs/templates/spec_template.md
  @@ -0,0 +1,21 @@
  +---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Cr... [truncated at 10KB]
---

# Add Post-Creation Duplicate Scan To `ff spec`

## Objective

Close the gap found in `SPEC-1784101804` (duplicate `spec_id`
escaped the title-only detector). After `ff spec` / `ff spec --ai`
writes a new spec file, automatically scan for duplicates — both an
**exact `spec_id` collision** and a **title-similarity** match — and
surface the result before the spec is treated as live.

## Requirements

### 1. Exact `spec_id` collision check (the missing guard)

- After `writeSpecFromTemplate` assigns `spec_id`, scan all existing
  `specs/*.md` for the same id. If found, this is a HARD conflict
  (two files, one id corrupts the ledger).
- This must run regardless of title — the title-based `FindDuplicateSpec`
  already covers fuzzy title matches; the id check is the new layer.

### 2. Post-creation scan hook

- Wire the scan into `createSpec` / `handleSpec` immediately after
  the file is written (before any background `ff sync` is queued).
- Reuse `listSpecs()` (already parses `spec_id` + `title`) so no
  new filesystem walk is needed.

### 3. Behavior on detection

- **id collision (hard)**: block creation or auto-link to the existing
  spec; never leave two files with one id on disk.
- **title similarity > threshold (soft)**: warn and offer link/update
  (reuse existing `promptForDuplicateAction` in interactive mode).

### 4. Pros / Cons to explore before locking the design

| Option | Pros | Cons |
|--------|------|------|
| Block on id collision, warn on title | Prevents ledger corruption; non-fatal for fuzzy dups | Interactive prompt breaks `--ai` headless mode |
| Auto-link on any match (title OR id) | Zero manual steps; safe for agents | May silently merge specs the user wanted separate |
| Warn-only (never block) | Simple; never interrupts flow | Does not actually prevent the SPEC-1784089531 class of bug |
| Exact-id check at write time + title check deferred to sync | Catches the corruption early, defers fuzzy to existing flow | Two code paths to maintain |

- Decide: should `--ai` mode auto-link (no prompt) or auto-block? The
  current `promptForDuplicateAction` returns `"link"` for AI mode — confirm
  that is the desired behavior for an id collision too.
- Performance: `listSpecs` reads every spec file. With 90+ specs this
  is still sub-millisecond; acceptable for a post-create hook. Confirm
  no O(n²) scan is introduced if called per-spec in a batch.

## Acceptance Criteria

- Creating a second spec file with an already-used `spec_id` is
  detected and handled (blocked or auto-linked), not silently written.
- Title-similarity detection still works (existing `FindDuplicateSpec`
  behavior preserved).
- `ff spec --ai` in headless mode does not hang on a prompt for
  an id collision — it auto-resolves (link or block) and logs it.
- Unit test: seed two specs with the same `spec_id` + different titles,
  run the post-create scan, assert collision is reported.
- No regression in the 13 currently-failing integration tests
  (this is additive; those remain tracked separately).
