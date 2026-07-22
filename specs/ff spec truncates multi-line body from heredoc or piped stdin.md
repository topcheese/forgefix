---
spec_id: "SPEC-1784679743"
status: review
repo_issue: 554
type: bug
version: "0.9.7"
root_cause: "parseSpecPositional only reads body from CLI args. Piped stdin is never read, so heredoc/pipe input is silently ignored."
resolution: "handleSpec now checks if stdin is a pipe (ModeCharDevice == 0) and reads the full stdin content as body when no body was provided via args."
linked_commits: ["5e2eaf5"]
---

# ff spec truncates multi-line body from heredoc or piped stdin

## Objective

When `ff spec` receives a multi-line body via heredoc (`<<'EOF'`) or piped stdin, only the first paragraph is written to the spec file. The rest of the content is silently dropped.

## Root Cause

`parseSpecPositional` in `cmd_spec.go` reads stdin via `bufio.Scanner` but splits on blank lines, treating the first blank line as the end of input. Heredoc input with blank lines between paragraphs causes the scanner to stop reading after the first paragraph. The interactive prompt flow reads the full body correctly because it uses a different code path.

## Problems This Causes

- Specs created via `ff spec --ai` with piped body content lose all content after the first blank line
- Detailed requirements, acceptance criteria, and implementation notes are silently dropped
- The spec appears complete (frontmatter is correct) but the body is truncated
- Users must manually edit the spec file to restore lost content

## Requirements

### 1. Read full stdin body
- When body is provided via heredoc or piped stdin, read until actual EOF (not first blank line)
- Preserve blank lines in the body content
- Strip only trailing whitespace, not internal blank lines

### 2. Maintain interactive behavior
- Interactive prompt flow should continue to work as-is
- Single-paragraph bodies should work unchanged

### 3. Regression prevention
- Unit test: pipe multi-paragraph body via stdin, verify all paragraphs appear in spec file
- Unit test: heredoc with blank lines between sections, verify full content preserved

## Acceptance Criteria
- `echo -e "## Section 1\n\n## Section 2" | ff spec "Test spec" --type feature` produces a spec with both sections
- `ff spec "Test spec" --type feature <<'EOF'` with multi-paragraph body preserves all paragraphs
- Interactive prompt flow is unchanged
- Existing tests pass (449 tests, 0 failures)
