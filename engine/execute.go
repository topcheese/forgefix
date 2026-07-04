package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ForgeFix/engine/housekeeper"
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
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH)

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
			case syscall.SIGWINCH:
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
		details := &ErrorDetails{
			TestName:     info.Name,
			FilePath:     info.FilePath,
			LineNumber:   info.FailureLine,
			ErrorMessage: info.ErrorTrace,
			StackTrace:   info.ErrorTrace,
		}
		if err := QueueCreateIssue(configDir, "", info.Name, details); err != nil {
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

func readProjectVersion(configDir string) string {
	path := ledgerPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.0.0"
	}
	var wrapper struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return "0.0.0"
	}
	if wrapper.Version == "" {
		return "0.0.0"
	}
	return wrapper.Version
}

func writeProjectVersion(configDir, version string) error {
	path := ledgerPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading ledger file: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling ledger: %w", err)
	}
	raw["version"] = version
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling updated ledger: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

func incrementPatchVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "0.0.1"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "0.0.1"
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, ".")
}

func isValidSemver(version string) bool {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	return re.MatchString(version)
}

func promptForVersion(current string) string {
	defaultVersion := incrementPatchVersion(current)
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Current project version: %s\n", current)
	fmt.Printf("Release version for this ship [%s]: ", defaultVersion)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		input = defaultVersion
	}
	if !isValidSemver(input) {
		fmt.Fprintf(os.Stderr, "Invalid semver format. Using default: %s\n", defaultVersion)
		return defaultVersion
	}
	return input
}

func updateSpecFileVersion(filePath, version string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading spec file %s: %w", filePath, err)
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if strings.HasPrefix(trimmed, "version:") {
			lines[i] = fmt.Sprintf("version: \"%s\"", version)
			found = true
		}
	}
	if !found {
		// Insert version before the closing ---
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" && i > 0 {
				lines = append(lines[:i], append([]string{fmt.Sprintf("version: \"%s\"", version)}, lines[i:]...)...)
				break
			}
		}
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

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
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		spec, parseErr := parseSpecFile(filePath)
		if parseErr != nil {
			continue
		}
		switch spec.Status {
		case "review":
			blocking = append(blocking, fmt.Sprintf("  %s (%s)", spec.SpecID, spec.Status))
		case "ship":
			shipSpecs = append(shipSpecs, spec.SpecID)
		}
	}

	if len(blocking) > 0 {
		return nil, fmt.Errorf("strict shipping gate blocked — the following specs are not ship-ready:\n%s",
			strings.Join(blocking, "\n"))
	}
	return shipSpecs, nil
}

