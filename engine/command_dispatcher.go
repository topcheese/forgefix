package engine

import (
	"io"
)

// CommandResult represents the outcome of executing a command.
// The caller (main.go) uses ExitCode to decide whether to os.Exit(0) or os.Exit(1).
type CommandResult struct {
	Message  string
	ExitCode int // 0 = success, 1 = error
}

// CommandDispatcher routes CLI subcommands to their handler methods.
// It holds the ambient context (working dir, config dir, IO writers) so that
// individual handlers remain focused on their subcommand's logic.
type CommandDispatcher struct {
	ConfigDir string
	WorkDir   string
	Stdout    io.Writer
	Stderr    io.Writer
}

// NewCommandDispatcher creates a dispatcher with the given context.
func NewCommandDispatcher(configDir, workDir string, stdout, stderr io.Writer) *CommandDispatcher {
	return &CommandDispatcher{
		ConfigDir: configDir,
		WorkDir:   workDir,
		Stdout:    stdout,
		Stderr:    stderr,
	}
}

// Execute routes the subcommand to its handler and returns the result.
// cmd is the subcommand string (e.g. "help", "version") and args are the
// remaining CLI tokens. Returns an error only for unexpected failures;
// command-level errors are embedded in CommandResult so the caller controls
// exit flow.
func (d *CommandDispatcher) Execute(cmd string, args []string) (CommandResult, error) {
	switch cmd {
	case "help", "--help":
		return d.handleHelp(), nil
	case "version", "-v", "--version":
		return d.handleVersion(), nil
	case "status":
		return d.handleStatus(args)
	case "archive":
		return d.handleArchive(args)
	case "specs":
		return d.handleListSpecs(args)
	case "ship":
		return d.handleShip(args)
	case "sync":
		return d.handleSync(args)
	case "spec":
		return d.handleSpec(args)
	case "commit":
		return d.handleCommit(args)
	case "--install-shortcut":
		return d.handleInstallShortcut(args)
	case "git":
		// ff git <cmd> <args> — explicit git passthrough
		if len(args) == 0 {
			return d.handleRun(cmd, args)
		}
		return d.handleGitPassthrough(args[0], args[1:])
	default:
		// Check if this is a known git command for transparent passthrough
		if GitPassthroughCommands[cmd] {
			return d.handleGitPassthrough(cmd, args)
		}
		// Empty cmd or unknown — run test suite (existing behavior)
		return d.handleRun(cmd, args)
	}
}

// handleHelp prints the full help text to the configured stdout writer.
func (d *CommandDispatcher) handleHelp() CommandResult {
	PrintHelp(d.Stdout)
	return CommandResult{ExitCode: 0}
}

// handleVersion prints version information to the configured stdout writer.
func (d *CommandDispatcher) handleVersion() CommandResult {
	PrintVersion(d.Stdout)
	return CommandResult{ExitCode: 0}
}
