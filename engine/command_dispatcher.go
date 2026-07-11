package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	case "config":
		if len(args) > 0 && args[0] == "validate" {
			return d.handleConfig(args)
		}
		if GitPassthroughCommands["config"] {
			return d.handleGitPassthrough("config", args)
		}
		return d.handleRun("config", args)
	case "export":
		return d.handleExport(args)
	case "import":
		return d.handleImport(args)
	case "sync":
		return d.handleSync(args)
	case "spec":
		return d.handleSpec(args)
	case "backlog":
		return d.handleBacklog(args)
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

// checkAndPromptUpdate queries the configured Gitea for the latest release.
// If a newer version exists it prints the info and prompts the user to update.
// In aiMode the prompt is skipped — only the latest version is printed.
func checkAndPromptUpdate(configDir string, stdout, stderr io.Writer, aiMode bool) {
	loaded, err := LoadPipelineConfig(configDir)
	if err != nil {
		return
	}
	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" {
		return
	}
	coord := NewCoordinatorFromConfig(loaded.Config, configDir, aiMode)
	if coord == nil {
		return
	}
	release, err := coord.LatestRelease()
	if err != nil {
		return
	}
	if release.TagName == "" {
		return
	}
	// Strip leading "v" for comparison
	latest := strings.TrimPrefix(release.TagName, "v")
	current := Version
	if latest <= current {
		return
	}
	fmt.Fprintf(stdout, "New version available: %s (current: %s)\n", release.TagName, current)
	if aiMode {
		return
	}
	fmt.Fprint(stdout, "Update now? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		return
	}
	runUpdate(configDir, coord, latest, stdout, stderr)
}

// runUpdate downloads the latest release binary and installs it.
func runUpdate(configDir string, coord *IssueCoordinator, version string, stdout, stderr io.Writer) {
	release, err := coord.LatestRelease()
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: %v\n", err)
		return
	}
	// Find asset matching current platform
	assetName := fmt.Sprintf("forgefix-%s", runtime.GOOS)
	var asset *ReleaseAsset
	for i, a := range release.Assets {
		if strings.Contains(a.Name, assetName) {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil {
		fmt.Fprintf(stderr, "Update failed: no asset found for %s\n", assetName)
		return
	}
	data, err := coord.DownloadReleaseAsset(asset.ID)
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: %v\n", err)
		return
	}
	// Write to a temp file, make executable, replace current binary
	tmpPath := filepath.Join(os.TempDir(), "ff-update")
	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
		fmt.Fprintf(stderr, "Update failed: writing temp file: %v\n", err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: finding executable: %v\n", err)
		return
	}
	backupPath := exe + ".bak"
	os.Rename(exe, backupPath) // ignore error if backup fails
	if err := os.Rename(tmpPath, exe); err != nil {
		os.Rename(backupPath, exe) // restore backup
		fmt.Fprintf(stderr, "Update failed: replacing binary: %v\n", err)
		return
	}
	fmt.Fprintf(stdout, "Updated to version %s.\n", version)
	// Propagate to all PATH locations
	bm := NewBinaryManager()
	bm.EnsureDev(configDir)
	bm.InstallGlobal()
}
