package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"ForgeFix/engine"

	"github.com/fatih/color"
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
	targetPath := ""
	if len(os.Args) > 1 {
		cmd = strings.ToLower(os.Args[1])
	}
	if len(os.Args) > 2 && (cmd == "sync" || cmd == "ship") && !strings.HasPrefix(os.Args[2], "-") {
		targetPath = os.Args[2]
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
	case "spec":
		flags := engine.ParseFlags(os.Args[2:])
		specName := ""
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") {
				specName = arg
				break
			}
		}
		if flags.Delete {
			if specName == "" {
				fmt.Fprintln(os.Stderr, "error: --delete requires a spec ID")
				fmt.Fprintln(os.Stderr, "usage: ff spec --delete <spec_id>")
				os.Exit(1)
			}
			if err := deleteSpec(projectRoot, specName); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		if specName == "" {
			fmt.Fprintln(os.Stderr, "error: spec command requires a name")
			fmt.Fprintln(os.Stderr, "usage: ff spec <name>")
			os.Exit(1)
		}
		if err := createSpec(projectRoot, specName, flags.AIMode); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "specs":
		flags := engine.ParseFlags(os.Args[2:])
		if err := runListSpecs(projectRoot, flags.All); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case "archive":
		archiveName, count, err := engine.ArchiveResolvedSpecs(projectRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if count == 0 {
			fmt.Println("No resolved specs to archive.")
			os.Exit(0)
		}
		fmt.Printf("Archived %d resolved specs to %s\n", count, archiveName)
		os.Exit(0)
	case "commit":
		flags := engine.ParseFlags(os.Args[2:])
		msg := flags.Message
		if msg == "" {
			msg = extractMessageFromArgs(os.Args[2:])
		}
		if err := runCommit(projectRoot, msg, flags.SpecID, flags.SpecType, flags.SpecVersion); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		ledgerDir := findLedgerDir(projectRoot)
		if err := engine.DrainHousekeepingQueueFromConfig(ledgerDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: housekeeping drain failed: %v\n", err)
		}

		if flags.SpecID != "" {
			if err := engine.SpawnBackgroundSync(projectRoot, flags.SpecID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			}
		}
		os.Exit(0)
	case "sync":
		flags := engine.ParseFlags(os.Args[2:])
		specID := flags.SpecID

		loaded, err := engine.LoadPipelineConfig(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
			fmt.Println("No GitHub/Repo credentials configured — skipping remote issue sync.")
			os.Exit(0)
		}

		fmt.Println("Syncing workspace tokens and endpoints system-wide...")
		if loaded.Config.GitHub.BaseURL != "" {
			fmt.Printf("Configuring Git NAS Gateway -> %s\n", loaded.Config.GitHub.BaseURL)
		}

		if err := engine.RunBackgroundSync(loaded.ConfigDir, specID); err != nil {
			fmt.Fprintf(os.Stderr, "sync failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sync completed successfully.")
		if err := engine.ClearSyncFailures(loaded.ConfigDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clear sync failures: %v\n", err)
		}
		os.Exit(0)
	case "ship":
		flags := engine.ParseFlags(os.Args[2:])
		loaded, err := engine.LoadPipelineConfig(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		engine.ShipReconciliation(loaded.Config, loaded.ConfigDir, flags.AIMode)
		fmt.Println("ship: running final verification and release pipeline…")
		os.Exit(0)
	case "version", "-v":
		result, _ := disp.Execute("version", nil)
		os.Exit(result.ExitCode)
	case "help", "--help":
		result, _ := disp.Execute("help", nil)
		os.Exit(result.ExitCode)
	case "--install-shortcut":
		binDir, warning, err := engine.InstallGlobal()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ForgeFix binary installed to %s\n", binDir)
		if warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
		os.Exit(0)
	case "":
		// No subcommand - run test suite
	default:
		// Unknown subcommand - treat as flags for test suite
	}

	flags := engine.ParseFlags(os.Args[1:])

	if flags.Help {
		result, _ := disp.Execute("help", nil)
		os.Exit(result.ExitCode)
	}
	if flags.Version {
		result, _ := disp.Execute("version", nil)
		os.Exit(result.ExitCode)
	}

	loaded, err := engine.LoadPipelineConfig(targetPath)
	if err != nil {
		if flags.AIMode {
			engine.EmitAIError("CONFIG_LOAD_FAILURE", err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if flags.FailureDecay > 0 {
		loaded.Config.FailureDecaySeconds = flags.FailureDecay
	}
	if flags.RunTest != "" {
		for i := range loaded.Config.Pipelines {
			loaded.Config.Pipelines[i].Command.Args = []string{"-run", fmt.Sprintf("^%s$", flags.RunTest), "./..."}
		}
	}
	engine.ExecuteSuite(loaded.Config, loaded.ConfigDir, flags.AIMode, false)
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

func createSpec(projectRoot, name string, aiMode bool) error {
	templatePath := filepath.Join(projectRoot, "templates", "spec_template.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	specID := fmt.Sprintf("SPEC-%d", time.Now().Unix())
	now := time.Now().Format("2006-01-02")

	title := strings.ReplaceAll(name, "-", " ")
	title = strings.Title(strings.ToLower(title))

	ledgerDir := findLedgerDir(projectRoot)

	origSpecID, origTitle, isDup := engine.FindDuplicateSpec(ledgerDir, title)
	if isDup {
		action := promptForDuplicateAction(title, origTitle, origSpecID, aiMode)
		switch action {
		case "link":
			fmt.Printf("Linking to existing spec %s (%s).\n", origSpecID, origTitle)
			return nil
		case "update":
			fmt.Printf("Updating existing spec %s (%s).\n", origSpecID, origTitle)
			return updateExistingSpec(ledgerDir, origSpecID, title)
		case "create":
			title += " [Dupe]"
			fmt.Printf("Creating new spec with [Dupe] suffix.\n")
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

	specDir := filepath.Join(projectRoot, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return fmt.Errorf("creating specs directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.md", name)
	filePath := filepath.Join(specDir, fileName)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing spec file: %w", err)
	}

	fmt.Printf("Created spec: %s\n", filePath)
	ledger, lerr := engine.LoadLedger(ledgerDir)
	if lerr == nil {
		entry := &engine.SpecEntry{
			SpecID:        specID,
			RepoIssueID:   0,
			Status:        "draft",
			LinkedCommits: []string{},
		}
		ledger.SetSpecEntry(specID, entry)
		_ = engine.SaveLedger(ledger, ledgerDir)
	}

	// Queue sync operation and spawn background sync for remote issue creation
	loaded, loadErr := engine.LoadPipelineConfig(projectRoot)
	if loadErr == nil && loaded.Config != nil {
		if err := engine.QueueSyncSpec(loaded.ConfigDir, specID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to queue spec sync: %v\n", err)
		} else {
			fmt.Printf("Queued sync for spec %s\n", specID)
		}

		if loaded.Config.AutoIssueManagement {
			if err := engine.SpawnBackgroundSync(loaded.ConfigDir, specID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			} else {
				fmt.Println("Triggered background sync for remote issue creation")
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

func deleteSpec(projectRoot, specID string) error {
	ledgerDir := findLedgerDir(projectRoot)
	ledger, err := engine.LoadLedger(ledgerDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}

	entry := ledger.GetSpecEntry(specID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", specID)
	}

	repoIssueID, err := ledger.DeleteSpec(specID, ledgerDir)
	if err != nil {
		return fmt.Errorf("deleting spec: %w", err)
	}

	fmt.Printf("Deleted spec %s\n", specID)

	loaded, err := engine.LoadPipelineConfig(projectRoot)
	if err == nil && loaded.Config != nil && repoIssueID > 0 {
		if err := engine.QueueDeleteIssue(loaded.ConfigDir, specID, repoIssueID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to queue delete issue: %v\n", err)
		} else {
			fmt.Printf("Queued delete_issue for spec %s (issue #%d)\n", specID, repoIssueID)
		}

		if loaded.Config.AutoIssueManagement {
			if err := engine.SpawnBackgroundSync(loaded.ConfigDir, specID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			} else {
				fmt.Println("Triggered background sync for remote reconciliation")
			}
		}
	}

	return nil
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

func findGitRoot(dir string) (string, error) {
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("not a git repository")
}

func runCommit(wd, msg, flagSpecID, specType, specVersion string) error {
	gitRoot, err := findGitRoot(wd)
	if err != nil {
		return err
	}

	specID := flagSpecID
	if specID == "" {
		specID = extractSpecID(msg)
	}
	if specID == "" {
		var err error
		specID, err = promptForSpecSelection(wd)
		if err != nil {
			return err
		}
	}

	var commitMsg string
	if specID != "" {
		specs, err := loadActiveSpecs(wd)
		if err != nil {
			return fmt.Errorf("loading specs: %w", err)
		}

		if !specExists(specs, specID) {
			return fmt.Errorf("spec %s not found in active specs", specID)
		}

		specDir := filepath.Join(wd, "specs")
		specFile, err := findSpecFileByID(specDir, specID)
		if err == nil && (specType != "" || specVersion != "") {
			ledgerDir := findLedgerDir(wd)
			wc := engine.LoadWorkflowConfig(ledgerDir)

			if specType != "" {
				if wc != nil && wc.CategoryLabelFor("type", specType) == "" {
					fmt.Fprintf(os.Stderr, "warning: type %q not found in label_categories.type defaults\n", specType)
				}
			}
			if specVersion != "" {
				if wc != nil && wc.CategoryLabelFor("version", specVersion) == "" {
					fmt.Fprintf(os.Stderr, "warning: version %q not found in label_categories.version defaults\n", specVersion)
				}
			}

			if err := updateSpecFileTypeVersion(specFile, specType, specVersion); err != nil {
				return fmt.Errorf("updating spec metadata: %w", err)
			}
			fmt.Printf("Updated spec %s: type=%q version=%q\n", specID, specType, specVersion)
		}

		if msg == "" {
			fmt.Print("Commit message (or q to quit): ")
			_, _ = fmt.Scanln(&msg)
			msg = strings.TrimSpace(msg)
			if strings.ToLower(msg) == "q" {
				return fmt.Errorf("aborted by user")
			}
			if msg == "" {
				return fmt.Errorf("empty commit message")
			}
		}

		commitMsg = fmt.Sprintf("feat: [%s] %s", specID, msg)
	} else {
		if msg == "" {
			fmt.Print("Commit message (or q to quit): ")
			_, _ = fmt.Scanln(&msg)
			msg = strings.TrimSpace(msg)
			if strings.ToLower(msg) == "q" {
				return fmt.Errorf("aborted by user")
			}
			if msg == "" {
				return fmt.Errorf("empty commit message")
			}
		}
		commitMsg = msg
	}

	commitHash, err := engine.AutoStageAndCommit(gitRoot, commitMsg)
	if err != nil {
		return err
	}

	if specID != "" {
		ledgerDir := findLedgerDir(wd)
		ledger, err := engine.LoadLedger(ledgerDir)
		if err != nil {
			return fmt.Errorf("loading ledger: %w", err)
		}
		if entry := ledger.GetSpecEntry(specID); entry != nil {
			entry.LinkedCommits = append(entry.LinkedCommits, commitHash)
			entry.Status = "in-progress"
			ledger.SetSpecEntry(specID, entry)
			if err := engine.SaveLedger(ledger, ledgerDir); err != nil {
				return fmt.Errorf("saving ledger: %w", err)
			}
		}
	}

	return nil
}

func findSpecFileByID(specDir, specID string) (string, error) {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.HasPrefix(content, "---") {
			continue
		}
		parts := strings.SplitN(content, "---", 3)
		if len(parts) < 3 {
			continue
		}
		if strings.Contains(parts[1], fmt.Sprintf(`spec_id: "%s"`, specID)) ||
			strings.Contains(parts[1], fmt.Sprintf("spec_id: %s", specID)) {
			return path, nil
		}
	}
	return "", fmt.Errorf("spec file not found for %s", specID)
}

func updateSpecFileTypeVersion(filePath, specType, specVersion string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)
	lines := strings.Split(content, "\n")

	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if !inFrontmatter {
			continue
		}
		if specType != "" && strings.HasPrefix(trimmed, "type:") {
			lines[i] = fmt.Sprintf("type: %s", specType)
		} else if specVersion != "" && strings.HasPrefix(trimmed, "version:") {
			lines[i] = fmt.Sprintf("version: %s", specVersion)
		}
	}

	// If the field didn't exist, insert before repo_issue or at end of frontmatter
	if specType != "" {
		hasType := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "type:") {
				hasType = true
				break
			}
		}
		if !hasType {
			lines = insertFrontmatterLine(lines, fmt.Sprintf("type: %s", specType))
		}
	}
	if specVersion != "" {
		hasVersion := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "version:") {
				hasVersion = true
				break
			}
		}
		if !hasVersion {
			lines = insertFrontmatterLine(lines, fmt.Sprintf("version: %s", specVersion))
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

func insertFrontmatterLine(lines []string, newLine string) []string {
	// Insert before repo_issue: if present, otherwise before last ---
	insertAt := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && i > 0 {
			insertAt = i
			break
		}
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "repo_issue:") {
			insertAt = i
			break
		}
	}
	if insertAt < 0 {
		insertAt = len(lines)
	}
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertAt]...)
	result = append(result, newLine)
	result = append(result, lines[insertAt:]...)
	return result
}

type SpecInfo struct {
	SpecID string
	Title  string
	Status string
	Type   string
}

func loadActiveSpecs(wd string) ([]SpecInfo, error) {
	specDir := filepath.Join(wd, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var specs []SpecInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFileForCommit(filePath)
		if err != nil {
			continue
		}
		if spec.Status != "resolved" && spec.Status != "deprecated" && spec.Status != "" {
			specs = append(specs, SpecInfo{
				SpecID: spec.SpecID,
				Title:  spec.Title,
				Status: spec.Status,
				Type:   spec.Type,
			})
		}
	}
	return specs, nil
}

func SelectSpec(wd string, allowNew bool) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	return selectSpecWithScanner(wd, allowNew, scanner)
}

func selectSpecWithScanner(wd string, allowNew bool, scanner *bufio.Scanner) (string, error) {
	ledgerDir := findLedgerDir(wd)
	ledger, err := engine.LoadLedger(ledgerDir)
	if err != nil {
		return "", fmt.Errorf("loading ledger: %w", err)
	}

	entries, err := ledger.ListSpecs(false)
	if err != nil {
		return "", fmt.Errorf("listing specs: %w", err)
	}

	if len(entries) == 0 && allowNew {
		fmt.Println("No active specs found. Creating a new bug spec...")
		return createNewBugSpec(scanner, wd, ledgerDir, ledger)
	}

	specsByType := groupSpecsByType(entries)

	categories := []string{"Feature", "Bug", "Refactor", "All"}
	typeMap := map[string]string{
		"Feature":  "feature",
		"Bug":      "bug",
		"Refactor": "refactor",
		"All":      "",
	}

	fmt.Println("Select spec category:")
	for i, cat := range categories {
		var count int
		if cat == "All" {
			count = len(entries)
		} else {
			count = len(specsByType[typeMap[cat]])
		}
		fmt.Printf("  [%d] %s (%d specs)\n", i+1, cat, count)
	}
	fmt.Println("  [q] Quit / Abort")
	fmt.Println("  [Enter] Skip / No Spec")

	for {
		fmt.Print("Select category: ")
		if !scanner.Scan() {
			return "", fmt.Errorf("reading input: %w", scanner.Err())
		}
		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "q" {
			os.Exit(0)
		}

		if input == "" {
			return "", nil
		}

		catIdx, err := strconv.Atoi(input)
		if err != nil || catIdx < 1 || catIdx > len(categories) {
			fmt.Println("Invalid selection")
			continue
		}

		selectedCategory := categories[catIdx-1]
		var filtered []*engine.SpecEntry
		if selectedCategory == "All" {
			filtered = entries
		} else {
			filtered = specsByType[typeMap[selectedCategory]]
		}

		if len(filtered) == 0 {
			fmt.Printf("No specs in category %s\n", selectedCategory)
			continue
		}

		return selectSpecFromList(filtered, scanner, allowNew, wd, ledgerDir, ledger)
	}
}

func groupSpecsByType(entries []*engine.SpecEntry) map[string][]*engine.SpecEntry {
	result := map[string][]*engine.SpecEntry{
		"feature":  {},
		"bug":      {},
		"refactor": {},
	}
	for _, e := range entries {
		specType := strings.ToLower(e.Type)
		if specType == "" {
			specType = "feature"
		}
		if _, ok := result[specType]; ok {
			result[specType] = append(result[specType], e)
		} else {
			result["feature"] = append(result["feature"], e)
		}
	}
	return result
}

func selectSpecFromList(specs []*engine.SpecEntry, scanner *bufio.Scanner, allowNew bool, wd, ledgerDir string, ledger *engine.LedgerEngine) (string, error) {
	fmt.Printf("\nSelect spec (or choose an option):\n")
	if allowNew {
		fmt.Println("  [0] New Bug")
	}
	fmt.Println("  [b] Back to categories")
	fmt.Println("  [q] Quit / Abort")
	fmt.Println("  [Enter] Skip / No Spec")
	for i, e := range specs {
		statusColor := ""
		if e.Status == "in-progress" {
			statusColor = "\033[33m"
		} else if e.Status == "ready" {
			statusColor = "\033[32m"
		} else if e.Status == "draft" {
			statusColor = "\033[36m"
		}
		resetColor := "\033[0m"
		fmt.Printf("  [%d] %s %s%s%s (%s)\n", i+1, e.SpecID, statusColor, e.Status, resetColor, e.Status)
	}

	for {
		fmt.Print("Select option: ")
		if !scanner.Scan() {
			return "", fmt.Errorf("reading input: %w", scanner.Err())
		}
		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "q" {
			os.Exit(0)
		}

		if input == "" {
			return "", nil
		}

		if input == "b" {
			return "", fmt.Errorf("back")
		}

		if input == "0" && allowNew {
			return createNewBugSpec(scanner, wd, ledgerDir, ledger)
		}

		idx, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid selection")
			continue
		}

		if idx < 1 || idx > len(specs) {
			fmt.Println("Invalid selection")
			continue
		}

		return specs[idx-1].SpecID, nil
	}
}

func createNewBugSpec(scanner *bufio.Scanner, wd, ledgerDir string, ledger *engine.LedgerEngine) (string, error) {
	fmt.Print("New bug title: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("reading title: %w", scanner.Err())
	}
	title := strings.TrimSpace(scanner.Text())
	if title == "" {
		fmt.Println("Title cannot be empty")
		return createNewBugSpec(scanner, wd, ledgerDir, ledger)
	}

	newSpecID := fmt.Sprintf("SPEC-%d-BUG", time.Now().Unix())
	now := time.Now().Format("2006-01-02")

	origSpecID, origTitle, isDup := engine.FindDuplicateSpec(ledgerDir, title)
	if isDup {
		action := promptForDuplicateAction(title, origTitle, origSpecID, false)
		switch action {
		case "link":
			fmt.Printf("Linking to existing spec %s (%s).\n", origSpecID, origTitle)
			return origSpecID, nil
		case "update":
			fmt.Printf("Updating existing spec %s (%s).\n", origSpecID, origTitle)
			if err := updateExistingSpec(ledgerDir, origSpecID, title); err != nil {
				return "", err
			}
			return origSpecID, nil
		case "create":
			title += " [Dupe]"
			fmt.Printf("Creating new spec with [Dupe] suffix.\n")
		}
	}

	specDir := filepath.Join(wd, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		return "", fmt.Errorf("creating specs directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.md", strings.ReplaceAll(title, " ", "-"))
	filePath := filepath.Join(specDir, fileName)

	content := fmt.Sprintf(`---
spec_id: "%s"
status: draft
type: bug
repo_issue: ""
created: %s
---
# %s

## Goal

## Technical Requirements

## Acceptance Criteria
`, newSpecID, now, title)

	if isDup {
		ref := fmt.Sprintf("\n\n> This spec has been identified as a duplicate of `%s`.", origSpecID)
		content += ref
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing spec file: %w", err)
	}

	entry := &engine.SpecEntry{
		SpecID:        newSpecID,
		RepoIssueID:   0,
		Status:        "draft",
		LinkedCommits: []string{},
	}
	ledger.SetSpecEntry(newSpecID, entry)
	if err := engine.SaveLedger(ledger, ledgerDir); err != nil {
		return "", fmt.Errorf("saving ledger: %w", err)
	}

	// Queue sync operation and spawn background sync for remote issue creation
	loaded, loadErr := engine.LoadPipelineConfig(wd)
	if loadErr == nil && loaded.Config != nil {
		if err := engine.QueueSyncSpec(loaded.ConfigDir, newSpecID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to queue spec sync: %v\n", err)
		}

		if loaded.Config.AutoIssueManagement {
			if err := engine.SpawnBackgroundSync(loaded.ConfigDir, newSpecID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			}
		}
	}

	fmt.Printf("Created new bug spec: %s (%s)\n", newSpecID, title)
	return newSpecID, nil
}

func promptForSpecSelection(wd string) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		specID, err := selectSpecWithScanner(wd, true, scanner)
		if err != nil {
			if err.Error() == "back" {
				continue
			}
			return "", err
		}
		return specID, nil
	}
}

func parseSpecFileForCommit(filePath string) (*SpecFileCommit, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed frontmatter")
	}

	frontmatter := parts[1]
	body := strings.TrimSpace(parts[2])

	spec := &SpecFileCommit{}

	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "spec_id:") {
			spec.SpecID = strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
			spec.SpecID = strings.Trim(spec.SpecID, `"`)
		} else if strings.HasPrefix(line, "status:") {
			spec.Status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			spec.Status = strings.Split(spec.Status, " ")[0]
		} else if strings.HasPrefix(line, "type:") {
			spec.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
			spec.Type = strings.Trim(spec.Type, `"`)
		}
	}

	if strings.HasPrefix(body, "# ") {
		titleLine := strings.SplitN(body, "\n", 2)[0]
		spec.Title = strings.TrimPrefix(titleLine, "# ")
	}

	return spec, nil
}

