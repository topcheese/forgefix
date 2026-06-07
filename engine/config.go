package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const maxDisplayHistory = 15

var (
	Red       = "\033[31m"
	Reset     = "\033[0m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	White     = "\033[37m"
	Bold      = "\033[1m"
	Underline = "\033[4m"
)

// ============================================================================
// DECLARATIVE PIPELINE SCHEMA
// ============================================================================

type PipelineConfig struct {
	ID             string        `yaml:"id"`
	Name           string        `yaml:"name"`
	Description    string        `yaml:"description"`
	PanelColor     string        `yaml:"panel_color"`
	Type           string        `yaml:"type"`
	Command        CommandConfig `yaml:"command"`
	TokenPatterns  TokenPatterns `yaml:"token_patterns"`
	TimeoutSeconds int           `yaml:"timeout_seconds"`
	LedgerFloor    int           `yaml:"ledger_floor"`
}

type LanguageConfig struct {
	RootAnchor    string        `yaml:"root_anchor"`
	TestCommand   string        `yaml:"test_command"`
	TokenPatterns TokenPatterns `yaml:"token_patterns"`
}

type LanguageMap map[string]LanguageConfig

type Config struct {
	Pipelines           []PipelineConfig `yaml:"pipelines"`
	Languages           LanguageMap      `yaml:"languages"`
	ExcludeDirs         []string         `yaml:"exclude_dirs"`
	GlobalTimeoutSeconds int             `yaml:"global_timeout_seconds"`
}

type CommandConfig struct {
	Type  string   `yaml:"type"`
	Args  []string `yaml:"args"`
	Paths []string `yaml:"paths"`
}

type TokenPatterns struct {
	TokenRun  string `yaml:"token_run"`
	TokenPass string `yaml:"token_pass"`
	TokenFail string `yaml:"token_fail"`
}

// ============================================================================
// GENERIC TEST EVENT STRUCTURES
// ============================================================================

type GenericTestEvent struct {
	RawJSON      map[string]interface{}
	MatchedToken string
	TokenType    string
	TestID       string
	TestName     string
	Elapsed      int
}

// ============================================================================
// TEST STATE TRACKING
// ============================================================================

type TestState int

const (
	StatePending TestState = iota
	StateRunning
	StateCompleted
	StateSkipped
)

type TestInfo struct {
	ID      string
	Name    string
	State   TestState
	Started time.Time
	Elapsed int
}

type TestTracker struct {
	mu            sync.RWMutex
	ActiveTests   map[string]*TestInfo
	Completed     map[string]*TestInfo
	CompletedIDs  map[string]bool
	History       []string
	Ran           int
	Passed        int
}

type TestResult struct {
	Name    string
	State   TestState
	Elapsed int
	Success bool
}

// ============================================================================
// DYNAMIC LEDGER ENGINE
// ============================================================================

type LedgerEntry struct {
	PipelineID      string `json:"pipeline_id"`
	TotalRan        int    `json:"total_ran"`
	TotalPassed     int    `json:"total_passed"`
	TotalFailed     int    `json:"total_failed"`
	HistoricalFloor int    `json:"historical_floor"`
	LastUpdate      string `json:"last_update"`
}

type LedgerEngine struct {
	mu      sync.RWMutex
	entries map[string]*LedgerEntry
}

func NewLedgerEngine() *LedgerEngine {
	return &LedgerEngine{
		entries: make(map[string]*LedgerEntry),
	}
}

func (le *LedgerEngine) GetOrCreateEntry(pipelineID string) *LedgerEntry {
	le.mu.Lock()
	defer le.mu.Unlock()
	if _, exists := le.entries[pipelineID]; !exists {
		le.entries[pipelineID] = &LedgerEntry{
			PipelineID:      pipelineID,
			TotalRan:        0,
			TotalPassed:     0,
			TotalFailed:     0,
			HistoricalFloor: 0,
			LastUpdate:      time.Now().Format(time.RFC3339),
		}
	}
	return le.entries[pipelineID]
}

func (le *LedgerEngine) UpdateEntry(pipelineID string, ran int, passed int, failed int) {
	le.mu.Lock()
	defer le.mu.Unlock()
	if _, exists := le.entries[pipelineID]; !exists {
		le.entries[pipelineID] = &LedgerEntry{
			PipelineID:      pipelineID,
			TotalRan:        0,
			TotalPassed:     0,
			TotalFailed:     0,
			HistoricalFloor: 0,
			LastUpdate:      time.Now().Format(time.RFC3339),
		}
	}
	entry := le.entries[pipelineID]
	entry.TotalRan = ran
	entry.TotalPassed = passed
	entry.TotalFailed = failed
	entry.LastUpdate = time.Now().Format(time.RFC3339)
	if passed > entry.HistoricalFloor {
		entry.HistoricalFloor = passed
	}
}

func (le *LedgerEngine) GetEntry(pipelineID string) *LedgerEntry {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.entries[pipelineID]
}

func (le *LedgerEngine) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return le.LoadFromJSON(data)
}

