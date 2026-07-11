---
spec_id: "SPEC-1783612710"
status: review
repo_issue: 516
type: feature
version: "0.9.5"
root_cause: "effectiveSpecBody was used for issue creation but not for body updates — the comparison and UpdateIssueBody calls used raw spec.Body instead, causing inconsistent posting for template-only bodies"
resolution: "Fixed in df6dd64 (engine/issue_coordinator.go:981-982 — comparison and update use effectiveSpecBody). Test added in 2e91545 (TestSyncIssueBodyUpdatesOnEffectiveBodyChange)."
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

- `spec_manager.go`: Added `isTemplateBody()` to detect template-only bodies
  and `effectiveSpecBody(spec)` to return a meaningful body (from frontmatter
  fields) when the body is empty template text. `effectiveSpecBody` was already
  used for issue creation (`CreateIssueWithBody` calls).
- `cmd_spec.go:242`: Added `sanitizeSpecTitle(name)` to handle `--` in spec
  names — replaces dashes with spaces and collapses multiple spaces.
- `issue_coordinator.go:981-982`: **Fixed gap** — the body comparison and
  `UpdateIssueBody` call used raw `spec.Body` instead of
  `effectiveSpecBody(spec)`. Changed to use `postedBody := effectiveSpecBody(spec)`
  so the comparison and update match what was actually posted at creation time.
  Previously, template-only bodies would cause a mismatch on every sync.

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
