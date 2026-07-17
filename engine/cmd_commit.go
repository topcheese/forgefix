package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// handleCommit creates a validated commit bound to a spec.
func (d *CommandDispatcher) handleCommit(args []string) (CommandResult, error) {
	flags := ParseFlags(args)
	msg := flags.Message
	if msg == "" {
		msg = ExtractMessageFromArgs(args)
	}

	commitHash, specID, commitMsg, err := runCommit(d.WorkDir, msg, flags.SpecID, flags.SpecType, flags.SpecVersion, flags.AIMode, d, flags.Body)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	ledgerDir := SpecConfigDir(d.WorkDir)
	if err := DrainHousekeepingQueueFromConfig(ledgerDir, flags.AIMode); err != nil {
		fmt.Fprintf(d.Stderr, "warning: housekeeping drain failed: %v\n", err)
	}

	if specID != "" {
		if err := UpdateLedgerAfterCommit(ledgerDir, specID, commitHash); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}

		// Fold the metadata changes into the previous commit so they're not left
		// as untracked side effects. The commit message remains the same.
		if err := amendLastCommit(d.WorkDir); err != nil {
			fmt.Fprintf(d.Stderr, "warning: failed to amend metadata into commit: %v\n", err)
		}

		if err := SpawnBackgroundSync(ledgerDir, specID); err != nil {
			fmt.Fprintf(d.Stderr, "warning: failed to spawn background sync: %v\n", err)
		}

		// Post the commit message as a comment on the linked remote issue
		if entry, lErr := getSpecEntry(ledgerDir, specID); lErr == nil && entry.RepoIssueID > 0 {
			if err := QueuePostComment(ledgerDir, "", entry.RepoIssueID, "Commit", commitMsg); err != nil {
				fmt.Fprintf(d.Stderr, "warning: failed to queue commit comment: %v\n", err)
			}
		}
	}

	return CommandResult{ExitCode: 0}, nil
}