func (le *LedgerEngine) SaveToFile(path string) error {
	data, err := json.MarshalIndent(le.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (le *LedgerEngine) LoadFromJSON(data []byte) error {
	var entries map[string]*LedgerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	le.mu.Lock()
	defer le.mu.Unlock()
	le.entries = entries
	return nil
}

func (le *LedgerEngine) ResetCurrentRun() {
	le.mu.Lock()
	defer le.mu.Unlock()
	for _, entry := range le.entries {
		// Preserve historical floors while resetting current run counters
		// This prevents false regression detection before tests complete
		historicalFloor := entry.HistoricalFloor
		entry.TotalRan = 0
		entry.TotalPassed = 0
		entry.TotalFailed = 0
		entry.HistoricalFloor = historicalFloor
	}
}

func (le *LedgerEngine) GetTotalRan() int {
	le.mu.RLock()
	defer le.mu.RUnlock()
	total := 0
	for _, entry := range le.entries {
		total += entry.TotalRan
	}
	return total
}

func (le *LedgerEngine) GetTotalPassed() int {
	le.mu.RLock()
	defer le.mu.RUnlock()
	total := 0
	for _, entry := range le.entries {
		total += entry.TotalPassed
	}
	return total
}

func (le *LedgerEngine) GetTotalFailed() int {
	le.mu.RLock()
	defer le.mu.RUnlock()
	total := 0
	for _, entry := range le.entries {
		total += entry.TotalFailed
	}
	return total
}

func (le *LedgerEngine) GetTotalFloor() int {
	le.mu.RLock()
	defer le.mu.RUnlock()
	total := 0
	for _, entry := range le.entries {
		total += entry.HistoricalFloor
	}
	return total
}

// ============================================================================
// DASHBOARD
// ============================================================================

type Dashboard struct {
	mu               sync.RWMutex
	Pipelines        []PipelineConfig
	TestTrackers     map[string]*TestTracker
	Ledger           *LedgerEngine
	ErrorLogs        []ErrorLog
	PipelineActive   bool
	errorCodes       []int
	SkippedPipelines map[string]bool
	SystemErrors     []string
	TimeoutFired     bool
	Bomb             BombState
	BombFrame        int
}

type ErrorLog struct {
	Timestamp time.Time
	Message   string
	ExitCode  int
}

func NewDashboard(pipelines []PipelineConfig) *Dashboard {
	return &Dashboard{
		Pipelines:      pipelines,
		TestTrackers:   make(map[string]*TestTracker),
		Ledger:         NewLedgerEngine(),
		PipelineActive: true,
	}
}

func (d *Dashboard) AddErrorCode(code int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.errorCodes = append(d.errorCodes, code)
}

func (d *Dashboard) GetExitCodes() []int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	clone := make([]int, len(d.errorCodes))
	copy(clone, d.errorCodes)
	return clone
}

func (d *Dashboard) GetTracker(pipelineID string) *TestTracker {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.TestTrackers[pipelineID]; !exists {
		d.TestTrackers[pipelineID] = &TestTracker{
			ActiveTests:  make(map[string]*TestInfo),
			Completed:    make(map[string]*TestInfo),
			CompletedIDs: make(map[string]bool),
			History:      make([]string, 0),
		}
	}
	return d.TestTrackers[pipelineID]
}

func (d *Dashboard) ResetTrackers() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, tracker := range d.TestTrackers {
		tracker.ActiveTests = make(map[string]*TestInfo)
		tracker.Completed = make(map[string]*TestInfo)
		tracker.CompletedIDs = make(map[string]bool)
		tracker.History = make([]string, 0)
	}
}