func ShipReconciliation(config *Config, configDir string, aiMode bool) {
	if config.GitHub == nil || config.GitHub.Token == "" || config.GitHub.Owner == "" || config.GitHub.Repo == "" {
		if aiMode {
			EmitAIError("GITHUB_CONFIG_ERROR", "GitHub/Repo not configured. Ship requires issue coordinator credentials.")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Error: GitHub/Repo not configured. Ship requires issue coordinator credentials.")
		os.Exit(1)
	}
	coord := NewCoordinatorFromConfig(config, configDir, aiMode)

	shipSpecs, err := checkShipGateSpecStatuses(configDir)
	if err != nil {
		if aiMode {
			EmitAIError("SHIP_GATE_ERROR", fmt.Sprintf("Strict Shipping Gate: %s", err.Error()))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := verifyCommitSpecBindings(configDir); err != nil {
		if aiMode {
			EmitAIError("COMMIT_BINDING_ERROR", err.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// When ship-ready specs exist, skip the audit log / resolution check
	// and proceed directly to push (spec-based workflow).
	if len(shipSpecs) == 0 {
		entries := ReadAuditLogEntries(configDir)
		if len(entries) == 0 {
			if aiMode {
				EmitAIError("AUDIT_LOG_ERROR", "No audit log entries found. Nothing to ship.")
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "No audit log entries found. Nothing to ship.")
			os.Exit(0)
		}

		var resolved []AuditEntry
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Message, "CREATED") {
				continue
			}
			comments, err := coord.GetIssueComments(entry.IssueNumber)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to fetch comments for issue #%d: %v\n", entry.IssueNumber, err)
				continue
			}
			for _, comment := range comments {
				if strings.Contains(comment.Body, "[ForgeFix Resolution Report]") {
					resolved = append(resolved, entry)
					break
				}
			}
		}

		if len(resolved) == 0 {
			fmt.Fprintln(os.Stderr, "Error: No resolved issues with [ForgeFix Resolution Report] tag found.")
			if aiMode {
				fmt.Println("AI mode: proceeding with ship despite no automated resolutions.")
			} else if !confirmPrompt("No automated resolutions detected. Proceed manually?") {
				fmt.Fprintln(os.Stderr, "Ship aborted.")
				os.Exit(1)
			}
			if !aiMode {
				fmt.Println("Proceeding with manual confirmation.")
				return
			}
		}

		if len(resolved) > 1 {
			fmt.Fprintf(os.Stderr, "Warning: Multiple resolved issues found (%d):\n", len(resolved))
			for _, r := range resolved {
				fmt.Fprintf(os.Stderr, "  - #%d: %s\n", r.IssueNumber, r.TestName)
			}
			if aiMode {
				fmt.Println("AI mode: proceeding with all resolutions.")
			} else if !confirmPrompt("Multiple resolutions found. Proceed with all?") {
				fmt.Fprintln(os.Stderr, "Ship aborted.")
				os.Exit(1)
			}
		}

		testNames := make(map[string]bool)
		for _, r := range resolved {
			testNames[r.TestName] = true
		}

		statusOut, err := execGit(configDir, "status", "--porcelain")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to check git status: %v\n", err)
			os.Exit(1)
		}

		var untracked []string
		for _, line := range strings.Split(statusOut, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "??") {
				untracked = append(untracked, strings.TrimSpace(line[2:]))
			}
		}
		if len(untracked) > 0 {
			fmt.Fprintln(os.Stderr, "Error: Workspace contains untracked files unrelated to test execution:")
			for _, f := range untracked {
				fmt.Fprintf(os.Stderr, "  - %s\n", f)
			}
			fmt.Fprintln(os.Stderr, "Ship aborted: commit or stash untracked files before shipping.")
			os.Exit(1)
		}

		stagedOut, err := execGit(configDir, "diff", "--cached", "--name-only")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get staged files: %v\n", err)
			os.Exit(1)
		}

		var stagedFiles []string
		for _, f := range strings.Split(stagedOut, "\n") {
			f = strings.TrimSpace(f)
			if f != "" {
				stagedFiles = append(stagedFiles, f)
			}
		}

		var unrelatedFiles []string
		for _, file := range stagedFiles {
			fileDiff, err := execGit(configDir, "diff", "--cached", "--", file)
			if err != nil {
				unrelatedFiles = append(unrelatedFiles, file)
				continue
			}
			related := false
			for testName := range testNames {
				if strings.Contains(fileDiff, testName) {
					related = true
					break
				}
			}
			if !related {
				unrelatedFiles = append(unrelatedFiles, file)
			}
		}
		if len(unrelatedFiles) > 0 {
			fmt.Fprintln(os.Stderr, "Error: The following staged changes are unrelated to resolved test failures:")
			for _, f := range unrelatedFiles {
				fmt.Fprintf(os.Stderr, "  - %s\n", f)
			}
			fmt.Fprintln(os.Stderr, "Ship aborted: all staged changes must be related to resolved test issues.")
			os.Exit(1)
		}

		fmt.Println("Ship validation passed.")
		fmt.Printf("Validated %d resolved issue(s) with matching staged changes.\n", len(resolved))
		for _, r := range resolved {
			fmt.Printf("  - #%d: %s\n", r.IssueNumber, r.TestName)
		}
	} else {
		fmt.Printf("Ship validation passed. %d spec(s) ready to ship.\n", len(shipSpecs))
	}

	// Prompt for release version before pushing
	currentVersion := readProjectVersion(configDir)
	shipVersion := promptForVersion(currentVersion)
	if shipVersion != currentVersion {
		if err := writeProjectVersion(configDir, shipVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write project version: %v\n", err)
		}
	}

	// Update version field on all shipped specs
	for _, id := range shipSpecs {
		spec, specErr := LoadSpecByID(configDir, id)
		if specErr == nil && spec != nil {
			if err := updateSpecFileVersion(spec.FilePath, shipVersion); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update version in spec %s: %v\n", id, err)
			}
		}
	}

	fmt.Println("Pushing to remote...")
	pushOut, pushErr := execGit(configDir, "push")
	if pushErr != nil {
		fmt.Fprintf(os.Stderr, "Error: git push failed: %v\n%s\n", pushErr, pushOut)
		os.Exit(1)
	}
	fmt.Println("Push successful.")

	if len(shipSpecs) > 0 {
		hq := housekeeper.NewHousekeepingQueue(configDir)
		if loadErr := hq.Load(); loadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load housekeeping queue: %v\n", loadErr)
		} else {
			for _, id := range shipSpecs {
				spec, specErr := LoadSpecByID(configDir, id)
				repoIssueID := 0
				payloadRaw := ""
				if specErr == nil && spec != nil {
					repoIssueID = spec.RepoIssue
						baseURL := ""
					if config.GitHub != nil {
						baseURL = config.GitHub.BaseURL
					}
					payload := housekeeper.ResolutionPayload{
						SpecID:    spec.SpecID,
						Title:     spec.Title,
						Version:   spec.Version,
						RepoIssue: spec.RepoIssue,
						SpecURL:   specFileWebURL(baseURL, config.GitHub.Owner, config.GitHub.Repo, spec.FilePath),
					}
					payloadRawBytes, _ := json.Marshal(payload)
					payloadRaw = string(payloadRawBytes)
				}

				if repoIssueID > 0 {
					hq.Enqueue(housekeeper.HousekeepingTask{
						Type:        housekeeper.TaskTypePostResolution,
						SpecID:      id,
						RepoIssueID: repoIssueID,
						Priority:    housekeeper.PriorityHigh,
						Payload:     payloadRaw,
					})
					hq.Enqueue(housekeeper.HousekeepingTask{
						Type:        housekeeper.TaskTypeCloseIssue,
						SpecID:      id,
						RepoIssueID: repoIssueID,
						Priority:    housekeeper.PriorityHigh,
					})
				}

				hq.Enqueue(housekeeper.HousekeepingTask{
					Type:     housekeeper.TaskTypeSyncMetadata,
					SpecID:   id,
					Priority: housekeeper.PriorityMedium,
				})
			}
			fmt.Printf("Enqueued %d housekeeping task(s) for shipped specs.\n", len(shipSpecs))
			fmt.Println("Run `ff sync` to close remote issues and post resolution comments.")
		}
	}
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
			return fmt.Errorf("orphaned commit detected: specID %s not found in ledger (commit %s)", specID, commitHash[:8])
		}

		if specEntry.Status != "in-progress" && specEntry.Status != "review" && specEntry.Status != "ship" && specEntry.Status != "closed" {
			return fmt.Errorf("orphaned commit detected: specID %s has invalid status '%s' (expected 'in-progress', 'review', 'ship', or 'closed') (commit %s)", specID, specEntry.Status, commitHash[:8])
		}
	}

	return nil
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