// runCommit executes the full commit workflow: resolves the spec, stages, commits,
// and updates the ledger. Returns the commit hash and spec ID.
func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode bool, d *CommandDispatcher, body string) (string, string, string, error) {
	gitRoot, err := findGitRootWalk(wd)
	if err != nil {
		return "", "", "", err
	}

	specID := flagSpecID
	if specID == "" {
		specID = extractSpecID(msg)
	}
	if specID == "" {
		var err error
		if aiMode {
			specID, err = autoDetectSpecFromWorkingTree(wd)
		} else {
			specID, err = promptForSpecSelection(wd, d)
		}
		if err != nil {
			return "", "", "", err
		}
	}

	// Check for ambiguous auto-detect: if no explicit --spec was given and we're in AI mode,
	// check if there are multiple recently-modified active specs that could cause ambiguity.
	// If so, require explicit --spec instead of guessing.
	explicitSpecGiven := flagSpecID != ""
	if aiMode && !explicitSpecGiven {
		if ambiguous, err := checkAmbiguousAutoDetect(wd); err != nil {
			return "", "", "", err
		} else if ambiguous {
			return "", "", "", fmt.Errorf("ambiguous auto-detect: multiple recently-modified active specs found. Use --spec <id> to explicitly bind")
		}
	}

	var commitMsg string
	if specID != "" {
		specDir := filepath.Join(wd, "specs")
		specFile, err := findSpecFileByID(specDir, specID)

		// Tier 2 — ff commit --ai reads and validates spec metadata (SOFT GATE)
		if err == nil {
			if data, rErr := os.ReadFile(specFile); rErr == nil {
				content := string(data)
				if strings.HasPrefix(content, "---") {
					parts := strings.SplitN(content, "---", 3)
					if len(parts) >= 3 {
						frontmatter := parts[1]
						isBug := strings.Contains(frontmatter, "type: bug")
						hasRootCause := false
						for _, fl := range strings.Split(frontmatter, "\n") {
							fl = strings.TrimSpace(fl)
							if strings.HasPrefix(fl, "root_cause:") {
								rv := strings.Trim(strings.TrimPrefix(fl, "root_cause:"), ` "`)
								if rv != "" {
									hasRootCause = true
								}
								break
							}
						}
						hasVersion := false
						for _, fl := range strings.Split(frontmatter, "\n") {
							fl = strings.TrimSpace(fl)
							if strings.HasPrefix(fl, "version:") {
								sv := strings.Trim(strings.TrimPrefix(fl, "version:"), ` "`)
								if sv != "" {
									hasVersion = true
								}
								break
							}
						}
						if isBug && !hasRootCause {
							fmt.Fprintf(os.Stderr, "warning: bug spec %s has no root_cause documented in frontmatter\n", specID)
						}
						if !hasVersion {
							fmt.Fprintf(os.Stderr, "warning: spec %s has no version set in frontmatter\n", specID)
						}
					}
				}
			}
		}

		if err == nil && (specType != "" || specVersion != "") {
			ledgerDir := SpecConfigDir(wd)
			wc := LoadWorkflowConfig(ledgerDir)

			if specType != "" {
				if wc != nil && wc.CategoryLabelFor("type", specType) == "" {
					fmt.Fprintf(d.Stderr, "warning: type %q not found in label_categories.type defaults\n", specType)
				}
			}
			if specVersion != "" {
				if wc != nil && wc.CategoryLabelFor("version", specVersion) == "" {
					fmt.Fprintf(d.Stderr, "warning: version %q not found in label_categories.version defaults\n", specVersion)
				}
			}

			if err := updateSpecFileTypeVersion(specFile, specType, specVersion); err != nil {
				return "", "", "", fmt.Errorf("updating spec metadata: %w", err)
			}
			fmt.Fprintf(d.Stdout, "Updated spec %s: type=%q version=%q\n", specID, specType, specVersion)
		}

		if msg == "" {
			fmt.Fprint(d.Stdout, "Commit message (or q to quit): ")
			var response string
			_, _ = fmt.Scanln(&response)
			msg = strings.TrimSpace(response)
			if strings.ToLower(msg) == "q" {
				return "", "", "", fmt.Errorf("aborted by user")
			}
			if msg == "" {
				return "", "", "", fmt.Errorf("empty commit message")
			}
		}

		// Strip the current spec's tag from message to avoid doubling, but leave other spec references intact
		cleaned := strings.TrimSpace(strings.ReplaceAll(msg, "["+specID+"]", ""))
		commitMsg = strings.TrimSpace(fmt.Sprintf("feat: [%s] %s", specID, cleaned))

		// Append body if provided (FR-4 — commit message body support)
		if body != "" {
			commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, body)
		}
	} else {
		if msg == "" {
			fmt.Fprint(d.Stdout, "Commit message (or q to quit): ")
			var response string
			_, _ = fmt.Scanln(&response)
			msg = strings.TrimSpace(response)
			if strings.ToLower(msg) == "q" {
				return "", "", "", fmt.Errorf("aborted by user")
			}
			if msg == "" {
				return "", "", "", fmt.Errorf("empty commit message")
			}
		}
		commitMsg = msg
	}

	// Write metadata to disk BEFORE the commit so they're included in the
	// staged changes and don't remain as untracked side effects.
	if specID != "" {
		configDir := SpecConfigDir(wd)
		specDir := filepath.Join(wd, "specs")
		// ff commit (human) sets status to "review", ff commit --ai sets status to "draft"
		targetStatus := "draft"
		if !aiMode {
			targetStatus = "review"
		}
		if specFile, fErr := findSpecFileByID(specDir, specID); fErr == nil {
			_ = UpdateSpecFileStatus(specFile, targetStatus)
		}
		if ledger, lErr := LoadLedger(configDir); lErr == nil {
			if entry := ledger.GetSpecEntry(specID); entry != nil {
				entry.Status = targetStatus
				ledger.SetSpecEntry(specID, entry)
				_ = SaveLedger(ledger, configDir)
			}
		}
	}

	// Keep CHANGELOG.md in sync with changes by appending an entry derived
	// from the conventional-commit message. Done before the commit so the
	// change is staged and captured in the same commit. Best-effort: a
	// failure here warns but does not fail the commit.
	if err := AppendChangelogEntry(wd, commitMsg); err != nil {
		fmt.Fprintf(d.Stderr, "warning: failed to update CHANGELOG.md: %v\n", err)
	}

	commitHash, err := AutoStageAndCommit(gitRoot, commitMsg)
	if err != nil {
		return "", "", "", err
	}

	return commitHash, specID, commitMsg, nil
}

