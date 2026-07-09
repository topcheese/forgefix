---
spec_id: "SPEC-1783157282"
status: closed
repo_issue: 461
type: feature
version: "0.9.4"
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

1. Created `engine/cmd_config.go` with handleConfig method
2. Added `case "config":` to `command_dispatcher.go` (conditionally routes `config validate` to handler, other config subcommands to git passthrough)
3. Validates: github owner/repo/token, base_url format, pipeline configs, language configs
4. Reuses existing `LoadPipelineConfig` for config loading
5. Outputs errors to stderr with actionable messages and config file path

## Acceptance Criteria

- [x] `ff config validate` exits 0 for valid config
- [x] `ff config validate` reports missing required fields
- [x] `ff config validate` reports invalid field formats
- [x] Error messages are actionable and specific
- [x] `ff config` (without validate) still passes through to git

## Verification

```bash
ff config validate              # Validates current config
ff config validate --json       # JSON error output
```
