---
spec_id: "SPEC-1783157282"
status: backlog
repo_issue: 461
type: feature
version: "v0.9.0"
root_cause: ""
resolution: ""
---
# Config Validation Command

## Objective

Add `ff config validate` command to validate the forgefix configuration file and report any issues before they cause problems during sync or ship.

## Requirements

1. `ff config validate` checks configuration file syntax
2. Validates required fields: github owner, repo, token
3. Validates optional fields: base_url format, label patterns
4. Reports specific errors with line numbers when possible
5. Exit code 0 for valid, non-zero for errors

## Implementation

1. Create `cmd_config.go` with validate subcommand
2. Reuse existing config parsing from config.go
3. Add validation functions for each field type
4. Output errors to stderr with helpful messages

## Acceptance Criteria

- [ ] `ff config validate` exits 0 for valid config
- [ ] `ff config validate` reports missing required fields
- [ ] `ff config validate` reports invalid field formats
- [ ] Error messages are actionable and specific

## Verification

```bash
ff config validate              # Validates current config
ff config validate --json       # JSON error output
```
