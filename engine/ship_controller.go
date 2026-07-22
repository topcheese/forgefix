package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

	// Finalize the CHANGELOG: promote [Unreleased] sections to the shipped
	// version so the changelog reflects real releases. Best-effort: a failure
	// warns but does not abort the ship. Committed before the push so the
	// finalized changelog rides along in the shipped commit.
	if err := FinalizeChangelogForRelease(sc.configDir, shipVersion); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to finalize CHANGELOG.md: %v\n", err)
	} else if _, cErr := execGit(sc.configDir, "add", "CHANGELOG.md"); cErr == nil {
		if _, cErr := execGit(sc.configDir, "commit", "-m",
			fmt.Sprintf("chore: finalize CHANGELOG for v%s", shipVersion)); cErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to commit finalized CHANGELOG.md: %v\n", cErr)
		}
	}

	// Update version field on all shipped specs
	for _, id := range shipSpecs {
		spec, specErr := LoadSpecByID(sc.configDir, id)
		if specErr == nil && spec != nil {
			if err := sc.vm.UpdateSpecFileVersion(spec.FilePath, shipVersion); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update version in spec %s: %v\n", id, err)
			}
		}
	}

	// Update version field on all backlog/draft specs to match the ship version
	// so the backlog stays current with the release cycle.
	if err := sc.updateBacklogVersion(shipVersion); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update backlog version: %v\n", err)
	}

	// Resolve and validate the ship remote BEFORE pushing. Public remotes
	// (github.com, gitlab.com, bitbucket.org, ...) require explicit confirmation
	// and are refused by default and in AI mode. Private remotes (including the
	// local NAS/Gitea) ship automatically.
	decision, err := resolveShipRemote(sc.config, gitRemoteMapFunc(sc.configDir))
	if err != nil {
		sc.fatalError("SHIP_REMOTE_ERROR", err.Error())
	}
	if decision.Public {
		if sc.aiMode {
			sc.fatalError("SHIP_PUBLIC_REMOTE_BLOCKED",
				fmt.Sprintf("Refusing to ship to public remote %q (%s) in AI mode. Set ship_remote to your private NAS remote to ship automatically, or run interactively to confirm.", decision.Remote, decision.URL))
		}
		if !confirmPrompt(fmt.Sprintf("⚠ Ship target %q (%s) is a PUBLIC remote. Push unreviewed code to a public host", decision.Remote, decision.URL)) {
			sc.fatalError("SHIP_PUBLIC_REMOTE_DECLINED", fmt.Sprintf("Ship to public remote %q (%s) was not confirmed. Aborting.", decision.Remote, decision.URL))
		}
	}

	// Git push
	fmt.Printf("Pushing to remote %q (%s)...\n", decision.Remote, decision.URL)
	pushOut, pushErr := execGit(sc.configDir, "push", decision.Remote)
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
	if err := DrainHousekeepingQueueFromConfig(sc.configDir, sc.aiMode); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: housekeeping drain failed: %v\n", err)
	} else {
		fmt.Println("Housekeeping complete.")
	}

	// Create a git tag for the version and push it, then create a release on the
	// remote (Gitea/GitHub). Non-fatal: failure must not block the ship.
	if len(shipSpecs) > 0 {
		tagName := fmt.Sprintf("v%s", shipVersion)
		if out, err := execGit(sc.configDir, "tag", "-a", tagName, "-m", fmt.Sprintf("ForgeFix release %s", tagName)); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create git tag %s: %v\n%s\n", tagName, err, out)
		} else if out, err := execGit(sc.configDir, "push", "origin", tagName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to push tag %s: %v\n%s\n", tagName, err, out)
		} else {
			fmt.Printf("Created and pushed tag %s.\n", tagName)
		}

		coord := sc.buildCoordinator()
		if coord != nil {
			releaseBody := sc.buildReleaseBody(shipSpecs, shipVersion)
			releaseID, err := coord.CreateRelease(shipVersion, releaseBody)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create release %s: %v\n", shipVersion, err)
			} else {
				fmt.Printf("Created release %s on remote.\n", shipVersion)
				sc.uploadPlatformBinaries(coord, releaseID, shipVersion)
			}
		}
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