// UpdateLedgerAfterCommit records the commit in the ledger and updates linked commits.
// It does NOT change the spec status (that's handled in runCommit based on human vs --ai mode).
func UpdateLedgerAfterCommit(configDir, specID, commitHash string) error {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return err
	}
	entry := ledger.GetSpecEntry(specID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", specID)
	}
	entry.LinkedCommits = append(entry.LinkedCommits, commitHash)
	// Do NOT change status here - status promotion is handled in runCommit based on human vs --ai mode
	ledger.SetSpecEntry(specID, entry)

	specDir := filepath.Join(configDir, "specs")
	specFile, err := findSpecFileByID(specDir, specID)
	if err != nil {
		return fmt.Errorf("spec file not found on disk for %s: %w", specID, err)
	}
	// Do NOT update status here - only update linked commits
	if err := UpdateSpecFileLinkedCommits(specFile, commitHash); err != nil {
		return fmt.Errorf("updating spec file linked commits: %w", err)
	}

	return SaveLedger(ledger, configDir)
}

// findGitRootWalk walks up from dir to find the .git directory.
func findGitRootWalk(dir string) (string, error) {
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

// findSpecFileByID searches the specs directory for a file with the given spec ID.
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

// updateSpecFileTypeVersion writes type/version fields in the spec file's frontmatter.
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

// insertFrontmatterLine inserts a line before repo_issue: or at end of frontmatter.
func insertFrontmatterLine(lines []string, newLine string) []string {
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

// SpecInfoEx holds parsed spec metadata for commit selection (extended version).
type SpecInfoEx struct {
	SpecID string
	Title  string
	Status string
	Type   string
}

// loadActiveSpecs reads all non-resolved spec files from the specs directory.
func loadActiveSpecs(wd string) ([]SpecInfoEx, error) {
	specDir := filepath.Join(wd, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var specs []SpecInfoEx
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
			specs = append(specs, SpecInfoEx{
				SpecID: spec.SpecID,
				Title:  spec.Title,
				Status: spec.Status,
				Type:   spec.Type,
			})
		}
	}
	return specs, nil
}

// SelectSpec prompts the user to choose a spec interactively.
func SelectSpec(wd string, allowNew bool) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	return selectSpecWithScanner(wd, allowNew, scanner, nil)
}

func promptForSpecSelection(wd string, d *CommandDispatcher) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		specID, err := selectSpecWithScanner(wd, true, scanner, d)
		if err != nil {
			if err.Error() == "back" {
				continue
			}
			return "", err
		}
		return specID, nil
	}
}

func selectSpecWithScanner(wd string, allowNew bool, scanner *bufio.Scanner, d *CommandDispatcher) (string, error) {
	ledgerDir := SpecConfigDir(wd)
	ledger, err := LoadLedger(ledgerDir)
	if err != nil {
		return "", fmt.Errorf("loading ledger: %w", err)
	}

	entries, err := ledger.ListSpecs(false)
	if err != nil {
		return "", fmt.Errorf("listing specs: %w", err)
	}

	out := io.Writer(os.Stdout)
	if d != nil && d.Stdout != nil {
		out = d.Stdout
	}

	fmt.Fprintln(out, "Select spec category:")
	if len(entries) == 0 && allowNew {
		fmt.Fprintln(out, "No active specs found. Creating a new bug spec...")
		return createNewBugSpec(scanner, wd, ledgerDir, ledger, d)
	}

	specsByType := groupSpecsByType(entries)

	categories := []string{"Feature", "Bug", "Refactor", "All"}
	typeMap := map[string]string{
		"Feature":  "feature",
		"Bug":      "bug",
		"Refactor": "refactor",
		"All":      "",
	}

	fmt.Fprintln(out, "Select spec category:")
	for i, cat := range categories {
		var count int
		if cat == "All" {
			count = len(entries)
		} else {
			count = len(specsByType[typeMap[cat]])
		}
		fmt.Fprintf(out, "  [%d] %s (%d specs)\n", i+1, cat, count)
	}
	fmt.Fprintln(out, "  [q] Quit / Abort")
	fmt.Fprintln(out, "  [Enter] Skip / No Spec")

	fmt.Fprint(out, "Select category: ")
	for {
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
			fmt.Fprintln(out, "Invalid selection")
			fmt.Fprint(out, "Select category: ")
			continue
		}

		selectedCategory := categories[catIdx-1]
		var filtered []*SpecEntry
		if selectedCategory == "All" {
			filtered = entries
		} else {
			filtered = specsByType[typeMap[selectedCategory]]
		}

		if len(filtered) == 0 {
			fmt.Fprintf(out, "No specs in category %s\n", selectedCategory)
			fmt.Fprint(out, "Select category: ")
			continue
		}

		return selectSpecFromList(filtered, scanner, allowNew, wd, ledgerDir, ledger, d)
	}
}

