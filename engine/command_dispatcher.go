package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
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
	case "--kanban":
		return d.handleKanban(args)
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

// handleVersion prints version information to the configured stdout writer
// and checks for a newer release on the NAS Gitea when configured.
// The displayed version comes from the project DB (the canonical source),
// falling back to the compile-time const when no project context exists.
func (d *CommandDispatcher) handleVersion() CommandResult {
	version := CurrentVersion(d.ConfigDir)
	if vm := NewVersionManager(d.ConfigDir); vm != nil {
		if pv := vm.CurrentVersion(); pv != "0.0.0" {
			version = pv
		}
	}
	fmt.Fprintf(d.Stdout, "ForgeFix %s\n", version)
	checkAndPromptUpdate(d.ConfigDir, d.Stdout, d.Stderr, false)
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
	if semver.Compare(release.TagName, "v"+CurrentVersion(configDir)) <= 0 {
		return
	}
	fmt.Fprintf(stdout, "New version available: %s (current: %s)\n", release.TagName, CurrentVersion(configDir))
	if aiMode {
		return
	}
	fmt.Fprint(stdout, "Update now? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		return
	}
	runUpdate(configDir, coord, strings.TrimPrefix(release.TagName, "v"), stdout, stderr)
}

// runUpdate downloads the latest release binary and installs it.
func runUpdate(configDir string, coord *IssueCoordinator, version string, stdout, stderr io.Writer) {
	release, err := coord.LatestRelease()
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: %v\n", err)
		EmitAIError("UPDATE_RELEASE_FETCH_FAILED", err.Error())
		_ = QueueUpdateFailure(configDir, "UPDATE_RELEASE_FETCH_FAILED", err.Error())
		return
	}
	// Find asset matching current platform. Assets are named "ff", "ff-linux-amd64",
	// "ff-darwin-amd64", "ff-darwin-arm64", "ff-windows-amd64.exe".
	platformSuffix := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	var asset *ReleaseAsset
	for i, a := range release.Assets {
		if strings.HasSuffix(a.Name, platformSuffix) {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil {
		// Fall back to exact match for the platform-agnostic "ff" binary.
		for i, a := range release.Assets {
			if a.Name == "ff" {
				asset = &release.Assets[i]
				break
			}
		}
	}
	if asset == nil {
		errMsg := fmt.Sprintf("no asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(stderr, "Update failed: %s\n", errMsg)
		EmitAIError("UPDATE_NO_ASSET", errMsg)
		_ = QueueUpdateFailure(configDir, "UPDATE_NO_ASSET", errMsg)
		return
	}
	data, err := coord.DownloadReleaseAsset(asset.ID)
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: %v\n", err)
		EmitAIError("UPDATE_DOWNLOAD_FAILED", err.Error())
		_ = QueueUpdateFailure(configDir, "UPDATE_DOWNLOAD_FAILED", err.Error())
		return
	}
	// Write to a temp file, make executable, replace current binary
	tmpPath := filepath.Join(os.TempDir(), "ff-update")
	if err := os.WriteFile(tmpPath, data, 0755); err != nil {
		fmt.Fprintf(stderr, "Update failed: writing temp file: %v\n", err)
		EmitAIError("UPDATE_WRITE_TEMP_FAILED", err.Error())
		_ = QueueUpdateFailure(configDir, "UPDATE_WRITE_TEMP_FAILED", err.Error())
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "Update failed: finding executable: %v\n", err)
		EmitAIError("UPDATE_FIND_EXECUTABLE_FAILED", err.Error())
		_ = QueueUpdateFailure(configDir, "UPDATE_FIND_EXECUTABLE_FAILED", err.Error())
		return
	}
	backupPath := exe + ".bak"
	os.Rename(exe, backupPath) // ignore error if backup fails
	if err := os.Rename(tmpPath, exe); err != nil {
		os.Rename(backupPath, exe) // restore backup
		fmt.Fprintf(stderr, "Update failed: replacing binary: %v\n", err)
		EmitAIError("UPDATE_REPLACE_BINARY_FAILED", err.Error())
		_ = QueueUpdateFailure(configDir, "UPDATE_REPLACE_BINARY_FAILED", err.Error())
		return
	}
	fmt.Fprintf(stdout, "Updated to version %s.\n", version)
	// Propagate to all PATH locations
	bm := NewBinaryManager()
	bm.EnsureDev(configDir)
	bm.InstallGlobal()
}
