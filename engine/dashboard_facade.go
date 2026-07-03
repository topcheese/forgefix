package engine

import (
	"sync"
	"sync/atomic"
)

// DashboardFacade provides backward-compatible interface to the new decomposed services.
// This allows gradual migration without breaking all consumers at once.
type DashboardFacade struct {
	// New services
	testTracker    *TestTrackerService
	pipelineMgr    *PipelineManager
	ledger         *LedgerEngine
	errorTracker   *ErrorTracker
	
	// Legacy state (for backward compat)
	mu                    sync.RWMutex
	PipelineActive        bool
	Bomb                  BombState
	BombFrame             int
	stopCh                chan struct{}
	stopOnce              sync.Once
	dirty                 atomic.Int32
	Coord                 *IssueCoordinator
	IssueRefs             map[string]*IssueInfo
	FailureDecaySecs      int
	TestCommandCompleted  bool
	ConfigDir             string
}

func NewDashboardFacade(pipelines []PipelineConfig, configDir string) *DashboardFacade {
	dirtySignal := func() {}
	f := &DashboardFacade{
		testTracker:      NewTestTrackerService(dirtySignal),
		pipelineMgr:      NewPipelineManager(pipelines),
		ledger:           NewLedgerEngine(),
		errorTracker:     NewErrorTracker(),
		PipelineActive:   true,
		stopCh:           make(chan struct{}),
		IssueRefs:        make(map[string]*IssueInfo),
		FailureDecaySecs: 15,
		ConfigDir:        configDir,
	}
	// Wire up dirty signal after construction
	f.testTracker = NewTestTrackerService(f.markDirty)
	return f
}

// markDirty implements the dirty signal for test tracker
func (f *DashboardFacade) markDirty() {
	f.dirty.Store(1)
}

// ========== Delegated methods ==========

func (f *DashboardFacade) GetTracker(pipelineID string) *TestTracker {
	return f.testTracker.GetTracker(pipelineID)
}

func (f *DashboardFacade) ResetTrackers() {
	f.testTracker.ResetTrackers()
}

func (f *DashboardFacade) MarkPipelineSkipped(id string) {
	f.pipelineMgr.MarkSkipped(id)
}

func (f *DashboardFacade) IsPipelineSkipped(id string) bool {
	return f.pipelineMgr.IsSkipped(id)
}

func (f *DashboardFacade) AddSystemError(msg string) {
	f.errorTracker.AddSystemError(msg)
	f.markDirty()
}

func (f *DashboardFacade) GetSystemErrors() []string {
	return f.errorTracker.GetSystemErrors()
}

func (f *DashboardFacade) AddErrorLog(exitCode int) {
	f.errorTracker.AddErrorLog(exitCode)
}

func (f *DashboardFacade) UpdatePipelineMetrics(pipelineID string, action string, testID string, elapsed int, result string, testName string) {
	f.testTracker.UpdateMetrics(pipelineID, action, testID, testName, elapsed, result, "", "", 0, 0)
	if testName != "" && (action == "pass" || action == "fail") {
		entry := f.ledger.GetOrCreateEntry(pipelineID)
		ran := entry.TotalRan + 1
		passed := entry.TotalPassed
		failed := entry.TotalFailed
		if action == "pass" {
			passed++
		} else {
			failed++
		}
		f.ledger.UpdateEntry(pipelineID, ran, passed, failed)
	}
}

func (f *DashboardFacade) UpdatePipelineMetricsWithDetails(pipelineID string, action string, testID string, elapsed int, result string, testName string, errorTrace string, filePath string, failureLine int, failureColumn int) {
	f.testTracker.UpdateMetrics(pipelineID, action, testID, testName, elapsed, result, errorTrace, filePath, failureLine, failureColumn)
	if testName != "" && (action == "pass" || action == "fail") {
		entry := f.ledger.GetOrCreateEntry(pipelineID)
		ran := entry.TotalRan + 1
		passed := entry.TotalPassed
		failed := entry.TotalFailed
		if action == "pass" {
			passed++
		} else {
			failed++
		}
		f.ledger.UpdateEntry(pipelineID, ran, passed, failed)
	}
}

func (f *DashboardFacade) GetMetrics(pipelineID string) (ran int, passed int, failed int, active map[string]*TestInfo, completed map[string]*TestInfo) {
	ran, passed, failed, active, completed = f.testTracker.GetMetrics(pipelineID)
	// Also get ledger entry for historical floor
	entry := f.ledger.GetEntry(pipelineID)
	if entry != nil {
		// Override with ledger data
		ran = entry.TotalRan
		passed = entry.TotalPassed
		failed = entry.TotalFailed
	}
	return ran, passed, failed, active, completed
}

func (f *DashboardFacade) GetTotalFailures() int {
	return f.ledger.GetTotalFailed()
}

func (f *DashboardFacade) SetPipelineActive(active bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PipelineActive = active
}

func (f *DashboardFacade) GetActivePipelines() []PipelineConfig {
	return f.pipelineMgr.GetActivePipelines()
}

func (f *DashboardFacade) GetErrorLogs() []ErrorLog {
	return f.errorTracker.GetErrorLogs()
}

func (f *DashboardFacade) AddErrorCode(code int) {
	f.errorTracker.AddErrorCode(code)
}

func (f *DashboardFacade) GetExitCodes() []int {
	// Not tracked in new service yet
	return []int{}
}

func (f *DashboardFacade) GetFailureDecaySeconds() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.FailureDecaySecs > 0 {
		return f.FailureDecaySecs
	}
	return 15
}

func (f *DashboardFacade) StopCh() <-chan struct{} {
	return f.stopCh
}

