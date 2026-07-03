package engine

import "fmt"

// handleArchive archives all resolved/closed specs into a timestamped document.
func (d *CommandDispatcher) handleArchive(args []string) (CommandResult, error) {
	archiveName, count, err := ArchiveResolvedSpecs(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	if count == 0 {
		fmt.Fprintln(d.Stdout, "No resolved specs to archive.")
		return CommandResult{ExitCode: 0}, nil
	}
	fmt.Fprintf(d.Stdout, "Archived %d resolved specs to %s\n", count, archiveName)
	return CommandResult{ExitCode: 0}, nil
}
