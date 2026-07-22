package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type PipelineRunner struct {
	Runner *Runner
	Parser *Parser
}

func ExecuteSuite(config *Config, configDir string, aiMode bool, watchMode bool) {
	dashboard := NewDashboard(config.Pipelines)
	dashboard.ConfigDir = configDir
	if config.FailureDecaySeconds > 0 {
		dashboard.FailureDecaySecs = config.FailureDecaySeconds
	}
	ledger, err := LoadLedger(configDir)
	if err != nil {
		if aiMode {
			EmitAIError("LEDGER_CORRUPTION", err.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error initializing workspace ledger: %v\n", err)
		os.Exit(1)
	}
	ledger.ResetCurrentRun()
	dashboard.SetLedger(ledger)
	dashboard.ResetTrackers()

	dashboard.Coord = NewCoordinatorFromConfig(config, configDir, aiMode)

	globalTimeout := 2 * time.Minute
	if config.GlobalTimeoutSeconds > 0 {
		globalTimeout = time.Duration(config.GlobalTimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()

	var ui *UI
	var uiQuit chan struct{}
	var uiWG sync.WaitGroup
	if !aiMode {
		ui = NewUI(dashboard)
		uiQuit = make(chan struct{})
		uiWG.Add(1)
		go func() {
			defer uiWG.Done()
			defer func() { recover() }()
			ui.StartRenderLoop(uiQuit)
		}()
		go ui.StartKeyboardListener()
	}

	runners := make(map[string]*PipelineRunner)
	var wg sync.WaitGroup
	var parseWG sync.WaitGroup
	globalRegistry := NewFingerprintRegistry()

	for _, pipeline := range config.Pipelines {
		workDir := configDir
		if lang, ok := config.Languages[pipeline.Type]; ok {
			if found := findAnchorDir(configDir, lang.RootAnchor, config.ExcludeDirs); found != "" {
				workDir = found
			} else {
				dashboard.MarkPipelineSkipped(pipeline.ID)
				dashboard.AddSystemError("Pipeline " + pipeline.ID + " skipped: " + lang.RootAnchor + " not found in tree")
				dashboard.AddErrorCode(0)
				continue
			}
		}

		runner := NewRunner(pipeline, dashboard)
		runner.SetWorkDir(workDir)
		parser := NewParser(pipeline)
		runners[pipeline.ID] = &PipelineRunner{Runner: runner, Parser: parser}

		wg.Add(1)
		go func(r *Runner) {
			defer wg.Done()
			defer func() { recover() }()
			if err := r.Start(); err != nil {
				dashboard.AddErrorCode(1)
			}
		}(runner)

		parseWG.Add(1)
		go func(r *Runner, p *Parser) {
			defer parseWG.Done()
			defer func() { recover() }()
			for line := range r.StdoutChan {
				event, _ := p.ParseLine(line)
				if event.MatchedToken != "" {
					dashboard.UpdatePipelineMetricsWithDetails(p.Config().ID, event.TokenType, event.TestID, event.Elapsed, event.MatchedToken, event.TestName, event.ErrorTrace, event.FilePath, event.FailureLine, event.FailureColumn)
				}

				// 🟡 HOOK: Use the existing tnt.go structures natively right here
				if event.FilePath != "" {
					// Check if this file contains a duplicate chunk sequence from another file
					isCopy, originalPath, err := globalRegistry.ScanAndCheck(event.FilePath)
					if err == nil && isCopy {
						// Append a Yellow Alert warning directly to the existing dashboard log
						warnMsg := fmt.Sprintf("⚠️ YELLOW ALERT: Code duplication detected in %s (cloned from %s)",
							filepath.Base(event.FilePath), filepath.Base(originalPath))
						dashboard.AddSystemError(warnMsg)
					} else if err == nil {
						// If it's unique code, register it so future pipelines can catch copies of it
						_ = globalRegistry.ScanAndRegister(event.FilePath)
					}
				}
			}
		}(runner, parser)

		go func(r *Runner) {
			defer func() { recover() }()
			for range r.StderrChan {
			}
		}(runner)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, sigWINCH)

	done := make(chan struct{})
	go func() {
		defer func() { recover() }()
		wg.Wait()
		parseWG.Wait()
		dashboard.TestCommandCompleted = true
		dashboard.markDirty()
		close(done)
	}()

mainLoop:
	for {
		select {
		case <-done:
			if ui != nil {
				close(uiQuit)
			}
			dashboard.SetPipelineActive(false)

			allMet := true
			for _, p := range config.Pipelines {
				if dashboard.IsPipelineSkipped(p.ID) {
					continue
				}
				e := dashboard.GetLedgerEntry(p.ID)
				ef := p.LedgerFloor
				if e != nil && ef == 0 {
					ef = e.HistoricalFloor
				}
				if e == nil || e.TotalPassed < ef {
					allMet = false
					break
				}
			}

			manageIssues := dashboard.Coord != nil

			if dashboard.GetTotalFailures() > 0 || !allMet {
				dashboard.Bomb = BombDetonated
				for _, pr := range runners {
					pr.Runner.Kill()
				}
				if manageIssues {
					handleDetonationIssues(dashboard, configDir)
					closeStaleIssues(dashboard, configDir)
				}
			} else {
				dashboard.Bomb = BombDefused
				if manageIssues {
					handleDefusedIssues(dashboard, configDir)
				}
			}
			break mainLoop

		case <-ctx.Done():
			if ui != nil {
				close(uiQuit)
			}
			dashboard.SetTimeoutFired(true)
			dashboard.SetPipelineActive(false)
			dashboard.TriggerDetonation()
			for _, pr := range runners {
				pr.Runner.Kill()
			}
			manageIssues := dashboard.Coord != nil
			if manageIssues {
				handleTimeoutIssues(dashboard, configDir)
				closeStaleIssues(dashboard, configDir)
			}
			break mainLoop

		case sig := <-sigChan:
			switch sig {
			case sigWINCH:
				if ui != nil {
					dashboard.markDirty()
				}
				continue
			default:
				if ui != nil {
					close(uiQuit)
				}
				dashboard.SetPipelineActive(false)
				fmt.Print("\033[0m\033[?25h")
				return
			}
		}
	}

	time.Sleep(200 * time.Millisecond)
	if err := SaveLedger(dashboard.GetLedger(), configDir); err != nil && !aiMode {
		fmt.Fprintf(os.Stderr, "warning: failed to save ledger metrics: %v\n", err)
	}

	if !aiMode && ui != nil {
		ui.inFinalScreen = true
		ui.renderFinal(dashboard, config)
		ui.waitForExit()
	} else if aiMode {
		if dashboard.GetBomb() == BombDetonated {
			EmitDetonated(dashboard)
		} else {
			EmitJSON(dashboard)
		}
	}

	if ledger := dashboard.GetLedger(); ledger != nil && ledger.GetTotalRan() == 0 {
		os.Exit(1)
	}
}

func collectFailedTests(d *Dashboard) []TestInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var failed []TestInfo
	for _, pipeline := range d.GetPipelinesSlice() {
		tracker := d.GetTestTrackersMap()[pipeline.ID]
		if tracker == nil {
			continue
		}
		for _, info := range tracker.Completed {
			if info.State == StateDud {
				f := *info
				f.TestID = f.ID
				f.TestName = f.Name
				failed = append(failed, f)
			}
		}
	}
	return failed
}

func handleDetonationIssues(d *Dashboard, configDir string) {
	failed := collectFailedTests(d)
	sysErrors := d.GetSystemErrors()

	for _, info := range failed {
		// 1. Create spec FIRST, capture the real specID
		title := sanitizeSpecTitle("Fix " + info.Name)
		var specID string
		if _, _, isDup := FindDuplicateSpec(configDir, title); !isDup {
			body := fmt.Sprintf("## Objective\nAutomatically created from failing test %s during ff --ai run.\n\n## Root Cause\nTest failed - see failure details below.\n\n## Failure Details\n- Test: %s\n- File: %s\n- Line: %d\n- Error: %s",
				info.Name, info.Name, info.FilePath, info.FailureLine, info.ErrorTrace)

			specVersion := incrementPatchVersion(CurrentVersion(configDir))
			sid, specPath, werr := writeSpecFromTemplate(configDir, title, title, body, "bug", specVersion, "")
			if werr != nil {
				d.AddSystemError(fmt.Sprintf("failed to create spec for %s: %v", info.Name, werr))
				continue
			}
			specID = sid
			// Register the spec in the ledger with the REAL specID
			if ledger, lerr := LoadLedger(configDir); lerr == nil {
				ledger.SetSpecEntry(specID, &SpecEntry{
					SpecID:        specID,
					RepoIssueID:   0,
					Status:        "draft",
					LinkedCommits: []string{},
				})
				_ = SaveLedger(ledger, configDir)
			}
			d.AddSystemError(fmt.Sprintf("created spec %s for failing test %s", specPath, info.Name))
		} else {
			// If duplicate, set specID to empty — the issue still gets
			// created but won't be force-linked to a new spec.
			specID = ""
		}

		// 2. Queue issue creation with the real specID (if we have one)
		details := &ErrorDetails{
			TestName:     info.Name,
			FilePath:     info.FilePath,
			LineNumber:   info.FailureLine,
			ErrorMessage: info.ErrorTrace,
			StackTrace:   info.ErrorTrace,
		}
		if err := QueueCreateIssue(configDir, specID, info.Name, details); err != nil {
			d.AddSystemError(fmt.Sprintf("failed to queue issue creation for %s: %v", info.Name, err))
			continue
		}

		d.mu.Lock()
		if d.IssueRefs == nil {
			d.IssueRefs = make(map[string]*IssueInfo)
		}
		d.IssueRefs[info.Name] = &IssueInfo{Number: 0, URL: "", Existed: false}
		d.mu.Unlock()
	}

	for _, sysErr := range sysErrors {
		if strings.Contains(sysErr, "issue #") ||
			strings.Contains(sysErr, "failed to close issue") ||
			strings.Contains(sysErr, "failed to create issue") {
			continue
		}
		if d.IssueRefs == nil {
			d.IssueRefs = make(map[string]*IssueInfo)
		}
		if _, exists := d.IssueRefs[sysErr]; exists {
			continue
		}
		title := strings.TrimPrefix(sysErr, "\u26a0\ufe0f YELLOW ALERT: ")
		details := &ErrorDetails{
			TestName:     title,
			ErrorMessage: sysErr,
		}
		if err := QueueCreateIssue(configDir, "", title, details); err != nil {
			continue
		}
		d.mu.Lock()
		d.IssueRefs[sysErr] = &IssueInfo{Number: 0, URL: "", Existed: false}
		d.mu.Unlock()
	}
}

func buildResolutionComment(issueNumber int, testName, statusLine, summary string, allOpen []GitHubIssue) string {
	var sb strings.Builder
	sb.WriteString("## Resolution — [ForgeFix Resolution Report]\n\n")
	sb.WriteString(statusLine + "\n\n")
	sb.WriteString(fmt.Sprintf("**Test:** `%s`\n", testName))
	sb.WriteString("**Closed by:** ForgeFix Auto-Resolution\n\n")
	sb.WriteString("### Summary\n\n")
	sb.WriteString(summary + "\n\n")

	var related []GitHubIssue
	for _, issue := range allOpen {
		if issue.Number != issueNumber {
			related = append(related, issue)
		}
	}
	if len(related) > 0 {
		sb.WriteString("### Related Open Issues\n\n")
		for _, issue := range related {
			sb.WriteString(fmt.Sprintf("- #%d: %s\n", issue.Number, issue.Title))
		}
	}
	return sb.String()
}

func handleDefusedIssues(d *Dashboard, configDir string) {
	if d.Coord == nil {
		return
	}

	if len(d.IssueRefs) == 0 {
		tracked := ReadAuditLog(configDir)
		for testName, number := range tracked {
			if d.IssueRefs == nil {
				d.IssueRefs = make(map[string]*IssueInfo)
			}
			d.IssueRefs[testName] = &IssueInfo{Number: number}
		}
	}

	for testName, ref := range d.IssueRefs {
		comment := buildResolutionComment(ref.Number, testName,
			"**Status:** ✅ ALL TESTS PASSED",
			"All pipeline tests passed successfully. The issue has been automatically closed.",
			nil)
		if err := QueuePostComment(configDir, testName, ref.Number, "Resolution", comment); err != nil {
			d.AddSystemError(fmt.Sprintf("failed to queue resolution comment for issue #%d: %v", ref.Number, err))
			continue
		}
		if err := QueueCloseIssue(configDir, testName, ref.Number); err != nil {
			d.AddSystemError(fmt.Sprintf("failed to queue close for issue #%d: %v", ref.Number, err))
			continue
		}
		DeleteAuditEntry(configDir, testName)
	}
	changelogPath := filepath.Join(configDir, "CHANGELOG.md")
	data, err := os.ReadFile(changelogPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		for _, ref := range d.IssueRefs {
			tag := fmt.Sprintf("(#%d)", ref.Number)
			if strings.Contains(line, tag) {
				lines[i] = "~~" + line + "~~"
			}
		}
	}
	_ = os.WriteFile(changelogPath, []byte(strings.Join(lines, "\n")), 0644)
}

func closeStaleIssues(d *Dashboard, configDir string) {
	if d.Coord == nil {
		return
	}
	tracked := ReadAuditLog(configDir)
	if len(tracked) == 0 {
		return
	}

	for testName, number := range tracked {
		if d.IssueRefs != nil {
			if _, exists := d.IssueRefs[testName]; exists {
				continue
			}
		}
		comment := buildResolutionComment(number, testName,
			"**Status:** ✅ RESOLVED",
			"This test is no longer failing. The issue has been automatically closed.",
			nil)
		if err := QueuePostComment(configDir, testName, number, "Resolution", comment); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to queue resolution comment on stale issue #%d for %s: %v\n", number, testName, err)
			continue
		}
		if err := QueueCloseIssue(configDir, testName, number); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to queue close for stale issue #%d for %s: %v\n", number, testName, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[AUDIT] queued close for stale issue #%d for %s (no longer failing)\n", number, testName)
		DeleteAuditEntry(configDir, testName)
	}
}

func handleTimeoutIssues(d *Dashboard, configDir string) {
	handleDetonationIssues(d, configDir)
}

func renderFinalFrame(d *Dashboard, config *Config) string {
	var r DashboardRenderer
	var sb strings.Builder
	sb.WriteString("\033[H\033[2J")

	for _, p := range config.Pipelines {
		sb.WriteString(d.RenderHeader(p))
		r.WriteBombFinal(&sb, d, p)

		if list := d.RenderTestList(p); list != "" {
			sb.WriteString(list)
		}
	}

	if d.GetBomb() == BombDetonated {
		sb.WriteString("\n" + d.RenderFailureReport())
		return sb.String()
	}
	if d.GetTimeoutFired() {
		r.WriteTimeoutSection(&sb, d)
		return sb.String()
	}

	sb.WriteString(d.FormatLedgerSummary(Bold, White, Green, Red, Reset))
	r.WriteSuccessFooter(&sb, d)
	sb.WriteString(Reset)
	return sb.String()
}

func findAnchorDir(root, anchor string, excludeDirs []string) string {
	if _, err := os.Stat(filepath.Join(root, anchor)); err == nil {
		return root
	}
	var found string
	_ = filepath.WalkDir(root, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			for _, excluded := range excludeDirs {
				if info.Name() == excluded {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if found != "" {
			return filepath.SkipAll
		}
		if info.Name() == anchor {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func EmitJSON(d *Dashboard) {
	payload := d.ToAIPayload()
	out, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(out))
}

func EmitDetonated(d *Dashboard) {
	payload := d.ToAIPayload()
	payload.Status = "DETONATED"
	out, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(out))
}

func EmitAIError(code, detail string) {
	payload := AIResponsePayload{
		Status:  "error",
		Version: "forgefix/v1",
		Metrics: AIMetricsSummary{},
		Pipelines: []AIPipelineResult{
			{
				ID:              "system",
				Name:            "System",
				Status:          "error",
				SuggestedAction: "CONFIG_LOAD_FAILURE: " + detail + ". Verify working config setup.",
				ErrorDetails:    code + ": " + detail,
			},
		},
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(out))
}

func execGit(configDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = configDir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %v failed: %s", args, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("executing git %v: %w", args, err)
	}
	return string(out), nil
}

func confirmPrompt(prompt string) bool {
	fmt.Fprintf(os.Stderr, "%s (y/N/q): ", prompt)
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))
	if response == "q" {
		os.Exit(0)
	}
	return response == "y" || response == "yes"
}

func LoadSpecByID(configDir, specID string) (*SpecFile, error) {
	specDir := filepath.Join(configDir, "specs")
	if _, statErr := os.Stat(specDir); os.IsNotExist(statErr) {
		specDir = filepath.Join(filepath.Dir(configDir), "specs")
	}
	entries, readErr := os.ReadDir(specDir)
	if readErr != nil {
		return nil, readErr
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		spec, parseErr := parseSpecFile(filepath.Join(specDir, entry.Name()))
		if parseErr != nil {
			continue
		}
		if spec.SpecID == specID {
			return spec, nil
		}
	}
	return nil, fmt.Errorf("spec %s not found", specID)
}

// ============================================================================
// PROJECT VERSION MANAGEMENT
// ============================================================================

func checkShipGateSpecStatuses(configDir string) (shipSpecs []string, err error) {
	specDir := filepath.Join(configDir, "specs")
	if _, statErr := os.Stat(specDir); os.IsNotExist(statErr) {
		specDir = filepath.Join(filepath.Dir(configDir), "specs")
	}
	entries, readErr := os.ReadDir(specDir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, nil
		}
		return nil, readErr
	}

	var blocking []string
	// First, check ledger for ship-ready specs
	ledger, ledgerErr := LoadLedger(configDir)
	if ledgerErr == nil {
		for specID, spec := range ledger.GetAllSpecEntries() {
			if spec.Status == "ship" {
				shipSpecs = append(shipSpecs, specID)
			} else if spec.Status == "in-progress" {
				blocking = append(blocking, fmt.Sprintf("  %s (%s)", specID, spec.Status))
			}
			// backlog and review status do NOT block — they are non-blocking labels
		}
	}

	// Then, check spec files on disk (for any additional ship-ready specs)
	// Disk status takes precedence over ledger status
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		spec, parseErr := parseSpecFile(filePath)
		if parseErr != nil {
			continue
		}
		if spec.Status == "ship" {
			// Check if this spec is already in shipSpecs from ledger
			found := false
			for _, existing := range shipSpecs {
				if existing == spec.SpecID {
					found = true
					break
				}
			}
			if !found {
				shipSpecs = append(shipSpecs, spec.SpecID)
			}
			// Disk says ship, so remove from blocking if present (disk takes precedence)
			newBlocking := make([]string, 0, len(blocking))
			for _, b := range blocking {
				if !strings.Contains(b, spec.SpecID) {
					newBlocking = append(newBlocking, b)
				}
			}
			blocking = newBlocking
		} else if spec.Status == "in-progress" {
			// Check if this spec is already in blocking list from ledger
			found := false
			for _, existing := range blocking {
				if strings.Contains(existing, spec.SpecID) {
					found = true
					break
				}
			}
			if !found {
				blocking = append(blocking, fmt.Sprintf("  %s (%s)", spec.SpecID, spec.Status))
			}
		}
		// backlog and review status do NOT block - disk backlog/review specs can still ship other specs
	}

	if len(blocking) > 0 {
		return nil, fmt.Errorf("strict shipping gate blocked — the following specs are not ship-ready:\n%s",
			strings.Join(blocking, "\n"))
	}
	return shipSpecs, nil
}

