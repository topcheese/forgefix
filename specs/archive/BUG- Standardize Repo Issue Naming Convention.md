---
spec_id: "SPEC-1781317040"
status: "backlog"
type: "type/standardization"
version: "version/v0.8.0"
repo_issue: 160
---

# Standardize Repo Issue Naming Convention

## Goal
Implement a consistent, machine-parseable, and human-readable naming convention for all issues tracked within the `repo` to improve maintainability.

## Design Requirements
- **Naming Schema:** Enforce the following format for all new issues:
  `[TYPE]/[CATEGORY]: [SHORT_DESCRIPTION]`
- **TYPE Definitions:** Use the following standardized prefixes:
  - `feat/`: New functionality.
  - `fix/`: Bug or defect resolution.
  - `docs/`: Documentation updates.
  - `ops/`: Maintenance, refactoring, or infrastructure (Replaces `chore`).
- **CATEGORY Definitions:** Limit categories to core engine subsystems (e.g., `engine`, `config`, `sync`, `driver`).
- **Formatting Constraints:**
  - `SHORT_DESCRIPTION` must use imperative mood.
  - No trailing punctuation.
  - Maximum length of 60 characters for the full title.

## Acceptance Criteria
- [ ] ForgeFix CLI validates new issue titles against the `[TYPE]/[CATEGORY]: [DESC]` regex pattern.
- [ ] Automated helpers for creating issues through ForgeFix utilize this template.
- [ ] Documentation updated to reflect the new issue naming standard.
- [ ] Existing issue triage process updated to enforce this naming convention for all incoming tickets.