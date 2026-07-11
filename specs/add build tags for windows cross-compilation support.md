---
spec_id: "SPEC-1783790935"
status: draft
repo_issue: ""
type: bug
version: "0.9.0"
root_cause: "engine files use syscall.SIGWINCH, Setpgid, Kill, Setsid which are Unix-only — GOOS=windows cross-compile fails"
resolution: ""
---
# Add Build Tags For Windows Cross-compilation Support

## Objective

`ff ship --ai` currently fails to build the Windows binary because several
source files use `syscall.SIGWINCH`, `Setpgid`, `Kill`, and `Setsid` which
are not available on Windows. Add Go build constraints (`//go:build !windows`)
to exclude Unix-specific code and create Windows stubs where needed.

## Implementation

- `engine/execute.go`: SIGWINCH signal handling — wrap in `//go:build !windows`.
- `engine/runner.go`: `Setpgid` and `syscall.Kill` — wrap in `//go:build !windows`.
  Create a `runner_windows.go` stub with no-op implementations.
- `engine/sync.go`: `Setsid` in `SpawnBackgroundSync` — wrap in `//go:build !windows`.
  Create `sync_windows.go` with an alternative implementation.
- Keep the cross-compile step in `uploadPlatformBinaries` as-is — it already
  sets `GOOS=windows GOARCH=amd64`; the build tags will allow it to succeed.

## Verification

- `GOOS=windows GOARCH=amd64 go build ./...` exits 0.
- `ff ship --ai` produces and uploads `ff-windows-amd64.exe`.
- `go test ./... -count=1` still passes on darwin/linux.
