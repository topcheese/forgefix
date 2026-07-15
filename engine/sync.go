package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ForgeFix/engine/housekeeper"

	"golang.org/x/term"
)

type SyncFailure struct {
	Timestamp  time.Time `json:"timestamp"`
	SpecID     string    `json:"spec_id,omitempty"`
	Error      string    `json:"error"`
	RetryCount int       `json:"retry_count"`
}

type SyncScheduleState struct {
	LastFullSync     time.Time `json:"last_full_sync,omitempty"`
	LastRetryAttempt time.Time `json:"last_retry_attempt,omitempty"`
}

const syncFailureLogName = ".sync_failures.log"
const syncScheduleStateName = ".sync_schedule.json"

type SyncOpType string

const (
	SyncOpCreateIssue     SyncOpType = "create_issue"
	SyncOpCloseIssue      SyncOpType = "close_issue"
	SyncOpPostComment     SyncOpType = "post_comment"
	SyncOpUpdateIssueBody SyncOpType = "update_issue_body"
	SyncOpSyncSpec        SyncOpType = "sync_spec"
	SyncOpDeleteIssue     SyncOpType = "delete_issue"
	SyncOpUpdateFailure   SyncOpType = "update_failure"
)

type SyncOperation struct {
	ID         string        `json:"id"`
	Type       SyncOpType    `json:"type"`
	Timestamp  time.Time     `json:"timestamp"`
	SpecID     string        `json:"spec_id,omitempty"`
	TestName   string        `json:"test_name,omitempty"`
	IssueNum   int           `json:"issue_number,omitempty"`
	Title      string        `json:"title,omitempty"`
	Body       string        `json:"body,omitempty"`
	Details    *ErrorDetails `json:"details,omitempty"`
	RetryCount int           `json:"retry_count"`
}

const syncQueueName = ".sync_queue.json"

func SyncQueuePath(configDir string) string {
	return filepath.Join(FFDir(configDir), syncQueueName)
}

func LoadSyncQueue(configDir string) ([]SyncOperation, error) {
	path := SyncQueuePath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ops []SyncOperation
	if err := json.Unmarshal(data, &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

func SaveSyncQueue(configDir string, ops []SyncOperation) error {
	path := SyncQueuePath(configDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func EnqueueSyncOp(configDir string, op SyncOperation) error {
	if configDir == "" {
		return nil
	}
	ops, _ := LoadSyncQueue(configDir)

	// Skip if an identical pending operation (same type, specID, issue number, test name) already exists
	for _, existing := range ops {
		if existing.Type == op.Type &&
			existing.SpecID == op.SpecID &&
			existing.IssueNum == op.IssueNum &&
			existing.TestName == op.TestName &&
			existing.RetryCount == 0 {
			return nil
		}
	}

	op.ID = fmt.Sprintf("%d-%s", time.Now().UnixNano(), op.Type)
	op.Timestamp = time.Now()
	op.RetryCount = 0
	ops = append(ops, op)
	return SaveSyncQueue(configDir, ops)
}

func DequeueSyncOp(configDir string) (SyncOperation, error) {
	ops, err := LoadSyncQueue(configDir)
	if err != nil || len(ops) == 0 {
		return SyncOperation{}, fmt.Errorf("queue empty")
	}
	op := ops[0]
	ops = ops[1:]
	if err := SaveSyncQueue(configDir, ops); err != nil {
		return SyncOperation{}, err
	}
	return op, nil
}

func PeekSyncQueue(configDir string) ([]SyncOperation, error) {
	return LoadSyncQueue(configDir)
}

func ClearSyncQueue(configDir string) error {
	path := SyncQueuePath(configDir)
	return os.Remove(path)
}

func QueueCreateIssue(configDir, specID, testName string, details *ErrorDetails) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpCreateIssue,
		SpecID:   specID,
		TestName: testName,
		Details:  details,
	})
}

func QueueCloseIssue(configDir, testName string, issueNumber int) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpCloseIssue,
		TestName: testName,
		IssueNum: issueNumber,
	})
}

func QueuePostComment(configDir, testName string, issueNumber int, title, body string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpPostComment,
		TestName: testName,
		IssueNum: issueNumber,
		Title:    title,
		Body:     body,
	})
}

