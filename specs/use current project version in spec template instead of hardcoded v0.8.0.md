---
spec_id: "SPEC-1783785602"
status: review
repo_issue: ""
type: bug
version: "0.9.0"
root_cause: "templates/spec_template.md hardcodes version: v0.8.0 regardless of the actual compiled binary version"
resolution: ""
---
# Use Current Project Version In Spec Template Instead Of Hardcoded V0.8.0

## Objective

The spec template hardcodes `version: "v0.8.0"` but the binary's actual
version (from `engine.Version` in `flags.go`) is `0.9.0`. Every new spec
created by `ff spec --ai` gets the wrong version. The binary should use its
own compiled-in version number for spec frontmatter, not a hardcoded string.

## Requirements

1. `createSpec` in `cmd_spec.go` must always write `engine.Version` into the
   spec file's `version` field — not just when `--ver` is provided.
2. Remove the hardcoded `v0.8.0` default from `templates/spec_template.md`.
3. The version comment at the bottom of the template should reference the
   actual binary version dynamically.
4. For update checking: `ff -v` already reads `engine.Version` from the
   binary. The check against the NAS Gitea releases API is separate and
   already implemented. The spec template version is the only remaining
   hardcoded value.

## Implementation

- In `createSpec`, replace the template's version line with `engine.Version`
  unconditionally (not just when `flags.SpecVersion != ""`).
- Update the template to use a placeholder like `version: "VERSION"` so it's
  obvious it gets replaced.
- The `--ver` flag overrides `engine.Version` when explicitly provided.

## Acceptance Criteria

- `ff spec --ai "test" "body"` creates a spec with `version: "0.9.0"`
  (matching `engine.Version`), not `v0.8.0`.
- `ff spec --ai "test" "body" --ver "0.5.0"` still allows manual override.
- All existing tests pass.

## Verification

- `ff spec --ai "testversion" "body"`, then check `version:` in the created
  file matches `engine.Version`.
- `go test ./... -count=1` green.
