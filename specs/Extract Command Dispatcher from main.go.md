---
spec_id: "SPEC-1783118155"
status: review
repo_issue: ""
type: refactor
root_cause: "main() function is ~226 lines with cognitive complexity 128 — monolithic switch-based command dispatcher violates SRP"
resolution: ""
---
# Extract Command Dispatcher from main.go

## Objective

Decouple the command routing and execution logic from `main.go` into a dedicated `CommandDispatcher` in the `engine/` package. Currently, `main()` contains a ~226-line switch-based command dispatcher with high cognitive complexity (128), mixing CLI parsing, routing, error handling, and side-effect orchestration in a single monolithic function. This violates the Single Responsibility Principle and makes the entry point untestable in isolation.

After extraction, `main()` should be a thin entry point that:
1. Bootstraps the environment
2. Creates a `CommandDispatcher`
3. Delegates to the dispatcher
4. Handles the result (print or exit)

The command implementations themselves shall migrate to separate files under `engine/commands/`, each with its own file and test.

## Requirements

1. **Command Dispatcher** — A new `engine/commands` package (or `engine/command_dispatcher.go`) that owns the subcommand routing table and execution. The dispatcher must:
   - Accept parsed `CLIArgs` and any ambient context (e.g., config dir, working dir)
   - Route subcommand strings (`"spec"`, `"commit"`, `"sync"`, `"ship"`, etc.) to dedicated handler functions
   - Return structured results (success value + error) rather than calling `os.Exit` directly
   - Support a `Run()` method that is callable from both CLI and test code

2. **Command Handlers** — Each subcommand (`spec`, `specs`, `commit`, `sync`, `ship`, `version`, `archive`, `help`, default/test) shall have its own handler function in a dedicated file. Handlers must:
   - Accept only the data they need (config dir, flags, IO writers)
   - Return `(result any, err error)` — never call `os.Exit` internally
   - Be independently unit-testable without spawning a full CLI process

3. **IO Abstraction** — All handlers must accept `io.Writer` for stdout/stderr rather than writing to `os.Stdout`/`os.Stderr` directly. This enables:
   - Table-driven tests that capture output buffers
   - Future support for JSON/structured output mode without code duplication
   - Elimination of global state dependency in test assertions

4. **Error Handling** — The dispatcher must centralize error formatting and exit-code decisions. Handlers return errors; the dispatcher decides whether to print, exit, or propagate.

5. **No Regressions** — All existing behavior must be preserved. The refactoring must not change:
   - Flag parsing semantics (already in `engine/flags.go`)
   - Output formatting (help text, version, table layouts)
   - Side-effect ordering (background sync, housekeeping drains, ledger saves)
   - Exit codes (0 on success, 1 on error)

## Implementation

### Phase 1: Dispatcher Contract & Migration (engine/command_dispatcher.go)

```go
package engine

type CommandResult struct {
    Message string
    ExitCode int  // 0 = success, 1 = error (for main() to call os.Exit)
}

type CommandDispatcher struct {
    ConfigDir string
    WorkDir   string
    Stdout    io.Writer
    Stderr    io.Writer
    Flags     CLIArgs
}

func NewCommandDispatcher(configDir, workDir string, stdout, stderr io.Writer) *CommandDispatcher {
    return &CommandDispatcher{
        ConfigDir: configDir,
        WorkDir:   workDir,
        Stdout:    stdout,
        Stderr:    stderr,
    }
}

// Execute routes subcommand "cmd" (e.g. "spec", "commit") and returns the result.
// Returns an error only for unexpected/internal failures; command-level errors
// are embedded in CommandResult so main() can control exit flow.
func (d *CommandDispatcher) Execute(cmd string, args []string) (CommandResult, error)
```

### Phase 2: Individual Command Files (engine/commands/)

Each command gets its own file:

| File | Handles | Source Functions |
|------|---------|-----------------|
| `commands/spec.go` | `ff spec <name>`, `ff spec --delete` | `createSpec`, `deleteSpec`, `createNewBugSpec`, duplicate handling |
| `commands/commit.go` | `ff commit` | `runCommit`, `promptForSpecSelection`, `selectSpecWithScanner`, `selectSpecFromList`, `groupSpecsByType` |
| `commands/sync.go` | `ff sync` | Inline sync orchestration (~50 lines in `main()`) |
| `commands/ship.go` | `ff ship` | Inline ship orchestration (~10 lines in `main()`) |
| `commands/list.go` | `ff specs` | `runListSpecs` |
| `commands/archive.go` | `ff archive` | Thin wrapper over `engine.ArchiveResolvedSpecs` |
| `commands/version.go` | `ff version` | Delegates to `engine.PrintVersion` |
| `commands/help.go` | `ff help` | Delegates to `engine.PrintHelp` |
| `commands/run.go` | default (test runner) | Inline or via existing `engine.ExecuteSuite` |

Each file exports a single function like `func HandleSpec(d *CommandDispatcher, args []string) CommandResult` that encapsulates all logic for that subcommand.

### Phase 3: Slim main.go

After migration, `main()` should reduce to approximately:

