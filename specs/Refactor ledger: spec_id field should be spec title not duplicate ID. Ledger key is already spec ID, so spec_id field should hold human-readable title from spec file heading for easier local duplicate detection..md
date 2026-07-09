---
spec_id: "SPEC-1783495360"
status: closed
repo_issue: 476
type: refactor
version: "v0.8.0"
root_cause: "spec_id field duplicated the map key, wasting a field that could hold the human-readable title"
resolution: "Refactored all creation/update sites to set spec_id to the spec title from the markdown heading. Migrated existing ledger entries."
---
# Refactor Ledger: Spec_id Field Should Be Spec Title Not Duplicate Id

## Objective

Refactor the `forgefix_ledger.json` to change the `spec_id` field in each spec mapping entry from a duplicate of the map key (which is already the spec ID) to the human-readable spec title from the spec file's markdown heading. This makes local searching for duplicate specs easier without needing to check GitHub issues online.

## Requirements

1. **FR-1**: The ledger key remains the spec ID (e.g., "SPEC-1783157281")
2. **FR-2**: The `spec_id` field in each entry becomes the spec title from the spec file's `#` heading (e.g., "Project Status Dashboard Command")
3. **FR-3**: For specs without a corresponding markdown file in `specs/`, keep the spec ID as fallback
4. **FR-4**: Update all code that reads/writes the ledger to handle the new field meaning
5. **FR-5**: All existing tests must pass after the change

## Implementation

1. Read all spec files from `specs/` directory to build a mapping of spec ID → title
2. Update the ledger JSON to replace `spec_id` values with titles
3. Update `engine/sync.go` SpecMapping struct if needed (the JSON tag is `spec_id`)
4. Update any code that creates/updates ledger entries to use titles
5. Run tests to verify

## Acceptance Criteria

- [x] Ledger `spec_id` fields contain human-readable titles
- [x] Ledger keys remain spec IDs
- [x] All existing tests pass
- [x] `ff spec` commands still work correctly
- [x] `ff sync` still works correctly

## Verification

- Run `go test ./...` to ensure all tests pass
- Run `ff ship` to verify shipping gate works
- Manually inspect ledger to confirm titles are present

## What was done

- `engine/ledger.go`: Updated `SpecEntry` struct comment; `SyncFromSpecsDir` extracts title from `# heading` and sets `SpecID: title`
- `engine/cmd_spec.go`: Creation site changed to `SpecID: title`
- `engine/sync.go`: `syncSingleSpec` uses `spec.Title`; `promoteReviewSpecs` loop uses map key for frontmatter match and `SetSpecEntry`
- `engine/cmd_export.go`: Added `Title` field to `specFileMeta`, added `extractSpecTitle()` helper, import uses `spec.Title`
- `engine/issue_coordinator.go`: Creation site uses `spec.Title`
- `engine/archive.go`: Uses map key `specID` instead of `entry.SpecID` for frontmatter and heading reconstruction
- `.ff/forgefix_ledger.json`: Migrated 12 entries to titles, 11 kept as spec ID fallback (no spec file on disk)
- `assets/dashboard.png`: Added dashboard screenshot to README
- All tests pass (`go test ./...` — 4/4 packages)
- Commit: `3afd5ba` — `refactor: ledger spec_id field now holds spec title [SPEC-1783495360]`

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->