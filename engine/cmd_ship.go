package engine

import (
	"fmt"
	"strings"
)

// handleShip runs the shipping gate: verifies all specs are in ship-status,
// pushes commits, and transitions shipped specs to closed.
func (d *CommandDispatcher) handleShip(args []string) (CommandResult, error) {
	flags := ParseFlags(args)

	// DANGEROUS-COMMAND GUARD: ff ship pushes commits and tags to a remote and
	// creates a release. Require explicit confirmation; in AI mode there is no
	// interactive input, so confirmPrompt returns false and the command is
	// refused by default — preventing an agent from silently shipping
	// unreviewed code to a remote.
	if !confirmPrompt("⚠ `ff ship` pushes commits and tags to a remote and creates a release. Continue") {
		fmt.Fprintln(d.Stdout, "ship: aborted — not confirmed.")
		return CommandResult{ExitCode: 1}, nil
	}

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

	ShipReconciliation(loaded.Config, loaded.ConfigDir, flags.AIMode)
	fmt.Fprintln(d.Stdout, "ship: running final verification and release pipeline…")
	return CommandResult{ExitCode: 0}, nil
}