func (d *Dashboard) MarkPipelineSkipped(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.SkippedPipelines == nil {
		d.SkippedPipelines = make(map[string]bool)
	}
	d.SkippedPipelines[id] = true
}

func (d *Dashboard) IsPipelineSkipped(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.SkippedPipelines[id]
}

func (d *Dashboard) AddSystemError(msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.SystemErrors = append(d.SystemErrors, msg)
}

func (d *Dashboard) GetSystemErrors() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	clone := make([]string, len(d.SystemErrors))
	copy(clone, d.SystemErrors)
	return clone
}

func (d *Dashboard) AddErrorLog(exitCode int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ErrorLogs = append(d.ErrorLogs, ErrorLog{
		Timestamp: time.Now(),
		Message:   "Pipeline execution failed",
		ExitCode:  exitCode,
	})
}

func (d *Dashboard) UpdatePipelineMetrics(pipelineID string, action string, testID string, elapsed int, result string, testName string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	tracker, exists := d.TestTrackers[pipelineID]
	if !exists {
		tracker = &TestTracker{
			ActiveTests:  make(map[string]*TestInfo),
			Completed:    make(map[string]*TestInfo),
			CompletedIDs: make(map[string]bool),
			History:      make([]string, 0),
		}
		d.TestTrackers[pipelineID] = tracker
	}

	switch action {
	case "run":
		if _, exists := tracker.ActiveTests[testID]; !exists {
			tracker.ActiveTests[testID] = &TestInfo{
				ID:      testID,
				Name:    testID,
				State:   StateRunning,
				Started: time.Now(),
			}
		}
	case "pass":
		if tracker.CompletedIDs[testID] {
			return
		}
		tracker.CompletedIDs[testID] = true
		if info, exists := tracker.ActiveTests[testID]; exists {
			info.State = StateCompleted
			info.Elapsed = elapsed
		} else {
			tracker.ActiveTests[testID] = &TestInfo{
				ID:      testID,
				Name:    testID,
				State:   StateRunning,
				Started: time.Now(),
			}
		}
		// Only increment metrics if TestName is present (verified test function identifier)
		if testName != "" {
			entry := d.Ledger.GetOrCreateEntry(pipelineID)
			d.Ledger.UpdateEntry(pipelineID, entry.TotalRan+1, entry.TotalPassed+1, entry.TotalFailed)
		}
		if info, exists := tracker.ActiveTests[testID]; exists {
			info.State = StateCompleted
			tracker.Completed[testID] = info
			delete(tracker.ActiveTests, testID)
		}
		tracker.History = append(tracker.History, "✓ "+testID)
	case "fail":
		if tracker.CompletedIDs[testID] {
			return
		}
		tracker.CompletedIDs[testID] = true
		if info, exists := tracker.ActiveTests[testID]; exists {
			info.State = StateCompleted
			info.Elapsed = elapsed
		} else {
			tracker.ActiveTests[testID] = &TestInfo{
				ID:      testID,
				Name:    testID,
				State:   StateRunning,
				Started: time.Now(),
			}
		}
		// Only increment metrics if TestName is present (verified test function identifier)
		if testName != "" {
			entry := d.Ledger.GetOrCreateEntry(pipelineID)
			d.Ledger.UpdateEntry(pipelineID, entry.TotalRan+1, entry.TotalPassed, entry.TotalFailed+1)
		}
		if info, exists := tracker.ActiveTests[testID]; exists {
			info.State = StateCompleted
			tracker.Completed[testID] = info
			delete(tracker.ActiveTests, testID)
		}
		tracker.History = append(tracker.History, "✗ "+testID)
	}
}

