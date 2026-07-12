---
spec_id: "SPEC-1783898268"
status: review
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
linked_commits: ["bd85a0f", "ebfe9fd", "1fb7e3c"]
---
## Goal

CHANGELOG.md is currently maintained by hand and has gone stale (last entry 2026-07-05) while the codebase keeps changing. Add an automated hook so that every `ff commit --ai` appends a changelog entry derived from the commit message, keeping the changelog in sync with actual changes without manual release bookkeeping.

## Technical Requirements

- On a successful commit bound to a spec, parse the conventional-commit message (`feat:`, `fix:`, `chore:`, etc.) and the SPEC-ID.
- Append a bullet under a date-grouped (`## [Unreleased] - YYYY-MM-DD`) section at the top of CHANGELOG.md, creating the section and a `### 🚀 Release Summary` block when absent.
- Idempotent / append-only: re-running the same commit does not create duplicate sections.
- No-op and non-fatal when CHANGELOG.md is missing.

## Acceptance Criteria

- A commit with message `feat: [SPEC-123] add thing` produces a dated Unreleased section with `- feat: add thing (SPEC-123)`.
- A second commit on the same day appends to the existing day section rather than creating a new one.
- Missing CHANGELOG.md does not error the commit.