type SpecFileCommit struct {
	SpecID string
	Title  string
	Status string
	Type   string
}

func extractSpecID(msg string) string {
	re := regexp.MustCompile(`\[(SPEC-\d+)\]`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func specExists(specs []SpecInfo, specID string) bool {
	for _, s := range specs {
		if s.SpecID == specID {
			return true
		}
	}
	return false
}

func extractMessageFromArgs(args []string) string {
	var msgParts []string
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			if arg == "--message" || arg == "-m" || arg == "--spec" || arg == "-s" || arg == "--failure-decay" || arg == "-d" || arg == "--run" || arg == "-r" || arg == "--all" || arg == "--type" || arg == "-t" || arg == "--ver" {
				skipNext = true
			}
			continue
		}
		msgParts = append(msgParts, arg)
	}
	return strings.Join(msgParts, " ")
}

func runListSpecs(wd string, all bool) error {
	configDir := wd
	ledger, err := engine.LoadLedger(configDir)
	if err != nil {
		return fmt.Errorf("failed to load ledger: %w", err)
	}

	specs, err := ledger.ListSpecs(all)
	if err != nil {
		return fmt.Errorf("failed to list specs: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)

	fmt.Fprintln(w, "SpecID\tRepo Issue\tStatus\tLinked Commits")
	fmt.Fprintln(w, "-------\t-----------\t------\t------------")

	cfg := ledger.WorkflowConfig
	statusColors := buildColorMap(cfg.Statuses)

	activeCount := 0
	archivedCount := 0

	for _, spec := range specs {
		repoIssue := spec.RepoIssueID
		if repoIssue == 0 {
			repoIssue = -1
		}

		linkedCommits := strings.Join(spec.LinkedCommits, ", ")
		if linkedCommits == "" {
			linkedCommits = "-"
		}

		colorizedStatus := colorizeStatus(spec.Status, statusColors)
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", spec.SpecID, repoIssue, colorizedStatus, linkedCommits)

		if cfg != nil && cfg.IsArchivedStatus(spec.Status) {
			archivedCount++
		} else if cfg == nil && (spec.Status == "resolved" || spec.Status == "deprecated") {
			archivedCount++
		} else {
			activeCount++
		}
	}

	w.Flush()

	fmt.Printf("\n")
	fmt.Printf("Active specs: %d\n", activeCount)
	fmt.Printf("Archived specs: %d\n", archivedCount)

	return nil
}

type colorFunc func(format string, a ...interface{}) string

func colorAttr(name string) color.Attribute {
	switch strings.ToLower(name) {
	case "black":
		return color.FgBlack
	case "red":
		return color.FgRed
	case "green":
		return color.FgGreen
	case "yellow":
		return color.FgYellow
	case "blue":
		return color.FgBlue
	case "magenta":
		return color.FgMagenta
	case "cyan":
		return color.FgCyan
	case "white":
		return color.FgWhite
	case "hiblack", "hi-black":
		return color.FgHiBlack
	case "hired", "hi-red":
		return color.FgHiRed
	case "higreen", "hi-green":
		return color.FgHiGreen
	case "hiyellow", "hi-yellow":
		return color.FgHiYellow
	case "hiblue", "hi-blue":
		return color.FgHiBlue
	case "himagenta", "hi-magenta":
		return color.FgHiMagenta
	case "hicyan", "hi-cyan":
		return color.FgHiCyan
	case "hiwhite", "hi-white":
		return color.FgHiWhite
	default:
		return color.FgWhite
	}
}

func buildColorMap(defs []engine.StatusDef) map[string]colorFunc {
	m := make(map[string]colorFunc, len(defs))
	for _, d := range defs {
		m[d.Name] = color.New(colorAttr(d.Color)).SprintfFunc()
	}
	return m
}

func colorizeStatus(status string, m map[string]colorFunc) string {
	if c, ok := m[status]; ok {
		return c(status)
	}
	return status
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