func (d *Dashboard) GetMetrics(pipelineID string) (Ran int, Passed int, Failed int, ActiveTests map[string]*TestInfo, Completed map[string]*TestInfo) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tracker := d.TestTrackers[pipelineID]
	if tracker == nil {
		return 0, 0, 0, make(map[string]*TestInfo), make(map[string]*TestInfo)
	}
	active := make(map[string]*TestInfo, len(tracker.ActiveTests))
	for k, v := range tracker.ActiveTests {
		cp := *v
		active[k] = &cp
	}
	comp := make(map[string]*TestInfo, len(tracker.Completed))
	for k, v := range tracker.Completed {
		cp := *v
		comp[k] = &cp
	}
	entry := d.Ledger.GetEntry(pipelineID)
	return entry.TotalRan, entry.TotalPassed, entry.TotalFailed,
		active, comp
}

func (d *Dashboard) GetTotalFailures() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	total := 0
	for _, entry := range d.Ledger.entries {
		total += entry.TotalFailed
	}
	return total
}

func (d *Dashboard) SetPipelineActive(active bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.PipelineActive = active
}

func (d *Dashboard) GetActivePipelines() []PipelineConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Pipelines
}

func (d *Dashboard) GetErrorLogs() []ErrorLog {
	d.mu.RLock()
	defer d.mu.RUnlock()
	clone := make([]ErrorLog, len(d.ErrorLogs))
	copy(clone, d.ErrorLogs)
	return clone
}

func (d *Dashboard) RenderPanel(pipeline PipelineConfig) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.SkippedPipelines[pipeline.ID] {
		return fmt.Sprintf("%s%s %s[SKIPPED]%s\n", Bold, pipeline.Name, Yellow, Reset)
	}

	entry := d.Ledger.GetEntry(pipeline.ID)
	if entry == nil {
		return fmt.Sprintf("%s%s%s\n", Bold, pipeline.Name, Reset)
	}

	effectiveFloor := pipeline.LedgerFloor
	if effectiveFloor == 0 {
		effectiveFloor = entry.HistoricalFloor
	}
	floorBroken := effectiveFloor > 0 && entry.TotalRan > 0 && entry.TotalPassed < effectiveFloor
	metricsColor := Reset
	if floorBroken {
		metricsColor = Red
	}

	var header string
	if floorBroken {
		header = fmt.Sprintf(
			"%s%s  Ran: %s%d%s | Pass: %s%d%s | Fail: %s%d%s  %s%s(⚠️ BASELINE BROKEN: Expected %d, Got %d)%s\n",
			Bold,
			pipeline.Name,
			metricsColor, entry.TotalRan, Reset,
			metricsColor, entry.TotalPassed, Reset,
			metricsColor, entry.TotalFailed, Reset,
			Bold, Red,
			effectiveFloor, entry.TotalPassed,
			Reset,
		)
	} else {
		header = fmt.Sprintf(
			"%s%s  Ran: %s%d%s | Pass: %s%d%s | Fail: %s%d%s%s\n",
			Bold,
			pipeline.Name,
			metricsColor, entry.TotalRan, Reset,
			metricsColor, entry.TotalPassed, Reset,
			metricsColor, entry.TotalFailed, Reset,
			Reset,
		)
	}

	tracker := d.TestTrackers[pipeline.ID]
	list := "\n"
	if tracker != nil {
		displayHistory := tracker.History
		truncated := 0
		if len(displayHistory) > maxDisplayHistory {
			truncated = len(displayHistory) - maxDisplayHistory
			displayHistory = displayHistory[truncated:]
		}
		if truncated > 0 {
			list += fmt.Sprintf("   %s... and %d more%s\n", White, truncated, Reset)
		}
		for _, line := range displayHistory {
			color := Green
			if strings.HasPrefix(line, "✗") {
				color = Red
			}
			list += fmt.Sprintf("   %s%s%s\n", color, line, Reset)
		}
		if len(tracker.ActiveTests) > 0 {
			list += fmt.Sprintf("   %s⏳ Running:%s\n", Yellow, Reset)
			for _, info := range tracker.ActiveTests {
				elapsed := time.Since(info.Started).Seconds()
				list += fmt.Sprintf("   %s   %s (%.1fs)%s\n", Yellow, info.Name, elapsed, Reset)
			}
		}
	}

	return header + list
}

