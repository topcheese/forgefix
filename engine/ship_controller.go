package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ForgeFix/engine/housekeeper"
)

// ShipController orchestrates the ship reconciliation flow.
// Each collaborator is injected via the constructor, keeping the
// function manageable and testable.
type ShipController struct {
	config    *Config
	configDir string
	aiMode    bool
	vm        *VersionManager
}

// NewShipController creates a ShipController with the given configuration.
func NewShipController(config *Config, configDir string, aiMode bool) *ShipController {
	return &ShipController{
		config:    config,
		configDir: configDir,
		aiMode:    aiMode,
		vm:        NewVersionManager(configDir),
	}
}

// Run executes the full ship reconciliation flow: gate check, audit resolution,
// version prompt, git push, and housekeeping enqueue.
func (sc *ShipController) Run() {
	sc.requireGitHubConfig()

	shipSpecs, err := checkShipGateSpecStatuses(sc.configDir)
	if err != nil {
		sc.fatalError("SHIP_GATE_ERROR", fmt.Sprintf("Strict Shipping Gate: %s", err.Error()))
	}

	if err := verifyCommitSpecBindings(sc.configDir); err != nil {
		sc.fatalError("COMMIT_BINDING_ERROR", err.Error())
	}

	// Two paths: spec-based (shipSpecs non-empty) or audit-log-based
	if len(shipSpecs) == 0 {
		coord := sc.buildCoordinator()
		if coord == nil {
			sc.fatalError("COORDINATOR_ERROR", "Failed to create issue coordinator")
		}
		sc.handleAuditPath(coord)
	} else {
		sc.logInfo(fmt.Sprintf("Ship validation passed. %d spec(s) ready to ship.", len(shipSpecs)))
	}

	// Version prompt and write
	shipVersion := sc.vm.HandleShipVersion(sc.aiMode)

	// Update version field on all shipped specs
	for _, id := range shipSpecs {
		spec, specErr := LoadSpecByID(sc.configDir, id)
		if specErr == nil && spec != nil {
			if err := sc.vm.UpdateSpecFileVersion(spec.FilePath, shipVersion); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update version in spec %s: %v\n", id, err)
			}
		}
	}

	// Git push
	fmt.Println("Pushing to remote...")
	pushOut, pushErr := execGit(sc.configDir, "push")
	if pushErr != nil {
		fmt.Fprintf(os.Stderr, "Error: git push failed: %v\n%s\n", pushErr, pushOut)
		os.Exit(1)
	}
	fmt.Println("Push successful.")

	// Enqueue housekeeping tasks for shipped specs
	sc.enqueueHousekeeping(shipSpecs, shipVersion)

	// Drain the queue immediately so shipped specs transition to "closed"
	// without requiring a separate ff sync.
	fmt.Println("Processing housekeeping (close issues, sync metadata)...")
	if err := DrainHousekeepingQueueFromConfig(sc.configDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: housekeeping drain failed: %v\n", err)
	} else {
		fmt.Println("Housekeeping complete.")
	}
}

// requireGitHubConfig checks that GitHub credentials are configured.
// Exits with an error if not.
func (sc *ShipController) requireGitHubConfig() {
	if sc.config.GitHub == nil || sc.config.GitHub.Token == "" || sc.config.GitHub.Owner == "" || sc.config.GitHub.Repo == "" {
		if sc.aiMode {
			EmitAIError("GITHUB_CONFIG_ERROR", "GitHub/Repo not configured. Ship requires issue coordinator credentials.")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Error: GitHub/Repo not configured. Ship requires issue coordinator credentials.")
		os.Exit(1)
	}
}

// buildCoordinator creates an IssueCoordinator from the controller's config.
func (sc *ShipController) buildCoordinator() *IssueCoordinator {
	return NewCoordinatorFromConfig(sc.config, sc.configDir, sc.aiMode)
}

// handleAuditPath processes the audit-log-based ship path: finding resolved
// issues, validating staged changes, and proceeding with ship.
func (sc *ShipController) handleAuditPath(coord *IssueCoordinator) {
	entries := ReadAuditLogEntries(sc.configDir)
	if len(entries) == 0 {
		if sc.aiMode {
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
		if sc.aiMode {
			fmt.Println("AI mode: proceeding with ship despite no automated resolutions.")
		} else if !confirmPrompt("No automated resolutions detected. Proceed manually?") {
			fmt.Fprintln(os.Stderr, "Ship aborted.")
			os.Exit(1)
		}
		if !sc.aiMode {
			fmt.Println("Proceeding with manual confirmation.")
			return
		}
	}

	if len(resolved) > 1 {
		fmt.Fprintf(os.Stderr, "Warning: Multiple resolved issues found (%d):\n", len(resolved))
		for _, r := range resolved {
			fmt.Fprintf(os.Stderr, "  - #%d: %s\n", r.IssueNumber, r.TestName)
		}
		if sc.aiMode {
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

	sc.validateStaging(testNames)
}

// validateStaging checks that staged changes are related to resolved test failures.
func (sc *ShipController) validateStaging(testNames map[string]bool) {
	statusOut, err := execGit(sc.configDir, "status", "--porcelain")
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

	stagedOut, err := execGit(sc.configDir, "diff", "--cached", "--name-only")
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
		fileDiff, err := execGit(sc.configDir, "diff", "--cached", "--", file)
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
	fmt.Printf("Validated %d resolved issue(s) with matching staged changes.\n", len(testNames))
}

// enqueueHousekeeping creates housekeeping tasks (resolution comment, close issue,
// sync metadata) for each shipped spec.
func (sc *ShipController) enqueueHousekeeping(shipSpecs []string, shipVersion string) {
	if len(shipSpecs) == 0 {
		return
	}

	hq := housekeeper.NewHousekeepingQueue(sc.configDir)
	if loadErr := hq.Load(); loadErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load housekeeping queue: %v\n", loadErr)
		return
	}

	baseURL := ""
	if sc.config.GitHub != nil {
		baseURL = sc.config.GitHub.BaseURL
	}

	for _, id := range shipSpecs {
		spec, specErr := LoadSpecByID(sc.configDir, id)
		repoIssueID := 0
		payloadRaw := ""
		if specErr == nil && spec != nil {
			repoIssueID = spec.RepoIssue
			payload := housekeeper.ResolutionPayload{
				SpecID:    spec.SpecID,
				Title:     spec.Title,
				Version:   shipVersion,
				RepoIssue: spec.RepoIssue,
				SpecURL:   specFileWebURL(baseURL, sc.config.GitHub.Owner, sc.config.GitHub.Repo, spec.FilePath),
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

// fatalError logs an error message and exits. In AI mode it also emits an AI event.
func (sc *ShipController) fatalError(eventType, message string) {
	if sc.aiMode {
		EmitAIError(eventType, message)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", message)
	os.Exit(1)
}

// logInfo prints an informational message to stdout.
func (sc *ShipController) logInfo(message string) {
	fmt.Println(message)
}
