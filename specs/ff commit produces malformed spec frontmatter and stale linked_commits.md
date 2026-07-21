---
spec_id: "SPEC-1784280001"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: ["07f33dc", "2ebba65"]
---

# ff commit produces malformed spec frontmatter and stale linked_commits

## Objective

`ff commit --ai` (and likely `ff commit`) corrupts the bound spec file's
frontmatter and writes a stale commit hash to `linked_commits`. This was
observed while committing SPEC-1784264619 and is reproducible across multiple
specs (e.g. SPEC-1784275332 shows the same defect).

## Root Cause

Two distinct defects:

1. **Duplicate `linked_commits:` keys.** `UpdateSpecFileLinkedCommits`
   (engine/cmd_backlog.go:94) walks the frontmatter line-by-line and, on the
   first `linked_commits:` line it finds, either replaces that line or — if it
   believes the field is missing — inserts a *new* `linked_commits:` line before
   the closing `---`. When the spec already carries a `linked_commits:` key (for
   example the `linked_commits: []` placeholder created by `ff spec`), the
   function can leave the original key in place and append another, producing
   three `linked_commits:` keys in the file. A YAML document with duplicate keys
   is ambiguous/undefined — parsers keep either the first or last arbitrarily.

2. **Stale hash after amend.** `ff commit` writes `linked_commits: ["<hash>"]`
   using the commit hash it just created, then **amends** that commit (folding
   the metadata change into it). The amend produces a *new* SHA, but the spec
   file and ledger still reference the pre-amend SHA. The linked commit therefore
   points at a dangling/rewritten object rather than HEAD.

   Observed in reflog for SPEC-1784264619:
   ```
   feab430 HEAD@{0}: commit (amend): feat: [SPEC-1784264619] refactor:
   64c802c HEAD@{1}: commit:       feat: [SPEC-1784264619] refactor:
   ```
   The spec recorded `64c802c` while HEAD became `feab430`.

## Problems

- **Invalid frontmatter**: duplicate `linked_commits:` keys break strict YAML
  parsers and make the field's value non-deterministic.
- **Broken traceability**: `linked_commits` points at a pre-amend SHA that is no
  longer HEAD, defeating the purpose of the field (pointer to the implementation).
- **Status never advanced**: the spec stays `draft` even after the implementing
  commit lands; there is no automatic transition to `implemented`/`closed`.
- **Systemic**: SPEC-1784275332 exhibits the same stale-hash pattern
  (`linked_commits: ["45e3297"]` while HEAD is `24c12a3`), confirming this is not
  a one-off.

## Requirements

### 1. De-duplicate `linked_commits` on write
- `UpdateSpecFileLinkedCommits` must never leave more than one `linked_commits:`
  key in the frontmatter. If duplicates exist, collapse them into a single key
  containing the union of all hashes (de-duplicated).
- Add a regression test that feeds a frontmatter with 2–3 duplicate
  `linked_commits:` keys and asserts exactly one key remains with the correct
  hash set.

### 2. Record the final SHA, not the pre-amend SHA
- After `ff commit` amends the commit, re-read the resulting HEAD SHA and write
  *that* to `linked_commits` (and the ledger). The hash written must equal
  `git rev-parse HEAD` at the end of the command.
- Alternatively, write `linked_commits` *after* the amend completes, not before.

### 3. Advance spec status on commit
- When `ff commit` successfully binds a commit to a spec, advance the spec
  `status` from `draft` to `implemented` (or the project's configured
  post-commit status), unless the spec is already in a terminal state.

### 4. Validate frontmatter on write
- Before writing the spec file back, assert the frontmatter parses as valid YAML
  with no duplicate keys. Fail loudly rather than writing corruption.

## Acceptance Criteria

- `ff commit --ai` on a spec with existing `linked_commits: []` produces exactly
  one `linked_commits:` key.
- The hash in `linked_commits` equals `git rev-parse HEAD` after the command
  returns.
- The bound spec's `status` is advanced past `draft` after a successful commit.
- No spec file written by `ff commit` contains duplicate frontmatter keys.
- All existing tests pass.