func ShipReconciliation(config *Config, configDir string, aiMode bool) {
	NewShipController(config, configDir, aiMode).Run()
}

func verifyCommitSpecBindings(configDir string) error {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return fmt.Errorf("failed to load ledger: %w", err)
	}

	gitRoot := findGitRoot(configDir)
	if gitRoot == "" {
		return fmt.Errorf("not a git repository")
	}

	out, err := exec.Command("git", "-C", gitRoot, "log", "--oneline", "--format=%H %s", "@{u}..HEAD").Output()
	if err != nil {
		return fmt.Errorf("failed to get unpushed commits: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	specIDRegex := regexp.MustCompile(`\[(SPEC-\d+)\]`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		commitHash := parts[0]
		commitMsg := parts[1]

		matches := specIDRegex.FindStringSubmatch(commitMsg)
		if len(matches) < 2 {
			continue
		}

		specID := matches[1]
		specEntry := ledger.GetSpecEntry(specID)
		if specEntry == nil {
			// Spec was removed from the ledger — check archive directory and DB.
			if isArchivedSpec(configDir, specID) || isSpecInDB(configDir, specID) {
				continue
			}
			return fmt.Errorf("orphaned commit detected: specID %s not found in ledger (commit %s)", specID, commitHash[:8])
		}

		if specEntry.Status != "backlog" && specEntry.Status != "draft" && specEntry.Status != "in-progress" && specEntry.Status != "review" && specEntry.Status != "ship" && specEntry.Status != "closed" {
			return fmt.Errorf("orphaned commit detected: specID %s has invalid status '%s' (expected 'backlog', 'draft', 'in-progress', 'review', 'ship', or 'closed') (commit %s)", specID, specEntry.Status, commitHash[:8])
		}
	}

	return nil
}

// isSpecInDB checks whether the spec ID exists in the SQLite database.
// Used as a fallback when the spec is not in the ledger but was archived to DB.
func isSpecInDB(configDir, specID string) bool {
	db, err := OpenDB(configDir)
	if err != nil {
		return false
	}
	defer db.Close()
	var count int
	err = db.Conn().QueryRow("SELECT COUNT(*) FROM specs WHERE spec_id = ?", specID).Scan(&count)
	return err == nil && count > 0
}

// isArchivedSpec reports whether specID appears in any archive file under
// specs/archive/. This prevents archived (shipped) specs from appearing as
// orphaned commits when their ledger entries have been cleaned up.
func isArchivedSpec(configDir, specID string) bool {
	archiveDir := filepath.Join(configDir, "specs", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(archiveDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), specID) {
			return true
		}
	}
	return false
}

func findGitRoot(dir string) string {
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