func groupSpecsByType(entries []*SpecEntry) map[string][]*SpecEntry {
	result := map[string][]*SpecEntry{
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

func selectSpecFromList(specs []*SpecEntry, scanner *bufio.Scanner, allowNew bool, wd, ledgerDir string, ledger *LedgerEngine, d *CommandDispatcher) (string, error) {
	out := io.Writer(os.Stdout)
	if d != nil && d.Stdout != nil {
		out = d.Stdout
	}

	fmt.Fprintf(out, "\nSelect spec (or choose an option):\n")
	if allowNew {
		fmt.Fprintln(out, "  [0] New Bug")
	}
	fmt.Fprintln(out, "  [b] Back to categories")
	fmt.Fprintln(out, "  [q] Quit / Abort")
	fmt.Fprintln(out, "  [Enter] Skip / No Spec")
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
		fmt.Fprintf(out, "  [%d] %s %s%s%s (%s)\n", i+1, e.SpecID, statusColor, e.Status, resetColor, e.Status)
	}

	fmt.Fprint(out, "Select option: ")
	for {
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
			return createNewBugSpec(scanner, wd, ledgerDir, ledger, d)
		}

		idx, err := strconv.Atoi(input)
		if err != nil {
			fmt.Fprintln(out, "Invalid selection")
			fmt.Fprint(out, "Select option: ")
			continue
		}

		if idx < 1 || idx > len(specs) {
			fmt.Fprintln(out, "Invalid selection")
			fmt.Fprint(out, "Select option: ")
			continue
		}

		return specs[idx-1].SpecID, nil
	}
}

