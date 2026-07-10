---
spec_id: "SPEC-1783612710"
status: draft
repo_issue: 516
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Fix Issue Body Syncing To Post Complete Spec Content

## Objective

`ff sync` posts `spec.Body` (content after frontmatter) as the remote issue
body. For draft specs with only template headings, this produces an issue with
no useful content. Additionally, spec names containing `--` produce garbled
titles (triple spaces). Fix the body quality validation and title formatting.

## Requirements

1. Validate spec body before posting — if the body is only template headings
   and comments, generate a better body from the spec frontmatter.
2. Fix title formatting when the spec name contains `--` (or other special
   characters that get mangled by the spec name parser).
3. Ensure body updates are correctly sent when the spec body changes
   (the body comparison in `SyncSpecs` must handle whitespace differences).

## Implementation

- In `syncSingleSpec` or `SyncSpecs`, before calling `CreateIssueWithBody`,
  check if `spec.Body` is essentially empty (only template text). If so,
  generate a body from the spec's frontmatter fields (objective, requirements).
- In `ff spec --ai`, sanitize the spec title to avoid garbled spacing from
  `--` or other special character sequences.
- In the body comparison at `issue_coordinator.go:956`, normalize whitespace
  before comparing `remoteIssue.Body` to `spec.Body`.

## Acceptance Criteria

- `ff sync` on a draft spec with only template body posts a meaningful issue.
- `ff spec --ai "fix --ai null pointer"` produces a clean spec title.
- Body updates are sent when the spec content changes (even whitespace-only).
- `TestSpecLifecycle`'s mock server validates received request bodies.

## Verification

- Unit test for title sanitization.
- Integration test verifies issue body content via mock request inspection.

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->