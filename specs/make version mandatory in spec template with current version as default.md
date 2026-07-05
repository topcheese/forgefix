---
spec_id: "SPEC-1783153514"
status: closed
repo_issue: 455
type: feature
root_cause: "The spec template did not include a version field, so newly created specs had no version label applied during sync. Only the status label was being set. Users had to manually add the version field, and many specs were created without it, resulting in incomplete labeling on remote issues."
resolution: "Added version field with current release (v0.8.0) as default to templates/spec_template.md, plus HTML comment block documenting available types and versions. New specs now include version automatically. See commit 5c4c377."
version: "0.8.1"
---
# Make Version Mandatory In Spec Template With Current Version As Default

## Objective
Ensure every new spec has a version field pre-populated with the current release version, and document available types so users know their options.

## Requirements
1. The spec template must include a `version` field with the current release version as default.
2. The spec template should document available types and versions in a comment block.
3. `ff spec` should create specs with the version field already set.

## Implementation
- Updated `templates/spec_template.md`:
  - Added `version: "v0.8.0"` (current release)
  - Added HTML comment block listing available types and versions

## Acceptance Criteria
- [ ] `ff spec <name>` creates a spec with `version: "v0.8.0"` in the frontmatter.
- [ ] Remote issues created from specs have the version label applied.
- [ ] Spec template includes documentation of available types.

## Verification