func (f *DashboardFacade) IsDirty() bool {
	return f.dirty.Load() == 1
}

func (f *DashboardFacade) ClearDirty() {
	f.dirty.Store(0)
}

// ========== Ledger delegation ==========

func (f *DashboardFacade) GetOrCreateLedgerEntry(pipelineID string) *LedgerEntry {
	return f.ledger.GetOrCreateEntry(pipelineID)
}

func (f *DashboardFacade) UpdateLedgerEntry(pipelineID string, ran, passed, failed int) {
	f.ledger.UpdateEntry(pipelineID, ran, passed, failed)
}

func (f *DashboardFacade) GetLedgerEntry(pipelineID string) *LedgerEntry {
	return f.ledger.GetEntry(pipelineID)
}

func (f *DashboardFacade) ResetLedgerCurrentRun() {
	f.ledger.ResetCurrentRun()
}

func (f *DashboardFacade) GetTotalRan() int {
	return f.ledger.GetTotalRan()
}

func (f *DashboardFacade) GetTotalPassed() int {
	return f.ledger.GetTotalPassed()
}

func (f *DashboardFacade) GetTotalFailed() int {
	return f.ledger.GetTotalFailed()
}

func (f *DashboardFacade) GetTotalFloor() int {
	return f.ledger.GetTotalFloor()
}

func (f *DashboardFacade) FormatLedgerSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt string) string {
	return f.ledger.FormatSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt)
}

// ========== Backward-compat helper for rendering ==========

func (f *DashboardFacade) GetPipelines() []PipelineConfig {
	return f.pipelineMgr.GetPipelines()
}

// Helper to check if there are any failures
func (f *DashboardFacade) HasFailures() bool {
	return f.GetTotalFailures() > 0
}

// Helper to get failure decay seconds
func (f *DashboardFacade) GetFailureDecay() int {
	return f.GetFailureDecaySeconds()
}

// For watcher.go compatibility
func (f *DashboardFacade) GetPipelineConfig(id string) *PipelineConfig {
	for _, p := range f.pipelineMgr.GetPipelines() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

// FormatSummary for backward compat with rendering
func (f *DashboardFacade) FormatSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt string) string {
	return f.FormatLedgerSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt)
}

// Helper methods for rendering
func (f *DashboardFacade) GetSkippedPipelines() map[string]bool {
	return f.pipelineMgr.GetSkippedPipelines()
}

func (f *DashboardFacade) GetTestTrackers() map[string]*TestTracker {
	return f.testTracker.GetAllTrackers()
}

func (f *DashboardFacade) GetSystemErrorLogs() []ErrorLog {
	return f.errorTracker.GetErrorLogs()
}

// LEDGER COMPAT - expose ledger directly for legacy code
func (f *DashboardFacade) GetLedger() *LedgerEngine {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ledger
}

func (f *DashboardFacade) SetLedger(ledger *LedgerEngine) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ledger = ledger
}

// Backward compat fields
func (f *DashboardFacade) GetPipelinesSlice() []PipelineConfig {
	return f.pipelineMgr.GetPipelines()
}

func (f *DashboardFacade) GetSkippedPipelinesMap() map[string]bool {
	// Build from pipeline manager
	skipped := make(map[string]bool)
	for _, p := range f.pipelineMgr.GetPipelines() {
		if f.pipelineMgr.IsSkipped(p.ID) {
			skipped[p.ID] = true
		}
	}
	return skipped
}

func (f *DashboardFacade) GetTestTrackersMap() map[string]*TestTracker {
	return f.testTracker.GetAllTrackers()
}

func (f *DashboardFacade) SetTestTracker(pipelineID string, tracker *TestTracker) {
	f.testTracker.SetTracker(pipelineID, tracker)
}

// For execute.go collectFailedTests
func (f *DashboardFacade) CollectFailedTests() []TestInfo {
	var failed []TestInfo
	// Get from test tracker
	for _, p := range f.pipelineMgr.GetPipelines() {
		ran, passed, failedCount, _, completed := f.testTracker.GetMetrics(p.ID)
		_ = ran; _ = passed; _ = failedCount
		for _, info := range completed {
			if info.State == StateDud {
				failed = append(failed, *info)
			}
		}
	}
	return failed
}

func (f *DashboardFacade) GetBomb() BombState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Bomb
}

func (f *DashboardFacade) SetBomb(state BombState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Bomb = state
}

func (f *DashboardFacade) GetBombFrame() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.BombFrame
}

func (f *DashboardFacade) SetBombFrame(frame int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.BombFrame = frame
}

func (f *DashboardFacade) IncrementBombFrame() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.BombFrame++
}

func (f *DashboardFacade) GetTimeoutFired() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.TestCommandCompleted // repurposed
}

func (f *DashboardFacade) SetTimeoutFired(fired bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.TestCommandCompleted = fired
}

func (f *DashboardFacade) GetConfigDir() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.ConfigDir
}

func (f *DashboardFacade) SetConfigDir(dir string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ConfigDir = dir
}

func (f *DashboardFacade) GetCoord() *IssueCoordinator {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.Coord
}

func (f *DashboardFacade) SetCoord(coord *IssueCoordinator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Coord = coord
}

func (f *DashboardFacade) GetIssueRefs() map[string]*IssueInfo {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.IssueRefs
}

func (f *DashboardFacade) SetIssueRefs(refs map[string]*IssueInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.IssueRefs = refs
}

// NewDashboard creates a new DashboardFacade (backward compatible constructor)
func NewDashboard(pipelines []PipelineConfig) *DashboardFacade {
	return NewDashboardFacade(pipelines, "")
}