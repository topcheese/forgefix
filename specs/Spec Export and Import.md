---
spec_id: "SPEC-1783157283"
status: ship
repo_issue: 462
type: feature
version: "0.9.3"
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

1. Created `engine/cmd_export.go` with handleExport and handleImport methods
2. Added `case "export":` and `case "import":` to `command_dispatcher.go`
3. Export uses tar.gz format, supporting --spec/-s filter and --output/-o flags
4. Import detects duplicates (skips by default), uses --force to overwrite
5. Import auto-registers new specs in the ledger

## Acceptance Criteria

- [x] `ff export -o specs.tar.gz` creates export file
- [x] `ff export -s SPEC-1783157282 -o spec.tar.gz` exports single spec
- [x] `ff import specs.tar.gz` imports specs
- [x] Duplicate specs are detected and skipped without --force
- [x] `--force` flag overwrites existing spec files

## Verification

```bash
ff export -o backup.tar.gz           # Export all specs
ff export SPEC-1783157280 -o spec.md  # Export single spec
ff import backup.tar.gz              # Import specs
```
