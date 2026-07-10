---
spec_id: "SPEC-1783709991"
status: ship
repo_issue: 519
type: feature
version: "v0.9.5"
root_cause: ""
resolution: ""
---
# Properly Fill Spec Content At Creation Time And Sync To Remote

## Objective

`ff spec --ai "title"` currently creates an empty spec file from the template
and syncs it to the remote issue tracker verbatim. The remote issue ends up
with no useful content. Fix: the spec MUST have content at creation time.
Never allow an empty spec to be created or synced.

## Requirements

1. **Human interactive** (`ff spec <title>`): prompt user to paste full content,
   or press Enter for section-by-section prompts. No empty spec is ever written.
2. **Agent mode** (`ff spec --ai <title> <content>`): the second positional
   argument is the full filled-out spec body; write it directly, bypassing the
   template placeholders for body sections. Never create from template alone
   in `--ai` mode.
3. **Never empty**: if no content is provided in either mode, abort with an
   error. An empty spec is never written to disk and never synced to remote.
4. **Remote sync**: the full content (not the template skeleton) is what gets
   posted to the remote issue via `ff sync`. The remote issue must contain the
   actual spec Objective, Requirements, Acceptance Criteria, etc.
5. **Agent must use `ff`**: the agent must never write spec files or edit
   `.ff/` files directly. All spec creation uses `ff spec --ai <title> <content>`.
6. **Frontmatter fields** (`type`, `version`, etc.) can still be set via flags
   (`--type`, `--ver`) in both modes.

## Implementation

- In `createSpec`, before writing the file, check if content was provided
  (interactive or AI). If the body ends up empty (no content from paste,
  no content from sections, no template fill), return an error.
- For `--ai` mode: accept a second positional argument as the full spec body.
  The body replaces everything after the frontmatter in the template.
- For interactive mode: prompt "Paste full spec content (or Enter for
  section-by-section):". If Enter, prompt for each section individually.
- The frontmatter is still generated from flags + template; the body content
  (everything after the second `---`) comes from user input.
- The template file (`templates/spec_template.md`) remains as a reference for
  the expected section structure, but sections that aren't filled are left
  empty (or omitted) rather than filled with placeholder text.

## Acceptance Criteria

- `ff spec --ai "feat" ""` → error: "spec content cannot be empty".
- `ff spec --ai "feat" "## Objective\nDo X\n\n## Requirements\n..."` → spec
  file written with that body, frontmatter auto-generated.
- `ff spec "feat"` (paste "## Objective...") → same result.
- `ff spec "feat"` (press Enter, then type each section) → spec written.
- `ff sync --ai` on a spec created with full content posts that content to the
  remote issue — not the template skeleton.
- No `os.WriteFile` of spec files or direct ledger edits by the agent.

## Verification

- Unit tests for content validation (empty rejection).
- Unit tests for AI mode second-argument parsing.
- Integration test: `ff spec --ai "test" "body"` → verify file content.
- Integration test: `ff sync --ai` → verify remote issue body matches spec.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->
