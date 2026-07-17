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
	specName, specBody := parseSpecPositional(args)

	// --delete handling (unchanged)
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
		fmt.Fprintln(d.Stderr, "error: spec command requires a name or spec ID")
		fmt.Fprintln(d.Stderr, "usage: ff spec <name|spec_id> [--status <status>] [--delete]")
		return CommandResult{ExitCode: 1}, nil
	}

	// --status flag: set status on existing spec
	if flags.SpecStatus != "" {
		if !isValidSpecStatus(flags.SpecStatus) {
			valid := []string{"backlog", "draft", "in-progress", "review", "ship", "closed"}
			fmt.Fprintf(d.Stderr, "error: invalid status %q — valid values: %s\n", flags.SpecStatus, strings.Join(valid, ", "))
			return CommandResult{ExitCode: 1}, nil
		}
		return d.setSpecStatus(specName, flags.SpecStatus)
	}

	// Check if spec exists (by spec_id) — if so, show interactive menu
	specDir := filepath.Join(d.ConfigDir, "specs")
	specFile, err := findSpecFileByID(specDir, specName)
	if err == nil {
		ledger, lErr := LoadLedger(d.ConfigDir)
		if lErr != nil {
			fmt.Fprintf(d.Stderr, "error: loading ledger: %v\n", lErr)
			return CommandResult{ExitCode: 1}, nil
		}
		entry := ledger.GetSpecEntry(specName)
		return d.promptAdvancement(specName, specFile, entry, ledger)
	}

	// Spec doesn't exist — create new (existing logic)
	if err := createSpec(d.ConfigDir, specName, specBody, d, flags); err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	return CommandResult{ExitCode: 0}, nil
}

