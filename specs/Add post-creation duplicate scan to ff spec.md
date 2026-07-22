---
spec_id: "SPEC-1784101811"
status: review
repo_issue: 534
type: chore
version: "0.9.6"
root_cause: ""
resolution: ""
linked_commits: ["2371aba", "65be191"]
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
