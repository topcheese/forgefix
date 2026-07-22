---
spec_id: "SPEC-1784679765"
status: review
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: "writeSpecFromTemplate always prepends '# <title>' to the body, but promptForSpecBody builds a body that already starts with '# <title>', producing two H1 lines. sanitizeSpecTitle also did not strip leading '# ' from user input."
resolution: "writeSpecFromTemplate now checks if the body already starts with '# <title>' and skips prepending if so. sanitizeSpecTitle now strips a leading '# ' prefix."
linked_commits: ["6f5005c"]
---

# ff spec generates duplicated H1 heading in spec file

## Objective

When `ff spec` creates a new spec file, the H1 heading is written twice, producing two consecutive `# Title` lines at the top of the body.

## Root Cause

`writeSpecFromTemplate` at `cmd_spec.go:232` generates the body as:
```go
content := fmt.Sprintf("---%s---\n# %s\n%s\n", frontmatter, title, body)
```

The `title` parameter comes from `sanitizeSpecTitle(name)` which does not strip a leading `# ` if the user provides one. Additionally, when the spec name is already a valid title, the `# ` prefix is added unconditionally, but if the body content also starts with `# Title` (from the user or from a previous template), the heading appears twice.

Observed in multiple newly-created spec files where lines 11-12 are identical `# Title` lines.

## Problems This Causes

- Rendered markdown shows the title twice
- Looks unprofessional in spec listings and web views
- Violates markdown best practices (single H1 per document)

## Requirements

### 1. Single H1 heading
- `writeSpecFromTemplate` must emit exactly one `# <title>` line
- If the body already starts with `# <title>`, do not add another
- If the body does not start with `# <title>`, prepend it

### 2. Sanitize title input
- `sanitizeSpecTitle` should strip leading `# ` from user input to prevent double-prefixing

### 3. Regression prevention
- Unit test: create spec with plain name, verify single H1
- Unit test: create spec with `# ` prefixed name, verify single H1
- Unit test: create spec with body starting with `# Title`, verify no duplication

## Acceptance Criteria
- `ff spec "My Feature" --type feature` produces exactly one `# My Feature` line
- `ff spec "# My Feature" --type feature` produces exactly one `# My Feature` line (not `# # My Feature`)
- Existing tests pass (449 tests, 0 failures)