func createNewBugSpec(scanner *bufio.Scanner, wd, ledgerDir string, ledger *LedgerEngine, d *CommandDispatcher) (string, error) {
	out := io.Writer(os.Stdout)
	if d != nil && d.Stdout != nil {
		out = d.Stdout
	}

	fmt.Fprint(out, "New bug title: ")
	if !scanner.Scan() {
		return "", fmt.Errorf("reading title: %w", scanner.Err())
	}
	title := strings.TrimSpace(scanner.Text())
	if title == "" {
		fmt.Fprintln(out, "Title cannot be empty")
		return createNewBugSpec(scanner, wd, ledgerDir, ledger, d)
	}

	newSpecID := fmt.Sprintf("SPEC-%d-BUG", time.Now().Unix())
	now := time.Now().Format("2006-01-02")

	origSpecID, origTitle, isDup := FindDuplicateSpec(ledgerDir, title)
	if isDup {
		action := promptForDuplicateAction(title, origTitle, origSpecID, false)
		switch action {
		case "link":
			fmt.Fprintf(out, "Linking to existing spec %s (%s).\n", origSpecID, origTitle)
			return origSpecID, nil
		case "update":
			fmt.Fprintf(out, "Updating existing spec %s (%s).\n", origSpecID, origTitle)
			if err := updateExistingSpec(ledgerDir, origSpecID, title); err != nil {
				return "", err
			}
			return origSpecID, nil
		case "create":
			title += " [Dupe]"
			fmt.Fprintf(out, "Creating new spec with [Dupe] suffix.\n")
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

	entry := &SpecEntry{
		SpecID:        title,
		RepoIssueID:   0,
		Status:        "draft",
		LinkedCommits: []string{},
	}
	ledger.SetSpecEntry(newSpecID, entry)
	if err := SaveLedger(ledger, ledgerDir); err != nil {
		return "", fmt.Errorf("saving ledger: %w", err)
	}

	loaded, loadErr := LoadPipelineConfig(wd)
	if loadErr == nil && loaded.Config != nil {
		if err := QueueSyncSpec(loaded.ConfigDir, newSpecID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to queue spec sync: %v\n", err)
		}

		if loaded.Config.AutoIssueManagement {
			if err := SpawnBackgroundSync(loaded.ConfigDir, newSpecID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to spawn background sync: %v\n", err)
			}
		}
	}

	fmt.Fprintf(out, "Created new bug spec: %s (%s)\n", newSpecID, title)
	return newSpecID, nil
}

// SpecFileCommit holds parsed data from a spec file's frontmatter.
type SpecFileCommit struct {
	SpecID string
	Title  string
	Status string
	Type   string
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

// autoDetectSpecFromWorkingTree picks the most relevant spec file in specs/
// so that `ff commit --ai` can bind to it without an interactive prompt.
// It prefers active specs (not yet shipped/closed) over shipped ones, and
// among those picks the most recently modified. This prevents auto-generated
// side-effect writes to shipped spec files from stealing the binding.
func autoDetectSpecFromWorkingTree(wd string) (string, error) {
	specDir := filepath.Join(wd, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no specs directory found; create one with `ff spec --ai <title>` first")
		}
		return "", err
	}

	type candidate struct {
		specID  string
		status  string
		modTime time.Time
	}

	var all, active []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		spec, parseErr := parseSpecFileForCommit(path)
		if parseErr != nil {
			continue
		}
		if spec.SpecID == "" {
			continue
		}
		c := candidate{specID: spec.SpecID, status: spec.Status, modTime: info.ModTime()}
		all = append(all, c)
		if spec.Status != "ship" && spec.Status != "closed" {
			active = append(active, c)
		}
	}

	if len(all) == 0 {
		return "", fmt.Errorf("no spec files found in specs/; create one with `ff spec --ai <title>` first")
	}

	pool := all
	if len(active) > 0 {
		pool = active
	}

	sort.Slice(pool, func(i, j int) bool {
		return pool[i].modTime.After(pool[j].modTime)
	})

	return pool[0].specID, nil
}

// checkAmbiguousAutoDetect checks if there are multiple recently-modified active specs
// that could cause ambiguity in auto-detection. Returns true if ambiguous (multiple
// active specs modified within a short time window), false otherwise.
func checkAmbiguousAutoDetect(wd string) (bool, error) {
	specDir := filepath.Join(wd, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	type candidate struct {
		specID  string
		status  string
		modTime time.Time
	}

	var active []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		spec, parseErr := parseSpecFileForCommit(path)
		if parseErr != nil {
			continue
		}
		if spec.SpecID == "" {
			continue
		}
		if spec.Status != "ship" && spec.Status != "closed" {
			active = append(active, candidate{specID: spec.SpecID, status: spec.Status, modTime: info.ModTime()})
		}
	}

	if len(active) < 2 {
		return false, nil
	}

	// Sort by modification time (most recent first)
	sort.Slice(active, func(i, j int) bool {
		return active[i].modTime.After(active[j].modTime)
	})

	// Check if the top 2 active specs were modified within a short time window (e.g., 5 minutes)
	// This indicates the user may have been working on multiple specs recently
	timeWindow := 5 * time.Minute
	if active[0].modTime.Sub(active[1].modTime) < timeWindow {
		return true, nil
	}

	return false, nil
}

// extractSpecID extracts a SPEC-XXXXXXX ID from a commit message.
func extractSpecID(msg string) string {
	re := regexp.MustCompile(`\[(SPEC-\d+)\]`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// specExists checks if a spec ID is in the given list.
func specExists(specs []SpecInfoEx, specID string) bool {
	for _, s := range specs {
		if s.SpecID == specID {
			return true
		}
	}
	return false
}

// ExtractMessageFromArgs extracts the commit message from CLI args by
// skipping flag tokens and their values.
func ExtractMessageFromArgs(args []string) string {
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

// getSpecEntry loads the ledger and returns the spec entry for the given spec ID.
func getSpecEntry(configDir, specID string) (*SpecEntry, error) {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return nil, err
	}
	entry := ledger.GetSpecEntry(specID)
	if entry == nil {
		return nil, fmt.Errorf("spec %s not found in ledger", specID)
	}
	return entry, nil
}

// AutoStageAndCommit stages all changes and creates a commit.
// This is the primary commit entry point used by runCommit.
func AutoStageAndCommit(gitRoot, commitMsg string) (string, error) {
	git := NewGitHelper(gitRoot)
	if err := git.AddAll(); err != nil {
		return "", fmt.Errorf("auto-stage: %w", err)
	}
	return git.Commit(commitMsg)
}

// amendLastCommit adds all modified working-tree files and amends them into
// the previous commit with --no-edit. This folds metadata updates (spec status,
// ledger bindings) into the commit that produced them so they don't remain
// as untracked side effects.
func amendLastCommit(wd string) error {
	gitRoot, err := findGitRootWalk(wd)
	if err != nil {
		return err
	}
	git := NewGitHelper(gitRoot)
	if err := git.AddAll(); err != nil {
		return fmt.Errorf("amend: %w", err)
	}
	return git.Amend()
}
