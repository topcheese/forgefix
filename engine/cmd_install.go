package engine

import "fmt"

// handleInstallShortcut installs the ff binary globally.
func (d *CommandDispatcher) handleInstallShortcut(args []string) (CommandResult, error) {
	bm := NewBinaryManager()
	binDir, warning, err := bm.InstallGlobal()
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	fmt.Fprintf(d.Stdout, "ForgeFix binary installed to %s\n", binDir)
	if warning != "" {
		fmt.Fprintln(d.Stderr, warning)
	}
	return CommandResult{ExitCode: 0}, nil
}
