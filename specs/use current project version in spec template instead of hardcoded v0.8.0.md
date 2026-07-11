---
spec_id: "SPEC-1783785602"
status: review
repo_issue: ""
type: bug
version: "0.9.0"
root_cause: "templates/spec_template.md hardcodes version: v0.8.0 regardless of the actual compiled binary version; ff -v shows stale compile-time Version constant"
resolution: "Step 1 (02efbf8): createSpec always fills version from engine.Version. Step 2 (dcdd0d7): WriteVersion updates spec template on ship. Step 3 (4d44e49): ff -v reads project version from DB meta, falls back to compile-time Version."
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

### Step 1 (done — 02efbf8)
- In `createSpec`, the template's `version:` line is now always replaced with
  `engine.Version` (not just when `--ver` is provided). The `--ver` flag still
  overrides when explicitly given.

### Step 2
- When `ff ship` bumps the project version via `VersionManager.WriteVersion`
  (or `SetProjectVersion`), also update `templates/spec_template.md` in place
  by replacing the `version: "vX.Y.Z"` line with the new shipped version.
- This keeps the template in sync with the project so `createSpec` reads the
  correct version even without relying on the compile-time `engine.Version`.
- If the template file is missing, skip silently (non-fatal).

## Acceptance Criteria

- `ff spec --ai "test" "body"` creates a spec with `version: "0.9.0"`
  (matching `engine.Version`), not `v0.8.0`.
- `ff spec --ai "test" "body" --ver "0.5.0"` still allows manual override.
- After `ff ship --ai`, `grep version: templates/spec_template.md` shows the
  new shipped version.
- All existing tests pass.

## Verification

- `ff spec --ai "testversion" "body"`, then check `version:` in the created
  file matches `engine.Version`.
- `go test ./... -count=1` green.