// setSpecStatus sets the status of an existing spec by ID.
func (d *CommandDispatcher) setSpecStatus(specID, status string) (CommandResult, error) {
	specDir := filepath.Join(d.ConfigDir, "specs")
	specFile, err := findSpecFileByID(specDir, specID)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: spec %s not found\n", specID)
		return CommandResult{ExitCode: 1}, nil
	}

	currentStatus, err := readSpecFileStatus(specFile)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: reading spec status: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	terminal := map[string]bool{"ship": true, "closed": true, "resolved": true, "deprecated": true}
	if terminal[currentStatus] {
		fmt.Fprintf(d.Stderr, "error: spec %s is %s — cannot change status\n", specID, currentStatus)
		return CommandResult{ExitCode: 1}, nil
	}

	if err := UpdateSpecFileStatus(specFile, status); err != nil {
		fmt.Fprintf(d.Stderr, "error: updating spec file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	ledger, lErr := LoadLedger(d.ConfigDir)
	if lErr == nil {
		entry := ledger.GetSpecEntry(specID)
		if entry != nil {
			entry.Status = status
			ledger.SetSpecEntry(specID, entry)
			_ = SaveLedger(ledger, d.ConfigDir)
		}
	}

	fmt.Fprintf(d.Stdout, "✓ Spec %s set to %s\n", specID, status)
	queueSpecBackgroundSync(d.ConfigDir, specID, d.Stdout, d.Stderr)
	return CommandResult{ExitCode: 0}, nil
}

// promptAdvancement shows an interactive menu for advancing an existing spec's status.
func (d *CommandDispatcher) promptAdvancement(specID, specFile string, entry *SpecEntry, ledger *LedgerEngine) (CommandResult, error) {
	if entry != nil {
		terminal := map[string]bool{"ship": true, "closed": true, "resolved": true, "deprecated": true}
		if terminal[entry.Status] {
			fmt.Fprintf(d.Stderr, "error: spec %s is %s — cannot change status\n", specID, entry.Status)
			return CommandResult{ExitCode: 1}, nil
		}
	}

	currentStatus := "unknown"
	if entry != nil {
		currentStatus = entry.Status
	}
	fmt.Fprintf(d.Stderr, "Spec %s (status: %s). Select advancement:\n", specID, currentStatus)
	fmt.Fprintln(d.Stderr, "  1. in-progress")
	fmt.Fprintln(d.Stderr, "  2. review")
	fmt.Fprintln(d.Stderr, "  3. ship")
	fmt.Fprintln(d.Stderr, "  4. close")
	fmt.Fprintln(d.Stderr, "  5. delete")
	fmt.Fprintln(d.Stderr, "  q. quit")
	fmt.Fprint(d.Stderr, "Choice: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var newStatus string
	switch input {
	case "1":
		newStatus = "in-progress"
	case "2":
		newStatus = "review"
	case "3":
		newStatus = "ship"
	case "4":
		newStatus = "closed"
	case "5":
		if err := deleteSpec(d.ConfigDir, specID); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "✓ Deleted spec %s\n", specID)
		return CommandResult{ExitCode: 0}, nil
	case "q", "":
		return CommandResult{ExitCode: 0}, nil
	default:
		fmt.Fprintln(d.Stderr, "Invalid choice")
		return CommandResult{ExitCode: 1}, nil
	}

	if err := UpdateSpecFileStatus(specFile, newStatus); err != nil {
		fmt.Fprintf(d.Stderr, "error: updating spec file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	if entry != nil {
		entry.Status = newStatus
		ledger.SetSpecEntry(specID, entry)
		_ = SaveLedger(ledger, d.ConfigDir)
	}

	fmt.Fprintf(d.Stdout, "✓ Spec %s advanced to %s\n", specID, newStatus)
	queueSpecBackgroundSync(d.ConfigDir, specID, d.Stdout, d.Stderr)
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

// writeSpecFromTemplate renders the spec template with the given fields and writes
// it to specs/<name>.md. It does not perform duplicate detection
// or body prompting — callers handle those. Returns the generated
// spec ID and file path.
func writeSpecFromTemplate(configDir, name, title, body, specType, specVersion, specRootCause string) (string, string, error) {
	templatePath := filepath.Join(configDir, "templates", "spec_template.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", "", fmt.Errorf("reading template: %w", err)
	}

	specID := fmt.Sprintf("SPEC-%d", time.Now().Unix())
	now := time.Now().Format("2006-01-02")

	tpl := string(templateContent)
	tpl = strings.ReplaceAll(tpl, `spec_id: ""`, fmt.Sprintf(`spec_id: "%s"`, specID))
	tpl = strings.ReplaceAll(tpl, "created: YYYY-MM-DD", fmt.Sprintf("created: %s", now))
	if specType != "" {
		tpl = strings.ReplaceAll(tpl, "type: feature", "type: "+specType)
	}
	// Update the version line (match any version: line).
	lines := strings.Split(tpl, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "version:") {
			lines[i] = fmt.Sprintf("version: \"%s\"", specVersion)
		}
	}
	tpl = strings.Join(lines, "\n")

	if specRootCause != "" {
		tpl = strings.ReplaceAll(tpl, `root_cause: ""`, fmt.Sprintf(`root_cause: "%s"`, specRootCause))
	}

	parts := strings.SplitN(tpl, "---", 3)
	if len(parts) < 3 {
		return "", "", fmt.Errorf("malformed template: expected frontmatter delimiters")
	}
	frontmatter := parts[1]

	content := fmt.Sprintf("---%s---\n%s\n", frontmatter, body)

	specDir := filepath.Join(configDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating specs directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.md", name)
	filePath := filepath.Join(specDir, fileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("writing spec file: %w", err)
	}
	return specID, filePath, nil
}

// createSpec creates a new spec file from the template.
func createSpec(configDir, name, bodyContent string, d *CommandDispatcher, flags CLIArgs) error {
	title := sanitizeSpecTitle(name)

	origSpecID, origTitle, isDup := FindDuplicateSpec(configDir, title)
	if isDup {
		action := promptForDuplicateAction(title, origTitle, origSpecID, flags.AIMode)
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

	// Tier 1: --type is required in --ai mode (FR-6)
	if flags.AIMode && flags.SpecType == "" {
		return fmt.Errorf("--type is required in --ai mode: valid values are feature, bug, refactor")
	}
	if flags.AIMode && flags.SpecType != "" && !isValidSpecType(flags.SpecType) {
		return fmt.Errorf("invalid --type %q: valid values are feature, bug, refactor", flags.SpecType)
	}

	// Resolve version from DB (not the compile-time const).
	specVersion := CurrentVersion(configDir)
	if flags.SpecVersion != "" {
		specVersion = flags.SpecVersion
	}

	body := strings.TrimSpace(bodyContent)
	if body == "" && !flags.AIMode {
		body = promptForSpecBody(title)
	}
	if body == "" {
		if flags.AIMode {
			return fmt.Errorf("spec body is required in --ai mode: ff spec --ai <title> <body>")
		}
		return fmt.Errorf("spec body cannot be empty")
	}

	specID, filePath, err := writeSpecFromTemplate(configDir, name, title, body, flags.SpecType, specVersion, flags.SpecRootCause)
	if err != nil {
		return err
	}

	// Post-creation exact spec_id collision check (SPEC-1784101811)
	// This catches the case where two specs end up with the same spec_id
	// (e.g., due to timestamp collision or manual manipulation).
	existingSpecID, existingTitle, existingPath, idCollision := FindSpecByID(configDir, specID)
	if idCollision && existingPath != filePath {
		// Hard conflict: two files with the same spec_id corrupts the ledger
		// In AI mode, auto-link to existing spec (consistent with promptForDuplicateAction behavior)
		// In interactive mode, prompt user
		if flags.AIMode {
			fmt.Fprintf(d.Stdout, "Exact spec_id collision detected: %s already exists as %s. Auto-linking.\n", specID, existingTitle)
			// Remove the newly created file since we're linking to existing
			_ = os.Remove(filePath)
			return nil
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("\n⚠ Exact spec_id collision detected!\n")
		fmt.Printf("  New spec:      %s (%s)\n", title, specID)
		fmt.Printf("  Existing spec: %s (%s)\n", existingTitle, existingSpecID)
		fmt.Printf("\nOptions:\n")
		fmt.Printf("  1. Link to existing spec (remove new file)\n")
		fmt.Printf("  2. Keep both (not recommended - corrupts ledger)\n")
		fmt.Printf("Choice [1-2]: ")

		for {
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			switch input {
			case "1", "link", "l":
				fmt.Fprintf(d.Stdout, "Linking to existing spec %s (%s). Removing new file.\n", existingSpecID, existingTitle)
				_ = os.Remove(filePath)
				return nil
			case "2", "keep", "k":
				fmt.Fprintf(d.Stdout, "Keeping both specs. WARNING: This will corrupt the ledger.\n")
				// Continue with ledger update (will overwrite)
			default:
				fmt.Printf("Invalid choice. Enter 1 or 2: ")
			}
		}
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

	if flags.AIMode {
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

// parseSpecPositional extracts the spec name and body from args, correctly
// skipping flag values (e.g., --type feature) so the value "feature" is not
// treated as a positional arg.
func parseSpecPositional(args []string) (name, body string) {
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		// Flags that consume the next arg — skip both the flag and its value.
		switch arg {
		case "--message", "-m", "--failure-decay", "-d", "--run", "-r",
			"--spec", "-s", "--type", "-t", "--ver", "--objective", "-o",
			"--requirements", "--req", "--acceptance", "-a",
			"--body", "--root-cause", "--search", "--task-title", "--status":
			i++ // skip the flag's value
		}
	}
	if len(positional) > 0 {
		name = positional[0]
	}
	if len(positional) > 1 {
		body = positional[1]
	}
	return
}

// sanitizeSpecTitle normalizes a raw spec name (e.g. from `ff spec --ai "fix --ai null
// pointer"`) into a clean issue title. It replaces runs of dashes with spaces and
// collapses any resulting multiple spaces into one, so phrases like "--ai" no
// longer produce double/garbled spacing. The result is Title-cased.
func sanitizeSpecTitle(name string) string {
	replaced := strings.ReplaceAll(name, "-", " ")
	fields := strings.Fields(replaced)
	title := strings.Join(fields, " ")
	title = strings.Title(strings.ToLower(title))
	return title
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

// promptForSpecBody prompts the user interactively for the spec body.
// First asks to paste the full body; if Enter is pressed, prompts section
// by section for Objective, Requirements, Implementation, Acceptance
// Criteria, and Verification.
func promptForSpecBody(title string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, "Paste full spec body (or press Enter for section-by-section): ")
	fullBody, _ := reader.ReadString('\n')
	fullBody = strings.TrimSpace(fullBody)
	if fullBody != "" {
		return fmt.Sprintf("# %s\n\n%s", title, fullBody)
	}

	var sections []string
	type prompt struct {
		heading string
		label   string
	}
	prompts := []prompt{
		{"Objective", "Objective"},
		{"Requirements", "Requirements"},
		{"Implementation", "Implementation"},
		{"Acceptance Criteria", "Acceptance Criteria"},
		{"Verification", "Verification"},
	}

	for _, p := range prompts {
		fmt.Fprintf(os.Stderr, "%s (or press Enter to skip): ", p.label)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text != "" {
			sections = append(sections, fmt.Sprintf("## %s\n\n%s", p.heading, text))
		} else {
			sections = append(sections, fmt.Sprintf("## %s\n", p.heading))
		}
	}

	body := fmt.Sprintf("# %s\n\n", title)
	if len(sections) > 0 {
		body += strings.Join(sections, "\n\n") + "\n"
	}
	return strings.TrimSpace(body)
}

// isValidSpecType checks whether the given type is one of the known spec types.
func isValidSpecType(t string) bool {
	switch t {
	case "feature", "bug", "refactor":
		return true
	default:
		return false
	}
}