func (d *Dashboard) RenderSummary() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalRan, totalPassed, totalFailed int
	var totalFloor int
	if d.Ledger != nil {
		totalRan = d.Ledger.GetTotalRan()
		totalPassed = d.Ledger.GetTotalPassed()
		totalFailed = d.Ledger.GetTotalFailed()
		totalFloor = d.Ledger.GetTotalFloor()
	}

	allNonSkippedOK := true
	var brokenFloors []struct {
		id    string
		name  string
		floor int
		got   int
	}
	if d.Ledger != nil {
		for _, p := range d.Pipelines {
			skipped := d.SkippedPipelines[p.ID]
			e := d.Ledger.GetEntry(p.ID)
			if !skipped && (e == nil || e.TotalRan == 0 || e.TotalFailed > 0) {
				allNonSkippedOK = false
			}
			ef := p.LedgerFloor
			if ef == 0 && e != nil {
				ef = e.HistoricalFloor
			}
			if !skipped && e != nil && e.TotalRan > 0 && ef > 0 && e.TotalPassed < ef {
				brokenFloors = append(brokenFloors, struct {
					id    string
					name  string
					floor int
					got   int
				}{p.ID, p.Name, ef, e.TotalPassed})
			}
		}
	} else {
		allNonSkippedOK = false
	}

	anyFloorBroken := len(brokenFloors) > 0

	var statusLine string
	if totalRan == 0 {
		statusLine = fmt.Sprintf("%s❌ SYSTEM ERROR: No test execution streams were detected or processed.%s\n", Red, Reset)
	} else if anyFloorBroken {
		statusLine = fmt.Sprintf("%s❌ BASELINE REGRESSION: %d pipeline(s) below configured floor%s\n", Red, len(brokenFloors), Reset)
	} else if totalFailed > 0 {
		statusLine = fmt.Sprintf("%s❌ FAILURE: %d test(s) failed%s\n", Red, totalFailed, Reset)
	} else if totalFloor > 0 && totalPassed < totalFloor {
		statusLine = fmt.Sprintf("%s❌ REGRESSION: passed=%d below baseline=%d%s\n", Red, totalPassed, totalFloor, Reset)
	} else if !allNonSkippedOK {
		statusLine = fmt.Sprintf("%s❌ CRITICAL FAILURE: Some pipeline(s) did not execute any tests or failed entirely%s\n", Red, Reset)
	} else {
		statusLine = fmt.Sprintf("%s✅ ALL SYSTEMS NOMINAL: ALL TESTS PASSED CLEANLY%s\n", Green, Reset)
	}

	result := fmt.Sprintf("\n%s========================================\n", Bold)
	result += statusLine
	result += fmt.Sprintf("%sTotal Tests: %d\n", White, totalRan) +
		fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset) +
		fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset) +
		fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset) +
		fmt.Sprintf("%s========================================\n", Bold)

	for _, pipeline := range d.Pipelines {
		result += "\n" + d.RenderPanel(pipeline)
	}

	for _, errMsg := range d.SystemErrors {
		result += fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset)
	}

	if anyFloorBroken {
		prompt := fmt.Sprintf("\n%s%s🤖 === FORGEFIX AUTOMATED AGENT CONTEXT PROMPT ===%s\n", Bold, Yellow, Reset)
		prompt += fmt.Sprintf("%sCopy and paste this segment into your AI coding agent interface to initiate self-healing:%s\n", Yellow, Reset)
		prompt += "-----------------------------------------------------------------\n"
		prompt += "You are an expert autonomous software engineer patching a test coverage floor regression.\n"
		for _, bf := range brokenFloors {
			prompt += fmt.Sprintf("- Pipeline ID: '%s'\n", bf.id)
			prompt += fmt.Sprintf("  - Baseline Floor Required: %d passing tests\n", bf.floor)
			prompt += fmt.Sprintf("  - Active Run passing Count: %d passing tests\n", bf.got)
		}
		prompt += "Analyze the git diff or file history inside the workspace. Determine if tests were deleted, commented out, or muted to mask a failure. Restore or rewrite the missing tests immediately.\n"
		prompt += "-----------------------------------------------------------------\n"
		result += prompt
	}

	return result
}

// ============================================================================
// CONFIGURATION LOADING
// ============================================================================

