package engine

import (
	"sync"
	"time"
)

// TestTrackerService manages test execution state for pipelines.
// Single Responsibility: Track test state transitions (run/pass/fail) and history.
type TestTrackerService struct {
	mu          sync.RWMutex
	trackers    map[string]*TestTracker
	dirtySignal func()
}

func NewTestTrackerService(dirtySignal func()) *TestTrackerService {
	return &TestTrackerService{
		trackers:    make(map[string]*TestTracker),
		dirtySignal: dirtySignal,
	}
}

func (s *TestTrackerService) GetTracker(pipelineID string) *TestTracker {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.trackers[pipelineID]; !exists {
		s.trackers[pipelineID] = &TestTracker{
			ActiveTests:  make(map[string]*TestInfo),
			Completed:    make(map[string]*TestInfo),
			CompletedIDs: make(map[string]bool),
			History:      make([]string, 0),
		}
	}
	return s.trackers[pipelineID]
}

func (s *TestTrackerService) ResetTrackers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markDirty()
	for _, tracker := range s.trackers {
		tracker.ActiveTests = make(map[string]*TestInfo)
		tracker.Completed = make(map[string]*TestInfo)
		tracker.CompletedIDs = make(map[string]bool)
		tracker.History = make([]string, 0)
		tracker.Ran = 0
		tracker.Passed = 0
	}
}

func (s *TestTrackerService) UpdateMetrics(pipelineID, action, testID, testName string, elapsed int, result string, errorTrace, filePath string, failureLine, failureColumn int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markDirty()

	tracker := s.getOrCreateTrackerLocked(pipelineID)

	switch action {
	case "run":
		if _, exists := tracker.ActiveTests[testID]; !exists {
			tracker.ActiveTests[testID] = &TestInfo{
				ID:      testID,
				Name:    testName,
				TestID:  testID,
				TestName: testName,
				State:   StateFiring,
				Started: time.Now(),
			}
		} else {
			tracker.ActiveTests[testID].State = StateFiring
		}
	case "pass":
		if tracker.CompletedIDs[testID] {
			return
		}
		tracker.CompletedIDs[testID] = true
		var info *TestInfo
		if existing, exists := tracker.ActiveTests[testID]; exists {
			info = existing
			info.State = StatePopped
			info.Elapsed = elapsed
		} else {
			info = &TestInfo{
				ID:       testID,
				Name:     testName,
				TestID:   testID,
				TestName: testName,
				State:    StatePopped,
				Started:  time.Now(),
				Elapsed:  elapsed,
			}
		}
		if info != nil {
			tracker.Completed[testID] = info
			delete(tracker.ActiveTests, testID)
		}
		tracker.History = append(tracker.History, "✓ "+testID)
		tracker.Passed++
		tracker.Ran++
	case "fail":
		if tracker.CompletedIDs[testID] {
			return
		}
		tracker.CompletedIDs[testID] = true
		var info *TestInfo
		if existing, exists := tracker.ActiveTests[testID]; exists {
			info = existing
			info.State = StateDud
			info.Elapsed = elapsed
			info.ErrorTrace = errorTrace
			info.FilePath = filePath
			info.FailureLine = failureLine
			info.FailureColumn = failureColumn
		} else {
			info = &TestInfo{
				ID:            testID,
				Name:          testName,
				TestID:        testID,
				TestName:      testName,
				State:         StateDud,
				Started:       time.Now(),
				Elapsed:       elapsed,
				ErrorTrace:    errorTrace,
				FilePath:      filePath,
				FailureLine:   failureLine,
				FailureColumn: failureColumn,
			}
		}
		if info != nil {
			tracker.Completed[testID] = info
			delete(tracker.ActiveTests, testID)
		}
		tracker.History = append(tracker.History, "✗ "+testID)
		tracker.Ran++
	}
}

func (s *TestTrackerService) GetMetrics(pipelineID string) (ran, passed, failed int, active, completed map[string]*TestInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tracker := s.trackers[pipelineID]
	if tracker == nil {
		return 0, 0, 0, make(map[string]*TestInfo), make(map[string]*TestInfo)
	}
	active = make(map[string]*TestInfo, len(tracker.ActiveTests))
	for k, v := range tracker.ActiveTests {
		cp := *v
		active[k] = &cp
	}
	completed = make(map[string]*TestInfo, len(tracker.Completed))
	for k, v := range tracker.Completed {
		cp := *v
		completed[k] = &cp
	}
	return tracker.Ran, tracker.Passed, tracker.Ran - tracker.Passed, active, completed
}

func (s *TestTrackerService) getOrCreateTrackerLocked(pipelineID string) *TestTracker {
	if _, exists := s.trackers[pipelineID]; !exists {
		s.trackers[pipelineID] = &TestTracker{
			ActiveTests:  make(map[string]*TestInfo),
			Completed:    make(map[string]*TestInfo),
			CompletedIDs: make(map[string]bool),
			History:      make([]string, 0),
		}
	}
	return s.trackers[pipelineID]
}

func (s *TestTrackerService) GetAllTrackers() map[string]*TestTracker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]*TestTracker, len(s.trackers))
	for k, v := range s.trackers {
		cp[k] = v
	}
	return cp
}

func (s *TestTrackerService) markDirty() {
	if s.dirtySignal != nil {
		s.dirtySignal()
	}
}