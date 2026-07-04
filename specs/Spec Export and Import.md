---
spec_id: "SPEC-1783157283"
status: backlog
repo_issue: 462
type: feature
version: "v0.9.0"
root_cause: ""
resolution: ""
---
# Spec Export and Import

## Objective

Add `ff export` and `ff import` commands to allow transferring specs between projects or creating backups of spec state.

## Requirements

1. `ff export` exports all active specs to a tarball or directory
2. `ff export --spec SPEC-XXXXX` exports a single spec
3. `ff import` imports specs from an export file
4. Imports handle duplicate detection
5. Optionally export with or without ledger metadata

## Implementation

1. Create `cmd_export.go` and `cmd_import.go`
2. Use tarball format for multi-spec exports
3. Include spec files and optionally ledger entries
4. Handle version conflicts on import
5. Interactive prompts for duplicate handling (or `--force` flag)

## Acceptance Criteria

- [ ] `ff export -o specs.tar.gz` creates export file
- [ ] `ff export SPEC-1783157280` exports single spec
- [ ] `ff import specs.tar.gz` imports specs
- [ ] Duplicate specs are detected and handled

## Verification

```bash
ff export -o backup.tar.gz           # Export all specs
ff export SPEC-1783157280 -o spec.md  # Export single spec
ff import backup.tar.gz              # Import specs
```