func QueueUpdateIssueBody(configDir string, issueNumber int, body string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpUpdateIssueBody,
		IssueNum: issueNumber,
		Body:     body,
	})
}

func QueueSyncSpec(configDir, specID string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:   SyncOpSyncSpec,
		SpecID: specID,
	})
}

func QueueDeleteIssue(configDir, specID string, issueNumber int) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpDeleteIssue,
		SpecID:   specID,
		IssueNum: issueNumber,
	})
}

// QueueUpdateFailure enqueues an update failure to the sync queue for attention.
// This ensures update failures (no asset found, download error, etc.) are not silently dropped.
func QueueUpdateFailure(configDir, errorCode, detail string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:   SyncOpUpdateFailure,
		Title:  errorCode,
		Body:   detail,
	})
}

func SyncFailureLogPath(configDir string) string {
	return filepath.Join(FFDir(configDir), syncFailureLogName)
}

func SyncScheduleStatePath(configDir string) string {
	return filepath.Join(FFDir(configDir), syncScheduleStateName)
}

func LoadSyncScheduleState(configDir string) (*SyncScheduleState, error) {
	path := SyncScheduleStatePath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncScheduleState{}, nil
		}
		return nil, err
	}
	var state SyncScheduleState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func SaveSyncScheduleState(configDir string, state *SyncScheduleState) error {
	path := SyncScheduleStatePath(configDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ShouldRunFullSync(configDir string, maxAgeDays int) (bool, error) {
	state, err := LoadSyncScheduleState(configDir)
	if err != nil {
		return false, err
	}
	if state.LastFullSync.IsZero() {
		return true, nil
	}
	return time.Since(state.LastFullSync) >= time.Duration(maxAgeDays)*24*time.Hour, nil
}

func ShouldRetryFailures(configDir string, retryIntervalHours int) (bool, error) {
	state, err := LoadSyncScheduleState(configDir)
	if err != nil {
		return false, err
	}
	if state.LastRetryAttempt.IsZero() {
		return true, nil
	}
	return time.Since(state.LastRetryAttempt) >= time.Duration(retryIntervalHours)*time.Hour, nil
}

func MarkFullSyncRun(configDir string) error {
	state, err := LoadSyncScheduleState(configDir)
	if err != nil {
		return err
	}
	state.LastFullSync = time.Now()
	return SaveSyncScheduleState(configDir, state)
}

func MarkRetryAttempt(configDir string) error {
	state, err := LoadSyncScheduleState(configDir)
	if err != nil {
		return err
	}
	state.LastRetryAttempt = time.Now()
	return SaveSyncScheduleState(configDir, state)
}

func LoadSyncFailures(configDir string) ([]SyncFailure, error) {
	path := SyncFailureLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var failures []SyncFailure
	if err := json.Unmarshal(data, &failures); err != nil {
		return nil, err
	}
	return failures, nil
}

func SaveSyncFailures(configDir string, failures []SyncFailure) error {
	path := SyncFailureLogPath(configDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(failures, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func RecordSyncFailure(configDir, specID, errMsg string) error {
	failures, _ := LoadSyncFailures(configDir)
	failures = append(failures, SyncFailure{
		Timestamp:  time.Now(),
		SpecID:     specID,
		Error:      errMsg,
		RetryCount: 0,
	})
	return SaveSyncFailures(configDir, failures)
}

func ClearSyncFailures(configDir string) error {
	path := SyncFailureLogPath(configDir)
	return os.Remove(path)
}

func HasPendingSyncFailures(configDir string) (bool, error) {
	failures, err := LoadSyncFailures(configDir)
	if err != nil {
		return false, err
	}
	return len(failures) > 0, nil
}

func RunBackgroundSync(configDir, specID string, aiMode bool) error {
	loaded, err := LoadPipelineConfig(configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Reconcile ledger from spec files early, regardless of remote access
	if reconcilLedger, reconcilErr := LoadLedger(configDir); reconcilErr == nil {
		if reconcilErr := reconcilLedger.SyncFromSpecsDir(configDir); reconcilErr == nil {
			if saveErr := SaveLedger(reconcilLedger, configDir); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save reconciled ledger: %v\n", saveErr)
			}
		}
	}

	// AUTO_ISSUE_MANAGEMENT GATE: In --ai mode without auto_issue_management,
	// skip all remote issue operations. Reconcile ledger from spec files locally.
	if aiMode && !loaded.Config.AutoIssueManagement {
		if ledger, loadErr := LoadLedger(configDir); loadErr == nil {
			if syncErr := ledger.SyncFromSpecsDir(configDir); syncErr == nil {
				if saveErr := SaveLedger(ledger, configDir); saveErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save reconciled ledger: %v\n", saveErr)
				}
			}
		}
		return nil
	}

	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
		// Reconcile ledger from spec files even without remote access
		if ledger, loadErr := LoadLedger(configDir); loadErr == nil {
			if syncErr := ledger.SyncFromSpecsDir(configDir); syncErr == nil {
				if saveErr := SaveLedger(ledger, configDir); saveErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save reconciled ledger: %v\n", saveErr)
				}
			}
		}
		return nil
	}

	baseURL := loaded.Config.GitHub.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	coord := NewIssueCoordinator(loaded.Config.GitHub.Owner, loaded.Config.GitHub.Repo, loaded.Config.GitHub.Token, baseURL)
	coord.SetConfigDir(configDir)
	if loaded.Config.FailureDecaySeconds > 0 {
		coord.SetFailureDecay(loaded.Config.FailureDecaySeconds)
	}
	ledger, _ := LoadLedger(configDir)
	coord.SetLedger(ledger)

	if err := processSyncQueue(coord, configDir, ledger); err != nil {
		return RecordSyncFailure(configDir, specID, err.Error())
	}

	maxAgeDays := 7
	retryIntervalHours := 1
	if loaded.Config.SyncSchedule != nil {
		if loaded.Config.SyncSchedule.MaxAgeDays > 0 {
			maxAgeDays = loaded.Config.SyncSchedule.MaxAgeDays
		}
		if loaded.Config.SyncSchedule.RetryIntervalHours > 0 {
			retryIntervalHours = loaded.Config.SyncSchedule.RetryIntervalHours
		}
	}

	shouldFullSync, err := ShouldRunFullSync(configDir, maxAgeDays)
	if err != nil {
		return fmt.Errorf("checking sync schedule: %w", err)
	}

	// Promote review specs to "ship" BEFORE syncing with remote, so the sync
	// doesn't prematurely close them when it sees a previously-closed remote issue.
	promoteReviewSpecs(configDir, aiMode)

	var syncErr error
	if specID != "" {
		syncErr = syncSingleSpec(coord, configDir, specID, loaded.Config)
	} else {
		if shouldFullSync {
			if err := coord.SyncIssues(configDir); err != nil {
				syncErr = fmt.Errorf("issue sync failed: %w", err)
			}
			if syncErr == nil {
				if err := MarkFullSyncRun(configDir); err != nil {
					return fmt.Errorf("marking full sync run: %w", err)
				}
			}
		}
		if err := coord.SyncSpecs(configDir); err != nil {
			if syncErr != nil {
				syncErr = fmt.Errorf("%v; spec sync failed: %w", syncErr, err)
			} else {
				syncErr = fmt.Errorf("spec sync failed: %w", err)
			}
		}

		if err := DrainHousekeepingQueue(configDir, coord); err != nil {
			fmt.Fprintf(os.Stderr, "warning: housekeeping drain failed: %v\n", err)
		}
	}

	hasFailures, _ := HasPendingSyncFailures(configDir)
	if hasFailures {
		shouldRetry, err := ShouldRetryFailures(configDir, retryIntervalHours)
		if err != nil {
			return fmt.Errorf("checking retry schedule: %w", err)
		}
		if shouldRetry {
			if err := processSyncQueue(coord, configDir, ledger); err != nil {
				return RecordSyncFailure(configDir, specID, err.Error())
			}
			if err := MarkRetryAttempt(configDir); err != nil {
				return fmt.Errorf("marking retry attempt: %w", err)
			}
		}
	}

	if syncErr != nil {
		return RecordSyncFailure(configDir, specID, syncErr.Error())
	}
	return nil
}

func processSyncQueue(coord *IssueCoordinator, configDir string, ledger *LedgerEngine) error {
	ops, err := LoadSyncQueue(configDir)
	if err != nil || len(ops) == 0 {
		return nil
	}

	var lastErr error
	remaining := []SyncOperation{}
	processedSpecIDs := make(map[string]bool)

	for _, op := range ops {
		var opErr error
		switch op.Type {
		case SyncOpCreateIssue:
			opErr = processCreateIssue(coord, configDir, ledger, op)
		case SyncOpCloseIssue:
			opErr = processCloseIssue(coord, configDir, op)
		case SyncOpPostComment:
			opErr = processPostComment(coord, configDir, op)
		case SyncOpUpdateIssueBody:
			opErr = processUpdateIssueBody(coord, op)
		case SyncOpSyncSpec:
			// Deduplicate sync operations for the same specID.
			// If we've already processed a sync operation for this specID,
			// skip it to prevent duplicate sync attempts.
			if processedSpecIDs[op.SpecID] {
				fmt.Fprintf(os.Stderr, "debug: skipping duplicate sync operation for spec %s\n", op.SpecID)
				continue
			}
			opErr = syncSingleSpec(coord, configDir, op.SpecID, nil)
			if opErr == nil {
				processedSpecIDs[op.SpecID] = true
			}
		case SyncOpDeleteIssue:
			opErr = processDeleteIssue(coord, configDir, op)
		}

		if opErr != nil {
			if errors.Is(opErr, ErrResourceNotFound) {
				// Issue no longer exists on remote — drop operation immediately
				fmt.Fprintf(os.Stderr, "warning: %s for issue #%d not found (404/410), dropping from sync queue\n", op.Type, op.IssueNum)

				// Proactively clean up the spec's repo_issue field so no further
				// operations are enqueued for this deleted issue.
				if op.SpecID != "" {
					clearRepoIssueForSpec(configDir, op.SpecID, coord)
				} else if op.IssueNum > 0 {
					clearRepoIssueByNumber(configDir, op.IssueNum, coord)
				}
				continue
			}
			lastErr = opErr
			op.RetryCount++
			if op.RetryCount < 3 {
				remaining = append(remaining, op)
			} else {
				RecordSyncFailure(configDir, op.SpecID, fmt.Sprintf("op %s failed after 3 retries: %v", op.Type, opErr))
			}
		}
	}

	if len(remaining) > 0 {
		if err := SaveSyncQueue(configDir, remaining); err != nil {
			return err
		}
	} else {
		if err := ClearSyncQueue(configDir); err != nil {
			return err
		}
	}

	return lastErr
}

func processCreateIssue(coord *IssueCoordinator, configDir string, ledger *LedgerEngine, op SyncOperation) error {
	if op.Details == nil {
		return fmt.Errorf("create_issue requires details")
	}
	issue, _, err := coord.EnsureIssue(op.TestName, op.Details)
	if err != nil {
		return err
	}
	if ledger != nil && op.SpecID != "" {
		if entry := ledger.GetSpecEntry(op.SpecID); entry != nil {
			entry.RepoIssueID = issue.Number
			ledger.SetSpecEntry(op.SpecID, entry)
			if err := SaveLedger(ledger, configDir); err != nil {
				return fmt.Errorf("saving ledger: %w", err)
			}
		}
	}
	return nil
}

func processCloseIssue(coord *IssueCoordinator, configDir string, op SyncOperation) error {
	if op.IssueNum <= 0 {
		return fmt.Errorf("close_issue requires issue_number")
	}
	if err := coord.CloseIssueByNumber(op.IssueNum); err != nil {
		return err
	}
	DeleteAuditEntry(configDir, op.TestName)
	return nil
}

func processPostComment(coord *IssueCoordinator, configDir string, op SyncOperation) error {
	if op.IssueNum <= 0 || op.Body == "" {
		return fmt.Errorf("post_comment requires issue_number and body")
	}
	return coord.PostComment(op.IssueNum, op.Body)
}

func processUpdateIssueBody(coord *IssueCoordinator, op SyncOperation) error {
	if op.IssueNum <= 0 || op.Body == "" {
		return fmt.Errorf("update_issue_body requires issue_number and body")
	}
	return coord.UpdateIssueBody(op.IssueNum, op.Body)
}

func processDeleteIssue(coord *IssueCoordinator, configDir string, op SyncOperation) error {
	if op.IssueNum <= 0 {
		return nil
	}
	return coord.CloseIssueByNumber(op.IssueNum)
}

func syncSingleSpec(coord *IssueCoordinator, configDir, specID string, cfg *Config) error {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	ledger := coord.GetLedger()
	if ledger == nil {
		ledger, _ = LoadLedger(configDir)
	}

	if ledger != nil {
		coord.ensureCategoryLabelsExist(ledger.WorkflowConfig)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		// Skip archive files — they are aggregated spec dumps, not individual specs
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}

		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil {
			continue
		}

		if spec.SpecID != specID && specID != "" {
			continue
		}

		if spec.RepoIssue == 0 {
			// Match SyncSpecs behavior: find existing remote issue by title first
			if existing, err := coord.findRemoteIssueByTitle(spec.Title); err == nil {
				spec.RepoIssue = existing.Number
				fmt.Printf("Found existing remote issue #%d for spec: %s (skipping POST)\n", existing.Number, spec.Title)
			} else {
				coord.markSpecAsDuplicateIfNeeded(spec)
				title := prefixedTitle(spec)
				if !IsValidIssueTitle(title) {
					title = fmt.Sprintf("feat/spec: %s", title)
				}
				if len(title) > maxTitleLength {
					// Truncate at last word boundary to fit within maxTitleLength
					cutAt := maxTitleLength
					for cutAt > 10 && title[cutAt-1] != ' ' {
						cutAt--
					}
					if cutAt <= 10 {
						cutAt = maxTitleLength
					}
					title = strings.TrimSpace(title[:cutAt])
				}
				issue, err := coord.CreateIssueWithBody(title, effectiveSpecBody(spec))
				if err != nil {
					return fmt.Errorf("creating issue for spec %s: %w", spec.Title, err)
				}
				spec.RepoIssue = issue.Number
				fmt.Printf("Created issue #%d for spec: %s\n", issue.Number, spec.Title)
			}
		} else {
			remoteIssue, err := coord.GetIssueByNumber(spec.RepoIssue)
			if err != nil {
				if errors.Is(err, ErrResourceNotFound) {
					// Issue was deleted from remote — clear the reference so next sync recreates it
					fmt.Fprintf(os.Stderr, "warning: issue #%d for spec %q not found (deleted), clearing reference\n", spec.RepoIssue, spec.Title)
					updateSpecFileRepoIssue(filePath, 0)
					spec.RepoIssue = 0
				} else {
					return fmt.Errorf("fetching issue #%d: %w", spec.RepoIssue, err)
				}
			}
			if remoteIssue != nil {
				desiredBody := effectiveSpecBody(spec)
				if normalizeWhitespace(remoteIssue.Body) != normalizeWhitespace(desiredBody) {
					if err := coord.UpdateIssueBody(spec.RepoIssue, desiredBody); err != nil {
						return fmt.Errorf("updating issue #%d: %w", spec.RepoIssue, err)
					}
				}
				if isResolvedStatus(spec.Status) && remoteIssue.State == "open" {
					if err := coord.PostResolutionComment(spec.RepoIssue, spec); err != nil {
						return fmt.Errorf("posting resolution comment on issue #%d: %w", spec.RepoIssue, err)
					}
					if err := coord.CloseIssueByNumber(spec.RepoIssue); err != nil {
						return fmt.Errorf("closing issue #%d: %w", spec.RepoIssue, err)
					}
				}
			}
		}

		if err := updateSpecFileRepoIssue(filePath, spec.RepoIssue); err != nil {
			return fmt.Errorf("updating spec file: %w", err)
		}

		if ledger != nil && spec.SpecID != "" {
			ledger.SetSpecEntry(spec.SpecID, &SpecEntry{
				SpecID:        spec.Title,
				RepoIssueID:   spec.RepoIssue,
				Status:        spec.Status,
				LinkedCommits: []string{},
			})
			if err := SaveLedger(ledger, configDir); err != nil {
				return fmt.Errorf("saving ledger: %w", err)
			}
		}

		coord.syncSpecLabels(spec, ledger.WorkflowConfig)
	}

	return nil
}

// field to 0, both in the spec file and in the ledger. This prevents future
// sync operations from being enqueued for a deleted remote issue.
func clearRepoIssueForSpec(configDir, specID string, coord *IssueCoordinator) {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil || spec.SpecID != specID {
			continue
		}
		if spec.RepoIssue == 0 {
			return
		}
		if err := updateSpecFileRepoIssue(filePath, 0); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clear repo_issue for spec %s: %v\n", specID, err)
			return
		}
		// Update ledger as well
		ledger := coord.GetLedger()
		if ledger == nil {
			ledger, _ = LoadLedger(configDir)
		}
		if ledger != nil {
			if entry := ledger.GetSpecEntry(specID); entry != nil {
				entry.RepoIssueID = 0
				ledger.SetSpecEntry(specID, entry)
				if err := SaveLedger(ledger, configDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save ledger after clearing repo_issue: %v\n", err)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "debug: cleared repo_issue for spec %s (issue was deleted on remote)\n", specID)
		return
	}
}

// clearRepoIssueByNumber finds a spec file by its repo_issue field value and clears
// it to 0. Used when a sync queue operation fails with 404 but has no SpecID.
func clearRepoIssueByNumber(configDir string, issueNum int, coord *IssueCoordinator) {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(de.Name(), "archive_") {
			continue
		}
		filePath := filepath.Join(specDir, de.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil || spec.RepoIssue != issueNum {
			continue
		}
		if err := updateSpecFileRepoIssue(filePath, 0); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clear repo_issue for %s: %v\n", spec.SpecID, err)
			return
		}
		// Update ledger as well
		ledger := coord.GetLedger()
		if ledger == nil {
			ledger, _ = LoadLedger(configDir)
		}
		if ledger != nil {
			if se := ledger.GetSpecEntry(spec.SpecID); se != nil {
				se.RepoIssueID = 0
				ledger.SetSpecEntry(spec.SpecID, se)
				if err := SaveLedger(ledger, configDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save ledger after clearing repo_issue: %v\n", err)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "debug: cleared repo_issue for issue #%d (deleted on remote)\n", issueNum)
		return
	}
}

func isResolvedStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "closed", "done", "fixed":
		return true
	default:
		return false
	}
}

// specMetadataSyncer implements housekeeper.MetadataSyncer to update spec
// files and the ledger from "ship" to "closed" after a successful push.
type specMetadataSyncer struct {
	configDir string
}

func (s *specMetadataSyncer) SyncMetadata(specID string) error {
	var filePath string

	specDir := filepath.Join(s.configDir, "specs")
	if _, err := os.Stat(specDir); os.IsNotExist(err) {
		specDir = filepath.Join(filepath.Dir(s.configDir), "specs")
	}
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return fmt.Errorf("reading specs dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fp := filepath.Join(specDir, entry.Name())
		data, err := os.ReadFile(fp)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(string(data), "---") {
			continue
		}
		parts := strings.SplitN(string(data), "---", 3)
		if len(parts) < 3 {
			continue
		}
		if strings.Contains(parts[1], specID) {
			filePath = fp
			break
		}
	}

	if filePath == "" {
		return fmt.Errorf("spec %s not found on disk", specID)
	}

	if err := UpdateSpecFileStatus(filePath, "closed"); err != nil {
		return fmt.Errorf("updating spec file: %w", err)
	}

	ledger, err := LoadLedger(s.configDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}
	entry := ledger.GetSpecEntry(specID)
	if entry == nil {
		return fmt.Errorf("spec %s not found in ledger", specID)
	}
	entry.Status = "closed"
	ledger.SetSpecEntry(specID, entry)
	return SaveLedger(ledger, s.configDir)
}

// DrainHousekeepingQueue processes all pending housekeeping tasks using the
// given IssueCoordinator as the backend for comment posting and issue closing.
func DrainHousekeepingQueue(configDir string, coord *IssueCoordinator) error {
	if coord == nil || coord.isInactive() {
		return nil
	}
	hq := housekeeper.NewHousekeepingQueue(configDir)
	if err := hq.Load(); err != nil {
		return fmt.Errorf("loading housekeeping queue: %w", err)
	}
	if hq.Len() == 0 {
		return nil
	}

	// Report any previously FAILED tasks for audit trail
	for _, t := range hq.GetAll() {
		if t.Status == housekeeper.StatusFailed {
			fmt.Fprintf(os.Stderr, "[HOUSEKEEPER] Task %s (%s) for spec %s on issue #%d FAILED after %d attempts: %s\n",
				t.ID, t.Type, t.SpecID, t.RepoIssueID, t.Attempts, t.LastError)
		}
	}

	syncer := &specMetadataSyncer{configDir: configDir}
	ctx := context.Background()
	registry := housekeeper.NewDefaultRegistry(coord, coord, syncer)
	return hq.Process(ctx, registry)
}

// DrainHousekeepingQueueFromConfig loads the pipeline config, creates an
// IssueCoordinator, and drains the housekeeping queue.
// Pass aiMode=true to respect the auto_issue_management gate in --ai mode.
func DrainHousekeepingQueueFromConfig(configDir string, aiMode bool) error {
	loaded, err := LoadPipelineConfig(configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// AUTO_ISSUE_MANAGEMENT GATE: In --ai mode, skip remote issue operations
	// when auto_issue_management is disabled.
	if aiMode && !loaded.Config.AutoIssueManagement {
		return nil
	}

	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
		// Reconcile ledger from spec files even without remote access
		if ledger, loadErr := LoadLedger(configDir); loadErr == nil {
			if syncErr := ledger.SyncFromSpecsDir(configDir); syncErr == nil {
				if saveErr := SaveLedger(ledger, configDir); saveErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save reconciled ledger: %v\n", saveErr)
				}
			}
		}
		return nil
	}
	baseURL := loaded.Config.GitHub.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	coord := NewIssueCoordinator(loaded.Config.GitHub.Owner, loaded.Config.GitHub.Repo, loaded.Config.GitHub.Token, baseURL)
	coord.SetConfigDir(configDir)
	return DrainHousekeepingQueue(configDir, coord)
}

// promoteReviewSpecs checks the ledger for specs in "review" status and prompts
// the user to confirm manual testing before advancing to "ship". Uses the
// spec title (from the spec file) for the prompt, not the opaque spec ID.
// Skips if stdin is not a terminal unless aiMode is true (auto-promote).
func promoteReviewSpecs(configDir string, aiMode bool) {
	if !term.IsTerminal(int(os.Stdin.Fd())) && !aiMode {
		return
	}

	ledger, err := LoadLedger(configDir)
	if err != nil {
		return
	}

	specDir := filepath.Join(configDir, "specs")

	type candidate struct {
		id       string
		title    string
		filePath string
	}
	var candidates []candidate

	for specID, entry := range ledger.GetAllSpecEntries() {
		if entry.Status != "review" {
			continue
		}

		title := entry.SpecID
		var fp string
		if entries, readErr := os.ReadDir(specDir); readErr == nil {
			for _, de := range entries {
				if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
					continue
				}
				fpath := filepath.Join(specDir, de.Name())
				data, readErr := os.ReadFile(fpath)
				if readErr != nil {
					continue
				}
				content := string(data)
				if strings.Contains(content, fmt.Sprintf(`spec_id: "%s"`, specID)) {
					fp = fpath
					for _, line := range strings.Split(content, "\n") {
						if strings.HasPrefix(strings.TrimSpace(line), "# ") {
							title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
							break
						}
					}
					break
				}
			}
		}
		candidates = append(candidates, candidate{id: specID, title: title, filePath: fp})
	}

	if len(candidates) == 0 {
		return
	}

	fmt.Fprintf(os.Stderr, "\nThe following specs are ready to ship:\n")
	for _, c := range candidates {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", c.id, c.title)
	}
	if aiMode {
		fmt.Fprintf(os.Stderr, "Auto-promoting %d spec(s) to ship (--ai mode)\n", len(candidates))
	} else {
		fmt.Fprintf(os.Stderr, "\nPromote these to \"ship\"? [y/N]: ")
		var response string
		if _, scanErr := fmt.Scanln(&response); scanErr != nil {
			return
		}
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			return
		}
	}

	for _, c := range candidates {
		entry := ledger.GetSpecEntry(c.id)
		if entry == nil {
			continue
		}
		entry.Status = "ship"
		ledger.SetSpecEntry(c.id, entry)

		if c.filePath != "" {
			if err := UpdateSpecFileStatus(c.filePath, "ship"); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to update spec file %s: %v\n", c.filePath, err)
			}
		}
	}

	if err := SaveLedger(ledger, configDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save ledger after promotion: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Promoted %d spec(s) to ship\n", len(candidates))
}
