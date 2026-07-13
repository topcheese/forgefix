package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// LEDGER FILE MANAGER
// ============================================================================

// loadLedgerFromJSONFile reads the JSON ledger file directly without opening a
// DB connection. Used only by ImportLedger for the one-time migration to SQLite.
// Returns (nil, nil) if no JSON file exists.
func loadLedgerFromJSONFile(configDir string) (*LedgerEngine, error) {
	if err := MigrateToFF(configDir); err != nil {
		return nil, fmt.Errorf("migrating to .ff/: %w", err)
	}
	path := FFLedgerPath(configDir)
	_, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading JSON ledger: %w", err)
	}
	var wrapper struct {
		Version      string                  `json:"version,omitempty"`
		Entries      map[string]*LedgerEntry `json:"entries"`
		SpecMappings map[string]*SpecEntry   `json:"spec_mappings"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("invalid ledger JSON: %w", err)
	}
	le := NewLedgerEngine()
	if wrapper.Version != "" {
		le.Version = wrapper.Version
	}
	if wrapper.Entries != nil {
		le.entries = wrapper.Entries
	}
	if wrapper.SpecMappings != nil {
		le.specMappings = wrapper.SpecMappings
	}
	return le, nil
}

func LoadLedger(configDir string) (*LedgerEngine, error) {
	if err := MigrateToFF(configDir); err != nil {
		return nil, fmt.Errorf("migrating to .ff/: %w", err)
	}

	ledger := NewLedgerEngine()
	ledger.WorkflowConfig = LoadWorkflowConfig(configDir)

	// Try loading from SQLite
	db, dbErr := OpenDB(configDir)
	if dbErr == nil {
		defer db.Close()

		// Read project version from meta
		version, _ := db.ProjectVersion()
		if version != "" {
			ledger.Version = version
		}

		// Read pipeline stats
		stats, err := db.GetAllPipelineStats()
		if err == nil {
			for _, s := range stats {
				ledger.entries[s.PipelineID] = &LedgerEntry{
					PipelineID:      s.PipelineID,
					TotalRan:        s.TotalRan,
					TotalPassed:     s.TotalPassed,
					TotalFailed:     s.TotalFailed,
					HistoricalFloor: s.HistoricalFloor,
					LastUpdate:      s.LastUpdate,
				}
			}
		}

		// Read specs from DB — query all non-archived rows with their linked commits.
		// Archived specs live only in the DB for query/reference; they are not
		// loaded into the in-memory LedgerEngine, which mirrors the old JSON
		// file behavior where archived entries were removed from the JSON file.
		rows, err := db.Conn().Query("SELECT spec_id, title, status, type, repo_issue_id FROM specs WHERE status != 'archived'")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var specID, title, st, specType string
				var repoIssueID int
				if err := rows.Scan(&specID, &title, &st, &specType, &repoIssueID); err != nil {
					continue
				}
				linkedCommits, _ := db.GetLinkedCommits(specID)
				ledger.specMappings[specID] = &SpecEntry{
					SpecID:        title,
					RepoIssueID:   repoIssueID,
					Status:        st,
					LinkedCommits: linkedCommits,
					Type:          specType,
				}
			}
		}
	} else {
		// DB not available — fall back to JSON file for legacy migration
		jsonLedger, jErr := loadLedgerFromJSONFile(configDir)
		if jErr == nil && jsonLedger != nil {
			ledger = jsonLedger
			ledger.WorkflowConfig = LoadWorkflowConfig(configDir)
		}
	}

	// Always sync from filesystem to catch newly-created specs not yet in DB
	// and to reconcile status changes made by editing spec files directly.
	if err := ledger.SyncFromSpecsDir(configDir); err != nil {
		return nil, fmt.Errorf("syncing ledger from specs dir: %w", err)
	}
	return ledger, nil
}

func SaveLedger(ledger *LedgerEngine, configDir string) error {
	// Write to SQLite
	db, err := OpenDB(configDir)
	if err != nil {
		// Fallback to JSON if DB is unavailable
		return ledger.SaveToFile(FFLedgerPath(configDir))
	}
	defer db.Close()

	tx, err := db.Conn().Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete all non-archived records before re-inserting the current state.
	// This mirrors the old JSON behavior where SaveLedger overwrote the entire file.
	if _, err := tx.Exec("DELETE FROM pipeline_stats"); err != nil {
		return fmt.Errorf("clearing pipeline stats: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM linked_commits WHERE spec_id IN (SELECT spec_id FROM specs WHERE status != 'archived')"); err != nil {
		return fmt.Errorf("clearing linked commits: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM specs WHERE status != 'archived'"); err != nil {
		return fmt.Errorf("clearing specs: %w", err)
	}

	// Persist project version
	if ledger.Version != "" && ledger.Version != "0.0.0" {
		if _, err := tx.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('project_version', ?)", ledger.Version); err != nil {
			return fmt.Errorf("saving project version: %w", err)
		}
	}

	// Persist pipeline stats
	for id, entry := range ledger.GetAllEntries() {
		if _, err := tx.Exec(`
			INSERT INTO pipeline_stats (pipeline_id, total_ran, total_passed, total_failed, historical_floor, last_update)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`, id, entry.TotalRan, entry.TotalPassed, entry.TotalFailed, entry.HistoricalFloor); err != nil {
			return fmt.Errorf("saving pipeline stats for %s: %w", id, err)
		}
	}

	// Persist spec mappings
	for specID, entry := range ledger.GetAllSpecEntries() {
		if _, err := tx.Exec(`
			INSERT INTO specs (spec_id, title, status, type, version, repo_issue_id, root_cause, resolution, body, updated_at)
			VALUES (?, ?, ?, ?, '', ?, '', '', '', datetime('now'))
		`, specID, entry.SpecID, entry.Status, entry.Type, entry.RepoIssueID); err != nil {
			return fmt.Errorf("saving spec %s: %w", specID, err)
		}
		for _, hash := range entry.LinkedCommits {
			if _, err := tx.Exec(
				"INSERT OR IGNORE INTO linked_commits (spec_id, commit_hash) VALUES (?, ?)",
				specID, hash,
			); err != nil {
				return fmt.Errorf("saving linked commit %s for %s: %w", hash, specID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
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

type SpecEntry struct {
	SpecID        string   `json:"spec_id"`        // Human-readable title from spec file heading (map key is the real spec ID)
	RepoIssueID   int      `json:"repo_issue_id"`
	Status        string   `json:"status"`
	LinkedCommits []string `json:"linked_commits"`
	Type          string   `json:"type,omitempty"`
}

type LedgerEngine struct {
	mu             sync.RWMutex
	entries        map[string]*LedgerEntry
	specMappings   map[string]*SpecEntry
	Version        string
	WorkflowConfig *WorkflowConfig
}

func NewLedgerEngine() *LedgerEngine {
	return &LedgerEngine{
		entries:      make(map[string]*LedgerEntry),
		specMappings: make(map[string]*SpecEntry),
		Version:      "0.0.0",
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
	entry, exists := le.entries[pipelineID]
	if !exists {
		entry = &LedgerEntry{PipelineID: pipelineID}
		le.entries[pipelineID] = entry
	}
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
	le.mu.Lock()
	if le.Version == "" {
		le.Version = "0.0.0"
	}
	if le.entries == nil {
		le.entries = make(map[string]*LedgerEntry)
	}
	if le.specMappings == nil {
		le.specMappings = make(map[string]*SpecEntry)
	}
	data, err := json.MarshalIndent(struct {
		Entries      map[string]*LedgerEntry `json:"entries"`
		SpecMappings map[string]*SpecEntry   `json:"spec_mappings"`
	}{
		Entries:      le.entries,
		SpecMappings: le.specMappings,
	}, "", "  ")
	le.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (le *LedgerEngine) LoadFromJSON(data []byte) error {
	var wrapper struct {
		Version      string                  `json:"version,omitempty"`
		Entries      map[string]*LedgerEntry `json:"entries"`
		SpecMappings map[string]*SpecEntry   `json:"spec_mappings"`
	}

	// Strictly attempt to unmarshal into the wrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return fmt.Errorf("invalid ledger format: %w", err)
	}

	le.mu.Lock()
	defer le.mu.Unlock()

	le.Version = wrapper.Version

	// Initialize with data or empty maps
	if wrapper.Entries != nil {
		le.entries = wrapper.Entries
	} else {
		le.entries = make(map[string]*LedgerEntry)
	}

	if wrapper.SpecMappings != nil {
		le.specMappings = wrapper.SpecMappings
	} else {
		le.specMappings = make(map[string]*SpecEntry)
	}

	return nil
}

func (le *LedgerEngine) ResetCurrentRun() {
	le.mu.Lock()
	defer le.mu.Unlock()
	for _, entry := range le.entries {
		entry.TotalRan = 0
		entry.TotalPassed = 0
		entry.TotalFailed = 0
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

// FormatSummary aggregates and formats metrics in a single read lock pass
func (le *LedgerEngine) GetAllEntries() map[string]*LedgerEntry {
	le.mu.RLock()
	defer le.mu.RUnlock()
	cp := make(map[string]*LedgerEntry, len(le.entries))
	for k, v := range le.entries {
		cp[k] = v
	}
	return cp
}

func (le *LedgerEngine) FormatSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt string) string {
	le.mu.RLock()
	defer le.mu.RUnlock()

	var ran, passed, failed, floor int
	for _, entry := range le.entries {
		ran += entry.TotalRan
		passed += entry.TotalPassed
		failed += entry.TotalFailed
		floor += entry.HistoricalFloor
	}

	return fmt.Sprintf("\n%s========================================\n%sTotal Tests: %d\nPassed: %s%d%s\nFailed: %s%d%s\nBaseline: %d\n========================================\n",
		boldOpt, whiteOpt, ran, greenOpt, passed, resetOpt, redOpt, failed, resetOpt, floor)
}

func (le *LedgerEngine) GetSpecEntry(specID string) *SpecEntry {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.specMappings[specID]
}

func (le *LedgerEngine) SetSpecEntry(specID string, entry *SpecEntry) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.specMappings[specID] = entry
}

func (le *LedgerEngine) GetAllSpecEntries() map[string]*SpecEntry {
	le.mu.RLock()
	defer le.mu.RUnlock()
	cp := make(map[string]*SpecEntry, len(le.specMappings))
	for k, v := range le.specMappings {
		cp[k] = v
	}
	return cp
}

func (le *LedgerEngine) DeleteSpecEntry(specID string) {
	le.mu.Lock()
	defer le.mu.Unlock()
	delete(le.specMappings, specID)
}

func (le *LedgerEngine) DeleteSpec(specID string, configDir string) (int, error) {
	le.mu.Lock()
	entry, exists := le.specMappings[specID]
	if !exists {
		le.mu.Unlock()
		return 0, fmt.Errorf("spec %s not found in ledger", specID)
	}
	repoIssueID := entry.RepoIssueID
	delete(le.specMappings, specID)
	le.mu.Unlock()

	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			filePath := filepath.Join(specDir, e.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			content := string(data)
			if strings.Contains(content, fmt.Sprintf(`spec_id: "%s"`, specID)) {
				if err := os.Remove(filePath); err != nil {
					return repoIssueID, fmt.Errorf("removing spec file: %w", err)
				}
				break
			}
		}
	}

	if err := SaveLedger(le, configDir); err != nil {
		return repoIssueID, fmt.Errorf("saving ledger: %w", err)
	}

	return repoIssueID, nil
}

func (le *LedgerEngine) DeleteSpecWithArchive(specID string, configDir string, coord *IssueCoordinator) (int, error) {
	le.mu.Lock()
	entry, exists := le.specMappings[specID]
	if !exists {
		le.mu.Unlock()
		return 0, fmt.Errorf("spec %s not found in ledger", specID)
	}
	repoIssueID := entry.RepoIssueID
	if repoIssueID <= 0 {
		le.mu.Unlock()
		return 0, fmt.Errorf("spec %s has no associated repo issue", specID)
	}
	le.mu.Unlock()

	if coord == nil {
		return 0, fmt.Errorf("coordinator required for archive-first deletion")
	}

	remoteIssue, err := coord.GetIssueByNumber(repoIssueID)
	if err != nil {
		return repoIssueID, fmt.Errorf("fetching remote issue #%d: %w", repoIssueID, err)
	}
	if remoteIssue == nil {
		return repoIssueID, fmt.Errorf("remote issue #%d not found", repoIssueID)
	}
	if remoteIssue.State != "closed" {
		return repoIssueID, fmt.Errorf("remote issue #%d is not closed (state: %s), archive-first aborted", repoIssueID, remoteIssue.State)
	}

	return le.DeleteSpec(specID, configDir)
}

func (le *LedgerEngine) GetSpecEntryByRepoIssue(repoIssueID int) *SpecEntry {
	le.mu.RLock()
	defer le.mu.RUnlock()
	for _, entry := range le.specMappings {
		if entry.RepoIssueID == repoIssueID {
			return entry
		}
	}
	return nil
}

func (le *LedgerEngine) ListSpecs(all bool) ([]*SpecEntry, error) {
	specs := le.GetAllSpecEntries()

	var filteredSpecs []*SpecEntry
	for _, spec := range specs {
		if all {
			filteredSpecs = append(filteredSpecs, spec)
		} else if le.WorkflowConfig != nil {
			if le.WorkflowConfig.IsActiveStatus(spec.Status) {
				filteredSpecs = append(filteredSpecs, spec)
			}
		} else if spec.Status != "closed" {
			filteredSpecs = append(filteredSpecs, spec)
		}
	}

	sort.Slice(filteredSpecs, func(i, j int) bool {
		return filteredSpecs[i].SpecID < filteredSpecs[j].SpecID
	})

	return filteredSpecs, nil
}

func (le *LedgerEngine) SyncFromSpecsDir(configDir string) error {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	le.mu.Lock()
	defer le.mu.Unlock()

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
		frontmatter := parts[1]
		body := strings.TrimSpace(parts[2])
		var specID, status, specType, title string
		lines := strings.Split(frontmatter, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "spec_id:") {
				specID = strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
				specID = strings.Trim(specID, `"`)
			} else if strings.HasPrefix(line, "status:") {
				status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
				status = strings.Split(status, " ")[0]
			} else if strings.HasPrefix(line, "type:") {
				specType = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
				specType = strings.Trim(specType, `"`)
			}
		}
		// Extract title from markdown heading
		if strings.HasPrefix(body, "# ") {
			titleLine := strings.SplitN(body, "\n", 2)[0]
			title = strings.TrimPrefix(titleLine, "# ")
		}
		if specID == "" || status == "" {
			continue
		}

		if existing, ok := le.specMappings[specID]; ok {
			existing.Status = status
			if title != "" {
				existing.SpecID = title
			}
			if specType != "" {
				existing.Type = specType
			}
		} else {
			titleVal := title
			if titleVal == "" {
				titleVal = specID
			}
			le.specMappings[specID] = &SpecEntry{
				SpecID:        titleVal,
				RepoIssueID:   0,
				Status:        status,
				LinkedCommits: []string{},
				Type:          specType,
			}
		}
	}
	return nil
}
