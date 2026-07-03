package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ForgeFix/engine"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting current directory: %v\n", err)
		os.Exit(1)
	}

	if err := engine.Bootstrap(wd); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap error: %v\n", err)
		os.Exit(1)
	}

	projectRoot := engine.FindProjectRoot(wd)
	if hasPending, _ := engine.HasPendingSyncFailures(projectRoot); hasPending {
		if err := promptRetrySyncFailures(projectRoot); err != nil {
			fmt.Fprintf(os.Stderr, "sync retry error: %v\n", err)
		}
	}

	for _, arg := range os.Args[1:] {
		if arg == "--install" {
			binDir, warning, err := engine.InstallGlobal()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if warning != "" {
				fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
			}
			fmt.Printf("Installed ff to %s\n", binDir)
			os.Exit(0)
		}
	}

	cmd := ""
	if len(os.Args) > 1 {
		cmd = strings.ToLower(os.Args[1])
	}

	skipConfigCheck := cmd == "help" || cmd == "--help" || cmd == "version" || cmd == "-v" || cmd == "sync" || cmd == "ship" || cmd == "archive" || cmd == "specs"

	if !skipConfigCheck {
		if _, found := findConfigFile(wd); !found {
			promptForInit(wd)
			os.Exit(0)
		}
	}

	if cmd != "version" && cmd != "help" && cmd != "--help" && cmd != "" {
		fmt.Fprintf(os.Stderr, "ForgeFix %s\n", engine.Version)
	}

	disp := engine.NewCommandDispatcher(projectRoot, wd, os.Stdout, os.Stderr)

	switch cmd {
	case "version", "-v", "help", "--help", "spec", "specs", "archive", "commit", "sync", "ship", "--install-shortcut":
		result, err := disp.Execute(cmd, os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "internal error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(result.ExitCode)
	default:
		// No subcommand or unknown — run test suite via dispatcher
		result, err := disp.Execute("", os.Args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "internal error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(result.ExitCode)
	}
}

func findConfigFile(wd string) (string, bool) {
	path, err := engine.FindAnyConfig(wd)
	if err != nil {
		return "", false
	}
	return filepath.Base(path), true
}

func promptForInit(wd string) bool {
	fmt.Print("ForgeFix configuration not found. Initialize? (y/N/q): ")
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "q" {
		fmt.Println("Aborted.")
		return false
	}
	if response == "y" || response == "yes" {
		target, err := engine.InitConfig(wd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error initializing config: %v\n", err)
			return false
		}
		fmt.Printf("created %s\n", target)

		if binDir, warning, err := engine.InstallGlobal(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: config created but binary install failed: %v\n", err)
			fmt.Fprintln(os.Stderr, "Run 'ff --install' later to install the ff command globally.")
		} else {
			fmt.Printf("Installed ff globally to %s\n", binDir)
			if warning != "" {
				fmt.Fprintln(os.Stderr, warning)
			}
		}
		return true
	}
	fmt.Println("OK, run 'ff' again when ready.")
	return false
}

func promptForDuplicateAction(newTitle, existingTitle, existingSpecID string, aiMode bool) string {
	if aiMode {
		return "link"
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n⚠ Duplicate detected!\n")
	fmt.Printf("  New:      %s\n", newTitle)
	fmt.Printf("  Existing: %s (%s)\n", existingTitle, existingSpecID)
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  1. Link to existing spec (add reference)\n")
	fmt.Printf("  2. Update existing spec\n")
	fmt.Printf("  3. Create new spec anyway (with [Dupe] suffix)\n")
	fmt.Printf("Choice [1-3]: ")

	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		switch input {
		case "1", "link", "l":
			return "link"
		case "2", "update", "u":
			return "update"
		case "3", "create", "c":
			return "create"
		default:
			fmt.Printf("Invalid choice. Enter 1, 2, or 3: ")
		}
	}
}

func updateExistingSpec(ledgerDir, existingSpecID, newTitle string) error {
	ledger, err := engine.LoadLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}
	entry := ledger.GetSpecEntry(existingSpecID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", existingSpecID)
	}
	entry.Status = "draft"
	ledger.SetSpecEntry(existingSpecID, entry)
	return engine.SaveLedger(ledger, ledgerDir)
}

var findLedgerDir = func(dir string) string {
	for {
		if found, err := engine.FindAnyConfig(dir); err == nil {
			return filepath.Dir(found)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

func promptRetrySyncFailures(configDir string) error {
	failures, err := engine.LoadSyncFailures(configDir)
	if err != nil {
		return err
	}
	if len(failures) == 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n⚠ %d background sync failure(s) detected:\n", len(failures))
	for i, f := range failures {
		specInfo := ""
		if f.SpecID != "" {
			specInfo = fmt.Sprintf(" (spec: %s)", f.SpecID)
		}
		fmt.Fprintf(os.Stderr, "  %d.%s %s\n", i+1, specInfo, f.Error)
	}

	fmt.Fprint(os.Stderr, "\nRetry failed syncs now? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" || input == "y" || input == "yes" {
		fmt.Fprintln(os.Stderr, "Retrying sync...")
		if err := engine.RunBackgroundSync(configDir, ""); err != nil {
			return fmt.Errorf("sync retry failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Sync retry completed.")
		if err := engine.ClearSyncFailures(configDir); err != nil {
			return fmt.Errorf("clearing failures: %w", err)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Sync retry skipped. Run 'ff sync' manually to retry.")
	}
	return nil
}
