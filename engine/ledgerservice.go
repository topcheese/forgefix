package engine

import (
	"fmt"
	"sync"
	"time"
)

// LedgerService provides aggregated metrics across pipelines.
// Single Responsibility: Aggregate and format ledger metrics for display.
type LedgerService struct {
	mu      sync.RWMutex
	entries map[string]*LedgerEntry
}

func NewLedgerService() *LedgerService {
	return &LedgerService{
		entries: make(map[string]*LedgerEntry),
	}
}

func (s *LedgerService) GetOrCreateEntry(pipelineID string) *LedgerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[pipelineID]; !exists {
		s.entries[pipelineID] = &LedgerEntry{
			PipelineID:      pipelineID,
			TotalRan:        0,
			TotalPassed:     0,
			TotalFailed:     0,
			HistoricalFloor: 0,
			LastUpdate:      time.Now().Format(time.RFC3339),
		}
	}
	return s.entries[pipelineID]
}

func (s *LedgerService) UpdateEntry(pipelineID string, ran, passed, failed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[pipelineID]
	if !exists {
		entry = &LedgerEntry{PipelineID: pipelineID}
		s.entries[pipelineID] = entry
	}
	entry.TotalRan = ran
	entry.TotalPassed = passed
	entry.TotalFailed = failed
	entry.LastUpdate = time.Now().Format(time.RFC3339)
	if passed > entry.HistoricalFloor {
		entry.HistoricalFloor = passed
	}
}

func (s *LedgerService) GetEntry(pipelineID string) *LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries[pipelineID]
}

func (s *LedgerService) ResetCurrentRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.entries {
		entry.TotalRan = 0
		entry.TotalPassed = 0
		entry.TotalFailed = 0
	}
}

func (s *LedgerService) GetTotalRan() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, entry := range s.entries {
		total += entry.TotalRan
	}
	return total
}

func (s *LedgerService) GetTotalPassed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, entry := range s.entries {
		total += entry.TotalPassed
	}
	return total
}

func (s *LedgerService) GetTotalFailed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, entry := range s.entries {
		total += entry.TotalFailed
	}
	return total
}

func (s *LedgerService) GetTotalFloor() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, entry := range s.entries {
		total += entry.HistoricalFloor
	}
	return total
}

func (s *LedgerService) FormatSummary(boldOpt, whiteOpt, greenOpt, redOpt, resetOpt string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ran, passed, failed, floor int
	for _, entry := range s.entries {
		ran += entry.TotalRan
		passed += entry.TotalPassed
		failed += entry.TotalFailed
		floor += entry.HistoricalFloor
	}

	return fmt.Sprintf("\n%s========================================\n%sTotal Tests: %d\nPassed: %s%d%s\nFailed: %s%d%s\nBaseline: %d\n========================================\n",
		boldOpt, whiteOpt, ran, greenOpt, passed, resetOpt, redOpt, failed, resetOpt, floor)
}

func (s *LedgerService) GetAllEntries() map[string]*LedgerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]*LedgerEntry, len(s.entries))
	for k, v := range s.entries {
		cp[k] = v
	}
	return cp
}