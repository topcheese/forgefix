---
spec_id: "SPEC-1783126960"
status: review
repo_issue: 444
version: "0.9.0"
type: feature
root_cause: ""
resolution: ""
---
# ff git Passthrough for Unsupported Commands

## Objective

Currently, ForgeFix only handles its own subcommands (`spec`, `commit`, `sync`, `ship`, `specs`, `archive`, `help`, `version`). Any other subcommand (e.g., `ff log`, `ff diff`, `ff branch`, `ff status`) falls through to the test runner with the flags treated as test arguments, which is confusing and breaks the muscle memory of users who expect `ff` to act as a git proxy.

Add an `ff git` passthrough mode (or direct passthrough for unrecognized `ff <cmd>` calls) that:
1. Detects unrecognized subcommands
2. Transpiles them to `git <cmd>`
3. Runs them with the ForgeFix TUI output wrapper
4. Returns the exit code transparently

## Requirements

1. **Direct passthrough** — `ff log`, `ff diff`, `ff branch`, `ff status`, `ff reset`, `ff rebase`, `ff merge` etc. should proxy to `git log`, `git diff`, `git branch` etc. with all arguments forwarded verbatim.

2. **`ff git` prefix** — `ff git <cmd>` should also work (e.g., `ff git log --oneline`) for explicit use.

3. **Transparent exit codes** — The passthrough must preserve git's exit code so shell scripts and CI pipelines work correctly.

4. **Passthrough detection** — If the subcommand is not a recognized ForgeFix command AND it matches a known git command (or simply isn't a flag), proxy to git rather than falling through to the test runner.

5. **Configuration opt-out** — Users should be able to disable passthrough via `<project>_ff.yaml`:
   ```yaml
   git_passthrough: false  # default: true
   ```

6. **No performance regression** — The passthrough check is O(1) — a simple lookup in a known-command set before falling through.

7. **TUI integration** — Output from the passthrough should display through the ForgeFix TUI when running in interactive mode, or as plain stdout/stderr in `--ai` mode.

## Implementation

### Phase 1: Passthrough Detection

In `command_dispatcher.go`, replace the `default` case with:

```go
func (d *CommandDispatcher) Execute(cmd string, args []string) (CommandResult, error) {
    switch {
    case isKnownForgeFixCommand(cmd):
        // existing routing...
    case isGitPassthroughCandidate(cmd):
        return d.handleGitPassthrough(cmd, args)
    default:
        return d.handleRun(cmd, args)
    }
}
```

### Phase 2: Git Command Set

Maintain a set of known git commands for passthrough detection:

```go
var gitPassthroughCommands = map[string]bool{
    "add": true, "bisect": true, "branch": true, "checkout": true,
    "cherry-pick": true, "clean": true, "clone": true, "commit": true,
    "config": true, "describe": true, "diff": true, "fetch": true,
    "format-patch": true, "grep": true, "init": true, "log": true,
    "maintenance": true, "merge": true, "mv": true, "notes": true,
    "pull": true, "push": true, "rebase": true, "reflog": true,
    "remote": true, "reset": true, "restore": true, "revert": true,
    "rm": true, "shortlog": true, "show": true, "sparse-checkout": true,
    "stash": true, "status": true, "submodule": true, "switch": true,
    "tag": true, "worktree": true,
}
```

### Phase 3: Passthrough Handler

```go
// handleGitPassthrough proxies unknown commands to git.
func (d *CommandDispatcher) handleGitPassthrough(cmd string, args []string) (CommandResult, error) {
    gitArgs := append([]string{cmd}, args...)
    c := exec.Command("git", gitArgs...)
    c.Stdout = d.Stdout
    c.Stderr = d.Stderr
    c.Stdin = os.Stdin

    err := c.Run()
    exitCode := 0
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        } else {
            fmt.Fprintf(d.Stderr, "git passthrough error: %v\n", err)
            exitCode = 1
        }
    }
    return CommandResult{ExitCode: exitCode}, nil
}
```

### Phase 4: Config Option

Add to `ffconfig.go`:

```go
type PipelineConfig struct {
    // ... existing fields
    GitPassthrough *bool `yaml:"git_passthrough,omitempty"`
}
```

And in the dispatcher, check the config before routing:

```go
func (d *CommandDispatcher) isGitPassthroughEnabled() bool {
    loaded, err := LoadPipelineConfig(d.ConfigDir)
    if err != nil || loaded.Config == nil || loaded.Config.GitPassthrough == nil {
        return true // default: enabled
    }
    return *loaded.Config.GitPassthrough
}
```

### Phase 5: `ff git` Prefix

Handle `ff git <cmd>` by detecting `cmd == "git"` and routing remaining args through the same passthrough:

```go
case "git":
    if len(args) == 0 {
        return d.handleRun(cmd, args) // "ff git" → test runner
    }
    return d.handleGitPassthrough(args[0], args[1:])
```

### File Changes

| File | Change |
|------|--------|
| `engine/command_dispatcher.go` | Add passthrough detection in `Execute()`; add `"git"` route |
| `engine/cmd_git.go` (new) | `handleGitPassthrough` implementation + git command set |
| `engine/cmd_git_test.go` (new) | Passthrough tests |
| `engine/ffconfig.go` | Optional `GitPassthrough` config field |
| `engine/config.go` | Update `PipelineConfig` struct |
| `main.go` | If needed for detection before dispatcher creation |

## Acceptance Criteria

1. [x] `ff log --oneline -5` outputs `git log --oneline -5`
2. [x] `ff status` outputs `git status` (short format)
3. [x] `ff branch` lists branches
4. [x] `ff git log` works as alias for `ff log`
5. [x] `ff commit` (a normal git commit) still works as a ForgeFix subcommand (not proxied)
6. [x] `ff diff --cached` shows staged diff
7. [x] Passthrough preserves exit codes (`ff log -999` → non-zero)
8. [x] Setting `git_passthrough: false` in config restores old behavior (unknown commands → test runner)
9. [x] `go build ./...` compiles without errors
10. [x] `go test ./...` passes without regressions

## Verification

```bash
# Build
cd /Users/james/work/forgefix && go build -o ff . && echo OK

# Full test suite
go test ./... -count=1

# Passthrough smoke tests
./ff log --oneline -5
./ff status --short
./ff branch
./ff diff --cached

# ForgeFix commands still work
./ff version
./ff help
./ff specs

# Exit code passthrough
test ! $(./ff log -999 2>/dev/null; echo $?) -eq 0

# Config disable (create override config in test project)
mkdir -p /tmp/fftest && cd /tmp/fftest
echo 'git_passthrough: false' > fftest_ff.yaml
../forgefix/ff log --oneline 2>&1 | grep -q 'not a git repository' || echo "passthrough disabled"
```