type LoadedConfig struct {
	Config    *Config
	ConfigDir string
}

func LoadPipelineConfig() (*LoadedConfig, error) {
	candidates := []string{
		"forgefix.yaml",
		filepath.Join("..", "forgefix.yaml"),
	}
	if exeDir, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exeDir), "forgefix.yaml"))
	}

	for _, configPath := range candidates {
		if _, err := os.Stat(configPath); err == nil {
			return loadConfigFromPath(configPath)
		}
	}
	fmt.Println(Bold + Red + "\n⚠️  forgefix.yaml not found.\n" + Reset)
	return nil, fmt.Errorf("forgefix.yaml not found")
}

func loadConfigFromPath(path string) (*LoadedConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %v", err)
	}
	configDir := filepath.Dir(absPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	for i, p := range config.Pipelines {
		lang, ok := config.Languages[p.Type]
		if !ok {
			continue
		}
		config.Pipelines[i].Command.Type = p.Type
		config.Pipelines[i].TokenPatterns = lang.TokenPatterns
	}

	return &LoadedConfig{
		Config:    &config,
		ConfigDir: configDir,
	}, nil
}

// ============================================================================
// AI MODE STRUCTURED OUTPUT
// ============================================================================

type AITestEntry struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type AIPipelineResult struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Ran             int           `json:"ran"`
	Passed          int           `json:"passed"`
	Failed          int           `json:"failed"`
	Floor           int           `json:"floor"`
	Skipped         bool          `json:"skipped"`
	Status          string        `json:"status"`
	SuggestedAction string        `json:"suggested_agent_action"`
	ErrorDetails    string        `json:"error_details,omitempty"`
	SystemErrors    []string      `json:"system_errors,omitempty"`
	Tests           []AITestEntry `json:"tests,omitempty"`
}

type AIMetricsSummary struct {
	TotalRan    int `json:"total_ran"`
	TotalPassed int `json:"total_passed"`
	TotalFailed int `json:"total_failed"`
	TotalFloor  int `json:"total_floor"`
}

type AIResponsePayload struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Metrics   AIMetricsSummary  `json:"metrics"`
	Pipelines []AIPipelineResult `json:"pipelines"`
}

