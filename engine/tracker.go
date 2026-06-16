package engine

import (
	"sync"
)

// ============================================================================
// TEST STATE TRACKER
// ============================================================================

type Tracker struct {
	mu      sync.RWMutex
	tests   map[string]*TestInfo
	results []TestResult
}

func NewTracker() *Tracker {
	return &Tracker{
		tests:   make(map[string]*TestInfo),
		results: make([]TestResult, 0),
	}
}

func (t *Tracker) Start(id, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.tests[id]; !exists {
		t.tests[id] = &TestInfo{
			ID:   id,
			Name: name,
		}
	}
}

func (t *Tracker) Complete(id string, elapsed int, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if info, exists := t.tests[id]; exists {
		info.State = StateCompleted
		info.Elapsed = elapsed
		t.results = append(t.results, TestResult{
			Name:    info.Name,
			State:   StateCompleted,
			Elapsed: elapsed,
			Success: success,
		})
	}
}

func (t *Tracker) GetResults() []TestResult {
	t.mu.RLock()
	defer t.mu.RUnlock()
	clone := make([]TestResult, len(t.results))
	copy(clone, t.results)
	return clone
}