// buildReleaseBody builds the markdown body for the remote release, listing
// each shipped spec as a bullet point. Called after a successful push.
func (sc *ShipController) buildReleaseBody(shipSpecs []string, shipVersion string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## ForgeFix Release %s\n\n", shipVersion))
	sb.WriteString("Shipped specs:\n\n")

	// Derive web base from the API base URL (strip /api/v1 suffix).
	webBase := ""
	if sc.config.GitHub != nil && sc.config.GitHub.BaseURL != "" {
		webBase = strings.TrimSuffix(sc.config.GitHub.BaseURL, "/api/v1")
	}

	for _, id := range shipSpecs {
		title := ""
		repoIssue := 0
		if spec, err := LoadSpecByID(sc.configDir, id); err == nil && spec != nil {
			title = spec.Title
			repoIssue = spec.RepoIssue
		}

		// Build clickable issue link when possible.
		issueURL := ""
		if repoIssue > 0 && webBase != "" && sc.config.GitHub != nil {
			issueURL = fmt.Sprintf("%s/%s/%s/issues/%d", webBase, sc.config.GitHub.Owner, sc.config.GitHub.Repo, repoIssue)
		}

		// Avoid doubled IDs: when title is missing or matches the ID, show ID once.
		if title == "" || title == id {
			if issueURL != "" {
				sb.WriteString(fmt.Sprintf("- [%s](%s)\n", id, issueURL))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", id))
			}
		} else {
			if issueURL != "" {
				sb.WriteString(fmt.Sprintf("- [%s](%s) %s\n", id, issueURL, title))
			} else {
				sb.WriteString(fmt.Sprintf("- `%s` %s\n", id, title))
			}
		}
	}
	return sb.String()
}

// uploadPlatformBinaries builds the ff binary for multiple platforms and uploads
// each as a release asset. Non-fatal — failures are warnings, not blockers.
func (sc *ShipController) uploadPlatformBinaries(coord *IssueCoordinator, releaseID int, shipVersion string) {
	type target struct {
		goos   string
		goarch string
		suffix string
	}
	targets := []target{
		{runtime.GOOS, runtime.GOARCH, ""},
		{"linux", "amd64", "-linux-amd64"},
		{"darwin", "amd64", "-darwin-amd64"},
		{"darwin", "arm64", "-darwin-arm64"},
		{"windows", "amd64", "-windows-amd64.exe"},
	}

	for _, t := range targets {
		assetName := "ff" + t.suffix
		buildCmd := exec.Command("go", "build", "-o", assetName, ".")
		buildCmd.Env = append(os.Environ(), "GOOS="+t.goos, "GOARCH="+t.goarch)
		buildCmd.Dir = sc.configDir
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: build failed for %s/%s: %v\n%s\n", t.goos, t.goarch, err, out)
			continue
		}

		data, err := os.ReadFile(assetName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: reading built binary %s: %v\n", assetName, err)
			continue
		}

		if err := coord.UploadReleaseAsset(releaseID, assetName, data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: upload failed for %s: %v\n", assetName, err)
		} else {
			fmt.Printf("Uploaded %s (%d KB)\n", assetName, len(data)/1024)
		}
		os.Remove(assetName)
	}
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

// updateBacklogVersion updates the version field of all specs in backlog or draft
// status to match the ship version, keeping the backlog current with the release cycle.
func (sc *ShipController) updateBacklogVersion(shipVersion string) error {
	db, err := OpenDB(sc.configDir)
	if err != nil {
		return fmt.Errorf("opening DB: %w", err)
	}
	defer db.Close()

	// Get all backlog and draft specs
	specs, err := db.GetSpecsByStatus("backlog", "draft")
	if err != nil {
		return fmt.Errorf("querying backlog/draft specs: %w", err)
	}

	if len(specs) == 0 {
		return nil
	}

	sc.logInfo(fmt.Sprintf("Updating version to %s for %d backlog/draft spec(s)...", shipVersion, len(specs)))

	for _, spec := range specs {
		// Load the full spec to get the file path
		fullSpec, err := LoadSpecByID(sc.configDir, spec.SpecID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load spec %s: %v\n", spec.SpecID, err)
			continue
		}
		if fullSpec == nil || fullSpec.FilePath == "" {
			fmt.Fprintf(os.Stderr, "Warning: spec %s has no file path\n", spec.SpecID)
			continue
		}

		// Update the version in the spec file
		if err := sc.vm.UpdateSpecFileVersion(fullSpec.FilePath, shipVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update version in spec %s: %v\n", spec.SpecID, err)
			continue
		}

		// Update the version in the DB
		if err := db.UpdateSpecVersion(spec.SpecID, shipVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update DB version for spec %s: %v\n", spec.SpecID, err)
			continue
		}

		sc.logInfo(fmt.Sprintf("  Updated %s to version %s", spec.SpecID, shipVersion))
	}

	return nil
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