func (d *Dashboard) ToAIPayload() AIResponsePayload {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalRan, totalPassed, totalFailed, totalFloor int
	if d.Ledger != nil {
		totalRan = d.Ledger.GetTotalRan()
		totalPassed = d.Ledger.GetTotalPassed()
		totalFailed = d.Ledger.GetTotalFailed()
		totalFloor = d.Ledger.GetTotalFloor()
	}

	allSkipped := true
	for _, p := range d.Pipelines {
		if !d.SkippedPipelines[p.ID] {
			allSkipped = false
			break
		}
	}

	anyFloorBroken := false
	if d.Ledger != nil {
		for _, p := range d.Pipelines {
			if d.SkippedPipelines[p.ID] {
				continue
			}
			e := d.Ledger.GetEntry(p.ID)
		ef := p.LedgerFloor
		if e != nil && ef == 0 {
			ef = e.HistoricalFloor
		}
		if e != nil && e.TotalRan > 0 && ef > 0 && e.TotalPassed < ef {
				anyFloorBroken = true
				break
			}
		}
	}

	overallStatus := "pass"
	if totalRan == 0 {
		if allSkipped {
			overallStatus = "pass"
		} else {
			overallStatus = "error"
		}
	} else if anyFloorBroken {
		overallStatus = "regression"
	} else if totalFailed > 0 {
		overallStatus = "fail"
	} else if totalFloor > 0 && totalPassed < totalFloor {
		overallStatus = "regression"
	} else if d.TimeoutFired {
		overallStatus = "timeout"
	}

	var pipelines []AIPipelineResult
	for _, p := range d.Pipelines {
		skipped := d.SkippedPipelines[p.ID]
		entry := d.Ledger.GetEntry(p.ID)

		status := "pass"
		var suggestedAction, errorDetails string
		var systemErrors []string

		ef := p.LedgerFloor
		if ef == 0 && entry != nil {
			ef = entry.HistoricalFloor
		}
		floorBroken := !skipped && entry != nil && entry.TotalRan > 0 && ef > 0 && entry.TotalPassed < ef

		if skipped {
			status = "skipped"
			suggestedAction = "No action required. This pipeline type resource was not found in the project tree."
		} else if entry == nil {
			status = "error"
			suggestedAction = "SYSTEM STREAM DATA DROP: Pipeline was not skipped but no execution data was captured. Verify test runner is installed and the workspace configuration is correct."
			errorDetails = "No ledger entry created — test runner may have failed before producing any events."
		} else if entry.TotalRan == 0 {
			status = "error"
			suggestedAction = "SYSTEM STREAM DATA DROP: No tests were executed. Check if the test runner is correctly installed, the project compiles, and the test command is properly configured."
			errorDetails = "Zero test streams detected for a non-skipped pipeline."
		} else if floorBroken {
			status = "regression"
			suggestedAction = fmt.Sprintf("BASELINE FLOOR BROKEN: Pipeline '%s' requires %d passing tests but only %d passed. Restore or rewrite the missing tests.", p.ID, ef, entry.TotalPassed)
			errorDetails = fmt.Sprintf("passed=%d below floor=%d", entry.TotalPassed, ef)
		} else if entry.TotalFailed > 0 {
			status = "fail"
			suggestedAction = fmt.Sprintf("TEST FAILURE: %d test(s) failed. Review the failed test names below and inspect the corresponding source files for assertion errors.", entry.TotalFailed)
			errorDetails = fmt.Sprintf("%d of %d tests failed", entry.TotalFailed, entry.TotalRan)
		} else if d.TimeoutFired {
			status = "timeout"
			suggestedAction = "TIMEOUT: The pipeline execution exceeded the global timeout. Consider increasing the timeout value in forgefix.yaml or optimizing slow tests."
			errorDetails = fmt.Sprintf("%d tests passed before timeout", entry.TotalRan)
		} else if totalFloor > 0 && totalPassed < totalFloor {
			status = "regression"
			suggestedAction = fmt.Sprintf("REGRESSION: %d test(s) went missing from the baseline of %d. Review recent code changes for removed or disabled tests.", totalFloor-totalPassed, totalFloor)
			errorDetails = fmt.Sprintf("passed=%d below baseline=%d", totalPassed, totalFloor)
		} else {
			suggestedAction = "All tests passed. No action required."
		}

		var testList []AITestEntry
		if tracker := d.TestTrackers[p.ID]; tracker != nil {
			for _, h := range tracker.History {
				entryStatus := "pass"
				cleanID := h
				if strings.HasPrefix(h, "✗ ") {
					entryStatus = "fail"
					cleanID = h[4:]
				} else if strings.HasPrefix(h, "✓ ") {
					cleanID = h[4:]
				}
				testList = append(testList, AITestEntry{
					ID:     cleanID,
					Status: entryStatus,
				})
			}
		}

		sysErrors := d.SystemErrors
		if len(sysErrors) > 0 {
			systemErrors = sysErrors
		}

		ran, passed, failed := 0, 0, 0
		floor := 0
		if entry != nil {
			ran = entry.TotalRan
			passed = entry.TotalPassed
			failed = entry.TotalFailed
			floor = entry.HistoricalFloor
		}

		pipelines = append(pipelines, AIPipelineResult{
			ID:              p.ID,
			Name:            p.Name,
			Ran:             ran,
			Passed:          passed,
			Failed:          failed,
			Floor:           floor,
			Skipped:         skipped,
			Status:          status,
			SuggestedAction: suggestedAction,
			ErrorDetails:    errorDetails,
			SystemErrors:    systemErrors,
			Tests:           testList,
		})
	}

	return AIResponsePayload{
		Status:  overallStatus,
		Version: "forgefix/v1",
		Metrics: AIMetricsSummary{
			TotalRan:    totalRan,
			TotalPassed: totalPassed,
			TotalFailed: totalFailed,
			TotalFloor:  totalFloor,
		},
		Pipelines: pipelines,
	}
}

var SafeColorAllocator = []string{Red, Green, White}

func getSafeColor(index int) string {
	return SafeColorAllocator[index%len(SafeColorAllocator)]
}
