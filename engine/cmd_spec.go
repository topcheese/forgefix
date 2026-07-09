package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// handleSpec creates, deletes, or manages spec files.
func (d *CommandDispatcher) handleSpec(args []string) (CommandResult, error) {
	flags := ParseFlags(args)

	specName := ""
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			specName = arg
			break
		}
	}

	if flags.Delete {
		if specName == "" {
			fmt.Fprintln(d.Stderr, "error: --delete requires a spec ID")
			fmt.Fprintln(d.Stderr, "usage: ff spec --delete <spec_id>")
			return CommandResult{ExitCode: 1}, nil
		}
		if err := deleteSpec(d.ConfigDir, specName); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		return CommandResult{ExitCode: 0}, nil
	}

	if specName == "" {
		fmt.Fprintln(d.Stderr, "error: spec command requires a name")
		fmt.Fprintln(d.Stderr, "usage: ff spec <name>")
		return CommandResult{ExitCode: 1}, nil
	}

	if err := createSpec(d.ConfigDir, specName, d, flags.AIMode); err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	return CommandResult{ExitCode: 0}, nil
}

// SpecConfigDir resolves the config directory for spec operations.
// It walks up from workDir to find the project root with a config file.
func SpecConfigDir(workDir string) string {
	dir := workDir
	for {
		if found, err := FindAnyConfig(dir); err == nil {
			return filepath.Dir(found)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// createSpec creates a new spec file from the template.
func createSpec(configDir, name string, d *CommandDispatcher, aiMode bool) error {
	templatePath := filepath.Join(configDir, "templates", "spec_template.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	specID := fmt.Sprintf("SPEC-%d", time.Now().Unix())
	now := time.Now().Format("2006-01-02")

	title := strings.ReplaceAll(name, "-", " ")
	title = strings.Title(strings.ToLower(title))

	origSpecID, origTitle, isDup := FindDuplicateSpec(configDir, title)
	if isDup {
		action := promptForDuplicateAction(title, origTitle, origSpecID, aiMode)
		switch action {
		case "link":
			fmt.Fprintf(d.Stdout, "Linking to existing spec %s (%s).\n", origSpecID, origTitle)
			return nil
		case "update":
			fmt.Fprintf(d.Stdout, "Updating existing spec %s (%s).\n", origSpecID, origTitle)
			return updateExistingSpec(configDir, origSpecID, title)
		case "create":
			title += " [Dupe]"
			fmt.Fprintf(d.Stdout, "Creating new spec with [Dupe] suffix.\n")
		}
	}

	content := string(templateContent)
	content = strings.ReplaceAll(content, `spec_id: ""`, fmt.Sprintf(`spec_id: "%s"`, specID))
	content = strings.ReplaceAll(content, "# [Title]", fmt.Sprintf("# %s", title))
	content = strings.ReplaceAll(content, "created: YYYY-MM-DD", fmt.Sprintf("created: %s", now))

	if isDup {
		ref := fmt.Sprintf("\n\n> This spec has been identified as a duplicate of `%s`.", origSpecID)
		content += ref
	}

	specDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return fmt.Errorf("creating specs directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.md", name)
	filePath := filepath.Join(specDir, fileName)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing spec file: %w", err)
	}

	fmt.Fprintf(d.Stdout, "Created spec: %s\n", filePath)
	ledger, lerr := LoadLedger(configDir)
	if lerr == nil {
		entry := &SpecEntry{
			SpecID:        title,
			RepoIssueID:   0,
			Status:        "draft",
			LinkedCommits: []string{},
		}
		ledger.SetSpecEntry(specID, entry)
		_ = SaveLedger(ledger, configDir)
	}

	// Queue sync operation and spawn background sync for remote issue creation
	loaded, loadErr := LoadPipelineConfig(configDir)
	if loadErr == nil && loaded.Config != nil {
		if err := QueueSyncSpec(loaded.ConfigDir, specID); err != nil {
			fmt.Fprintf(d.Stderr, "warning: failed to queue spec sync: %v\n", err)
		} else {
			fmt.Fprintf(d.Stdout, "Queued sync for spec %s\n", specID)
		}

		if loaded.Config.AutoIssueManagement {
			if err := SpawnBackgroundSync(loaded.ConfigDir, specID); err != nil {
				fmt.Fprintf(d.Stderr, "warning: failed to spawn background sync: %v\n", err)
			} else {
				fmt.Fprintln(d.Stdout, "Triggered background sync for remote issue creation")
			}
		}
	}

	if aiMode {
		return nil
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// deleteSpec removes a spec from the ledger, filesystem, and remote.
func deleteSpec(configDir, specID string) error {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}

	entry := ledger.GetSpecEntry(specID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", specID)
	}

	repoIssueID, err := ledger.DeleteSpec(specID, configDir)
	if err != nil {
		return fmt.Errorf("deleting spec: %w", err)
	}

	fmt.Printf("Deleted spec %s\n", specID)

	loaded, err := LoadPipelineConfig(configDir)
	if err == nil && loaded.Config != nil && repoIssueID > 0 {
		if err := QueueDeleteIssue(loaded.ConfigDir, specID, repoIssueID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to queue delete issue: %v\n", err)
		} else {
			fmt.Printf("Queued delete_issue for spec %s (issue #%d)\n", specID, repoIssueID)
		}

		if loaded.Config.AutoIssueManagement {
			if err := SpawnBackgroundSync(loaded.ConfigDir, specID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			} else {
				fmt.Println("Triggered background sync for remote reconciliation")
			}
		}
	}

	return nil
}

// updateExistingSpec resets a spec's status to draft.
func updateExistingSpec(configDir, existingSpecID, newTitle string) error {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}
	entry := ledger.GetSpecEntry(existingSpecID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", existingSpecID)
	}
	entry.Status = "draft"
	ledger.SetSpecEntry(existingSpecID, entry)
	return SaveLedger(ledger, configDir)
}

// promptForDuplicateAction asks the user how to handle a duplicate spec.
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
