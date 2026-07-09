package engine

import (
	"fmt"
	"strings"
)

// handleSync synchronizes local spec status to the remote issue tracker.
func (d *CommandDispatcher) handleSync(args []string) (CommandResult, error) {
	flags := ParseFlags(args)
	specID := flags.SpecID

	// Extract optional target path from args (first non-flag token)
	targetPath := ""
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			targetPath = arg
			break
		}
	}

	loaded, err := LoadPipelineConfig(targetPath)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
		fmt.Fprintln(d.Stdout, "No GitHub/Repo credentials configured — skipping remote issue sync.")
		return CommandResult{ExitCode: 0}, nil
	}

	fmt.Fprintln(d.Stdout, "Syncing workspace tokens and endpoints system-wide...")
	if loaded.Config.GitHub.BaseURL != "" {
		fmt.Fprintf(d.Stdout, "Configuring Git NAS Gateway -> %s\n", loaded.Config.GitHub.BaseURL)
	}

	bm := NewBinaryManager()
	if err := bm.EnsureDev(loaded.ConfigDir); err != nil {
		fmt.Fprintf(d.Stderr, "warning: binary bootstrap failed: %v\n", err)
	}
	binDir, installWarning, installErr := bm.InstallGlobal()
	if installErr != nil {
		fmt.Fprintf(d.Stderr, "warning: global binary update failed: %v\n", installErr)
	} else {
		fmt.Fprintf(d.Stdout, "Updated ff binary globally to %s\n", binDir)
		if installWarning != "" {
			fmt.Fprintln(d.Stderr, installWarning)
		}
	}

	if err := RunBackgroundSync(loaded.ConfigDir, specID, flags.AIMode); err != nil {
		fmt.Fprintf(d.Stderr, "sync failed: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	fmt.Fprintln(d.Stdout, "Sync completed successfully.")
	if err := ClearSyncFailures(loaded.ConfigDir); err != nil {
		fmt.Fprintf(d.Stderr, "warning: failed to clear sync failures: %v\n", err)
	}

	return CommandResult{ExitCode: 0}, nil
}