```go
func main() {
    wd, err := os.Getwd()
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

    if err := engine.Bootstrap(wd); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

    projectRoot := engine.FindProjectRoot(wd)
    if hasPending, _ := engine.HasPendingSyncFailures(projectRoot); hasPending {
        promptRetrySyncFailures(projectRoot)  // stays until extracted
    }

    // Handle --install flag early
    for _, arg := range os.Args[1:] {
        if arg == "--install" {
            binDir, warning, err := engine.InstallGlobal()
            if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
            if warning != "" { fmt.Fprintln(os.Stderr, warning) }
            fmt.Printf("Installed ff to %s\n", binDir)
            os.Exit(0)
        }
    }

    cmd := ""
    targetPath := ""
    if len(os.Args) > 1 { cmd = strings.ToLower(os.Args[1]) }
    if len(os.Args) > 2 && (cmd == "sync" || cmd == "ship") && !strings.HasPrefix(os.Args[2], "-") {
        targetPath = os.Args[2]
    }

    if cmd != "help" && cmd != "--help" && cmd != "version" && cmd != "-v" && cmd != "sync" && cmd != "ship" && cmd != "archive" && cmd != "specs" {
        if _, found := findConfigFile(wd); !found {
            promptForInit(wd)
            os.Exit(0)
        }
    }

    if cmd != "version" && cmd != "help" && cmd != "--help" && cmd != "" {
        fmt.Fprintf(os.Stderr, "ForgeFix %s\n", engine.Version)
    }

    dispatcher := engine.NewCommandDispatcher(projectRoot, wd, os.Stdout, os.Stderr)
    result, err := dispatcher.Execute(cmd, os.Args[2:])
    if err != nil {
        fmt.Fprintf(os.Stderr, "internal error: %v\n", err)
        os.Exit(1)
    }
    if result.Message != "" {
        fmt.Println(result.Message)
    }
    os.Exit(result.ExitCode)
}
```

Remaining helper functions in `main.go` (e.g., `promptRetrySyncFailures`, `findConfigFile`, `promptForInit`) are kept in place for this pass unless they are purely command-supporting, in which case they migrate with their command.

### Migration Order (per-file, test-first)

1. `commands/help.go` + `commands/help_test.go` — simplest, establishes the pattern
2. `commands/version.go` + `commands/version_test.go` — trivially testable
3. `commands/archive.go` + `commands/archive_test.go` — thin delegation
4. `commands/list.go` + `commands/list_test.go` — involves table rendering
5. `commands/ship.go` + `commands/ship_test.go` — involves ledger state
6. `commands/sync.go` + `commands/sync_test.go` — involves issue coordinator
7. `commands/spec.go` + `commands/spec_test.go` — most complex, involves spec CRUD
8. `commands/commit.go` + `commands/commit_test.go` — most complex, involves git, ledger, interactive prompts
9. `commands/run.go` + `commands/run_test.go` — test runner delegation
10. `command_dispatcher.go` + `command_dispatcher_test.go` — the routing table, wired last
11. `main.go` — slimmed down to ~60 lines

### Interaction with Existing Architecture (DDD / Clean Architecture)

- **Domain layer** (`engine/ledger.go`, `engine/issue_coordinator.go`, `engine/ffconfig.go`) — untouched. These remain pure domain services.
- **Infrastructure** (`engine/discovery.go`, `engine/ffdir.go`, `engine/sync.go`) — untouched.
- **Application layer** (these new command handlers) — orchestrate domain services based on CLI input. This is the proper place for `os.Exit`-free orchestration.
- **Entry point** (`main.go`) — the outermost adapter in Clean Architecture terms: parse CLI, bootstrap, delegate, exit.

## Acceptance Criteria

1. [x] `main()` is reduced from ~226 lines to ≤80 lines
2. [x] `main.go` never calls `os.Exit` from inside a switch case — only at the top-level control flow
3. [x] Every subcommand handler is independently unit-testable via `CommandDispatcher.Execute()`
4. [x] Handlers accept `io.Writer` for output — no direct writes to `os.Stdout`/`os.Stderr`
5. [x] No handler function calls `os.Exit` — all errors are returned as `error` or `CommandResult.ExitCode`
6. [x] All existing CLI behavior is preserved (help text, version output, sync flow, ship gate, commit flow, spec CRUD, archive, test runner)
7. [x] `go build ./...` compiles without errors
8. [x] `go test ./...` passes without regressions

## Verification

1. **Build verification:**
   ```bash
   go build -o ff . && echo OK
   ```

2. **Unit tests (all packages):**
   ```bash
   go test ./... -v -count=1 2>&1 | tail -20
   ```

3. **Functional smoke tests:**
   ```bash
   ./ff version                           # → ForgeFix 0.8.0
   ./ff help                              # → usage text
   ./ff --help                            # → usage text (alternate form)
   ./ff --ai                              # → JSON test output (no crash)
   ./ff specs --all                       # → spec table (no crash)
   ./ff archive                           # → archive or "no resolved specs"
   ```

4. **Command handler unit tests (new pattern):**
   ```go
   func TestHelpCommandWritesToWriter(t *testing.T) {
       var buf strings.Builder
       d := engine.NewCommandDispatcher("/tmp", "/tmp", &buf, &buf)
       result, err := d.Execute("help", nil)
       assert.NoError(t, err)
       assert.Contains(t, buf.String(), "ForgeFix")
       assert.Equal(t, 0, result.ExitCode)
   }
   ```

5. **Regression baseline:** Ledger and spec files must remain unmodified by the refactoring. Compare ledger checksum before/after:
   ```bash
   sha256sum .ff/forgefix_ledger.json   # verify no change
   ```
