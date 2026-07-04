package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"ForgeFix/engine/housekeeper"
)

type SyncFailure struct {
	Timestamp  time.Time `json:"timestamp"`
	SpecID     string    `json:"spec_id,omitempty"`
	Error      string    `json:"error"`
	RetryCount int       `json:"retry_count"`
}

type SyncScheduleState struct {
	LastFullSync    time.Time `json:"last_full_sync,omitempty"`
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
)

type SyncOperation struct {
	ID        string     `json:"id"`
	Type      SyncOpType `json:"type"`
	Timestamp time.Time  `json:"timestamp"`
	SpecID    string     `json:"spec_id,omitempty"`
	TestName  string     `json:"test_name,omitempty"`
	IssueNum  int        `json:"issue_number,omitempty"`
	Title     string     `json:"title,omitempty"`
	Body      string     `json:"body,omitempty"`
	Details   *ErrorDetails `json:"details,omitempty"`
	RetryCount int       `json:"retry_count"`
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
		Type:      SyncOpCloseIssue,
		TestName:  testName,
		IssueNum:  issueNumber,
	})
}

func QueuePostComment(configDir, testName string, issueNumber int, title, body string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:      SyncOpPostComment,
		TestName:  testName,
		IssueNum:  issueNumber,
		Title:     title,
		Body:      body,
	})
}

func QueueUpdateIssueBody(configDir string, issueNumber int, body string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:      SyncOpUpdateIssueBody,
		IssueNum:  issueNumber,
		Body:      body,
	})
}

func QueueSyncSpec(configDir, specID string) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:    SyncOpSyncSpec,
		SpecID:  specID,
	})
}

func QueueDeleteIssue(configDir, specID string, issueNumber int) error {
	return EnqueueSyncOp(configDir, SyncOperation{
		Type:     SyncOpDeleteIssue,
		SpecID:   specID,
		IssueNum: issueNumber,
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

func RunBackgroundSync(configDir, specID string) error {
	loaded, err := LoadPipelineConfig(configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
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
			opErr = syncSingleSpec(coord, configDir, op.SpecID, nil)
		case SyncOpDeleteIssue:
			opErr = processDeleteIssue(coord, configDir, op)
		}

		if opErr != nil {
			lastErr = opErr
			if errors.Is(opErr, ErrResourceNotFound) {
				// Issue no longer exists on remote — drop operation immediately
				fmt.Fprintf(os.Stderr, "warning: %s for issue #%d returned 404, dropping from sync queue\n", op.Type, op.IssueNum)
				continue
			}
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

		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil {
			continue
		}

		if spec.SpecID != specID && specID != "" {
			continue
		}

		if spec.RepoIssue == 0 {
			title := spec.Title
			if !IsValidIssueTitle(title) {
				title = fmt.Sprintf("feat/spec: %s", title)
			}
			issue, err := coord.CreateIssueWithBody(title, spec.Body)
			if err != nil {
				return fmt.Errorf("creating issue for spec %s: %w", spec.Title, err)
			}
			spec.RepoIssue = issue.Number
		} else {
			remoteIssue, err := coord.GetIssueByNumber(spec.RepoIssue)
			if err != nil {
				return fmt.Errorf("fetching issue #%d: %w", spec.RepoIssue, err)
			}
			if remoteIssue != nil {
				if remoteIssue.Body != spec.Body {
					if err := coord.UpdateIssueBody(spec.RepoIssue, spec.Body); err != nil {
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
				SpecID:       spec.SpecID,
				RepoIssueID:  spec.RepoIssue,
				Status:       spec.Status,
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

func SpawnBackgroundSync(configDir, specID string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}

	args := []string{"sync"}
	if specID != "" {
		args = append(args, "--spec", specID)
	}

	cmd := exec.Command(exe, args...)
	cmd.Dir = configDir

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting background sync: %w", err)
	}

	return cmd.Process.Release()
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
// files from "ship" to "closed" after a successful push.
type specMetadataSyncer struct {
	configDir string
}

func (s *specMetadataSyncer) SyncMetadata(specID string) error {
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
		filePath := filepath.Join(specDir, entry.Name())
		data, err := os.ReadFile(filePath)
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
		if !strings.Contains(parts[1], specID) {
			continue
		}
		// Found the matching spec file — update status to closed
		updated := strings.Replace(
			content,
			`status: "ship"`,
			`status: "closed"`,
			1,
		)
		if updated == content {
			// Try without quotes
			updated = strings.Replace(
				content,
				"status: ship",
				"status: closed",
				1,
			)
		}
		if updated != content {
			return os.WriteFile(filePath, []byte(updated), 0644)
		}
		return nil
	}
	return fmt.Errorf("spec %s not found", specID)
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
func DrainHousekeepingQueueFromConfig(configDir string) error {
	loaded, err := LoadPipelineConfig(configDir)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if loaded.Config.GitHub == nil || loaded.Config.GitHub.Token == "" || loaded.Config.GitHub.Owner == "" || loaded.Config.GitHub.Repo == "" {
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