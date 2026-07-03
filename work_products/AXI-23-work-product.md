# AXI-23 Work Product: Forge Fix Not Installing On Init

## Deliverable Files

### Workspace Files Modified
- `main.go` — Restored `promptForInit`, added auto-install after config creation, config check uses `wd`; refactored `promptForInit` to return `bool` instead of calling `os.Exit` directly
- `main_test.go` — Added `TestPromptForInitInstallsBinaryOnYes` test
- `engine/ffdir.go` — `FindProjectRoot` matches only `{dirbasename}_ff.yaml`; added `hasConfigFileNamed`, `FindMatchingConfig`

### Key Changes

1. **`main.go:282-320`** — `promptForInit` restored and made testable:
   - Prompts user before creating config (`y/N/q`)
   - On `y`/`yes`: calls `InitConfig(wd)` then `InstallGlobal()` to install `ff` to `~/.local/bin/`
   - Returns `bool` (true = initialized) instead of calling `os.Exit` directly
   - `os.Exit(0)` moved to caller (`main()`) for testability

2. **`main.go:96-101`** — Config check uses `wd` (cwd) instead of `projectRoot`:
   - Prevents `/Users/james/james_ff.yaml` from falsely satisfying the config check
   - Config is created in the actual working directory, not the resolved project root

3. **`main_test.go`** — New test `TestPromptForInitInstallsBinaryOnYes`:
   - Creates temp project with `go.mod` and test binary
   - Mocks stdin with "y" response
   - Verifies `promptForInit` creates the `{dirname}_ff.yaml` config file
   - Verifies `InstallGlobal()` installs the binary to `~/.local/bin/ff`

4. **`engine/ffdir.go:86-140`** — `FindProjectRoot` directory-name matching:
   - Walk-up now checks for `{dirbasename}_ff.yaml` (e.g., `forgefix_ff.yaml`), not any `_ff.yaml`
   - Prevents the 96K-line `/Users/james/james_ff.yaml` from being picked up as a project root
   - Same guard applied to `.ff/` + yaml fallback heuristic

## Verification Results

### Build
```
go build ./... => OK (no output)
```

### Tests
```
ok  ForgeFix              0.271s
ok  ForgeFix/engine       5.968s
ok  ForgeFix/engine/housekeeper  0.013s
ok  ForgeFix/tests        0.985s
```

All 4 test packages pass. The new `TestPromptForInitInstallsBinaryOnYes` test specifically verifies that the `promptForInit` function creates the config AND installs the binary globally when the user answers "y".

### `ff --ai` Verification
Confirmed `ff --ai` produces valid JSON output. The "zero test streams" issue is pre-existing (same result from committed code) and unrelated to the install-on-init fix.

## Status: COMPLETE

### Final Disposition
- **Issue Status**: Done
- **Work Mode**: Standard
- **Agent**: Coder - Jimmy
- **Completion Date**: 2026-07-03
