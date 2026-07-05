---
spec_id: "SPEC-1783225332"
status: in-progress
repo_issue: ""
type: refactor
version: "v0.8.1"
root_cause: "Bootstrap(), EnsureDevBinary(), and InstallGlobal() each had independent copy implementations for copying the ff binary. copyBinary in shortcut.go was a separate implementation. Three copy paths with the same logic."
resolution: "Created binary_manager.go with BinaryManager type. EnsureDev() replaces both Bootstrap() and EnsureDevBinary() (same logic consolidated). InstallGlobal() replaces old shortcut.go version. copyFile() helper shared by all paths. Package-level convenience wrappers in ffdir.go and shortcut.go delegate to BinaryManager."
---

# Consolidate Overlapping Binary Management Into BinaryManager

## Objective

Consolidate three separate binary copy paths (Bootstrap, EnsureDevBinary, InstallGlobal) and the standalone copyBinary function into a single `BinaryManager` type with shared `copyFile` logic.

## Requirements

- New `BinaryManager` type in `binary_manager.go`
- `EnsureDev(configDir)` replaces `Bootstrap()` + `EnsureDevBinary()` (consolidated copy logic)
- `InstallGlobal()` replaces the stand-alone `InstallGlobal()` in `shortcut.go`
- `copyFile(src, dst, mode)` internal helper shared by all copy paths
- `copyBinary()` / `preferLocalBinary()` remain as thin package-level wrappers for backward compat
- `getSystemBinDir()`, `ensureInPath()`, `DetectShellProfile()` stay in `shortcut.go` (PATH utilities)
- All existing callers updated: `cmd_install.go`, `cmd_sync.go`, `ffdir.go` (MigrateToFF)
- All existing tests pass

## Implementation

1. Create `engine/binary_manager.go` with:
   - `BinaryManager` struct (no fields — stateless)
   - `NewBinaryManager()` constructor
   - `EnsureDev(configDir)` — binary lookup + stale check + copy to `.ff/bin/ff`
   - `InstallGlobal()` — global binary install + PATH update + shell profile warn
   - `copyBinary(src, binDir)` — copies binary as "ff" and "FF" (or "ff.exe"/"FF.exe" on Windows)
   - `preferLocalBinary()` — finds local `./ff` if distinct from running binary
   - `copyFile(src, dst, mode)` — generic file copy helper
2. Update `ffdir.go`:
   - `Bootstrap()` → thin wrapper: `NewBinaryManager().EnsureDev(configDir)`
   - `EnsureDevBinary()` → thin wrapper: `NewBinaryManager().EnsureDev(configDir)`
   - `MigrateToFF()` → calls `NewBinaryManager().EnsureDev(configDir)`
3. Update `shortcut.go`:
   - `InstallGlobal()` → thin wrapper: `NewBinaryManager().InstallGlobal()`
   - `copyBinary()` → thin wrapper: `NewBinaryManager().copyBinary()`
   - `preferLocalBinary()` → thin wrapper: `NewBinaryManager().preferLocalBinary()`
4. Update `cmd_install.go` → uses `NewBinaryManager().InstallGlobal()` directly
5. Update `cmd_sync.go` → uses `NewBinaryManager()` for both EnsureDev and InstallGlobal

## Acceptance Criteria

- [x] BinaryManager in binary_manager.go with EnsureDev and InstallGlobal methods
- [x] EnsureDev replaces Bootstrap + EnsureDevBinary (same logic, no duplication)
- [x] copyFile helper shared by all binary copy paths
- [x] copyBinary / preferLocalBinary are thin wrappers delegating to BinaryManager
- [x] All callers updated to use BinaryManager directly where possible
- [x] All 340+ tests pass

## Verification

```bash
go build ./...
go test ./... -count=1 -timeout 120s
go test ./engine/ -run "TestCopyBinary|TestInstallGlobal|TestPreferLocalBinary|TestEnsureInPath|TestGetSystemBinDir" -v
```
