# Version Fix Action Plan

## Objective
Fix the version inconsistency where `ff archive` prints the hardcoded const (`0.9.0`) while `ff -v` prints the DB version (`0.9.6`).

## Root Cause
The code is split between two sources:
- `engine.Version` (hardcoded const in flags.go: `Version = "0.9.0"`)
- `VersionManager.CurrentVersion()` (reads from DB/ledger, returns `0.9.6`)

Locations using `engine.Version` (const):
- `main.go:62` - per-command banner
- `flags.go:156` - PrintHelp
- `flags.go:159` - PrintVersion
- `command_dispatcher.go:111` - handleVersion() (but fixes to use VM)
- `command_dispatcher.go:156` - checkAndPromptUpdate() 
- `cmd_spec.go:158` - writeSpecFromTemplate parameter
- `execute.go:283` - writeSpecFromTemplate parameter
- Probably other locations

## Solution
1. Create `engine.CurrentVersion()` - unified version source using VersionManager
2. Replace ALL direct `engine.Version` usage with `engine.CurrentVersion()`
3. Remove `Version` const from engine/flags.go
4. Update checkAndPromptUpdate to compare using CurrentVersion()
5. Ensure all version displays use the same canonical source

## Implementation Files
- engine/flags.go: remove Version const, add CurrentVersion() function
- engine/command_dispatcher.go: use CurrentVersion in handleVersion and checkAndPromptUpdate
- engine/cmd_spec.go: use CurrentVersion instead of Version for specVersion
- engine/execute.go: use CurrentVersion instead of Version for specVersion  
- main.go: use CurrentVersion instead of Version for banner
- Possibly engine/ledger.go - remove Version field
- Possibly other files that use engine.Version

## Testing Strategy
1. Ensure `ff -v` still works (should still print version)
2. Ensure `ff archive` banner uses same version as `ff -v`
3. Ensure checkAndPromptUpdate compares against correct version
4. Ensure spec creation uses correct version from DB
5. Run existing tests to verify no regressions
