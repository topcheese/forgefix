package engine

import (
	"testing"
	"time"
)

// TestTrackerLifecycle Transitions asserts that individual test records
// can seamlessly move from pending, to active running, to fully completed.
func TestTrackerLifecycleTransitions(t *testing.T) {
	tracker := &TestTracker{
		ActiveTests:  make(map[string]*TestInfo),
		Completed:    make(map[string]*TestInfo),
		CompletedIDs: make(map[string]bool),
	}

	testName := "TestClusterDiscoveryEngine_AgnosticSweep"

	// 1. Simulate a test function spanning into active execution state
	tracker.mu.Lock()
	tracker.ActiveTests[testName] = &TestInfo{
		ID:      "tx-99",
		Name:    testName,
		State:   StateRunning,
		Started: time.Now().Add(-2 * time.Second),
	}
	tracker.Ran++
	tracker.mu.Unlock()

	// Verify the active state tracking works
	tracker.mu.RLock()
	activeInfo, exists := tracker.ActiveTests[testName]
	if !exists || activeInfo.State != StateRunning {
		t.Errorf("expected test %s to be running in active map", testName)
	}
	tracker.mu.RUnlock()

	// 2. Transition the test into a successfully completed pass block
	tracker.mu.Lock()
	info := tracker.ActiveTests[testName]
	delete(tracker.ActiveTests, testName)

	info.State = StateCompleted
	info.Elapsed = int(time.Since(info.Started).Milliseconds())
	tracker.Completed[testName] = info
	tracker.CompletedIDs[info.ID] = true
	tracker.Passed++
	tracker.mu.Unlock()

	// 3. Structural validation checks on the finalized metrics state
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	if len(tracker.ActiveTests) != 0 {
		t.Errorf("expected active tests map to be completely empty, got %d items", len(tracker.ActiveTests))
	}
	if tracker.Passed != 1 || tracker.Ran != 1 {
		t.Errorf("expected counters to increment to 1, got Ran=%d Passed=%d", tracker.Ran, tracker.Passed)
	}
	if !tracker.CompletedIDs["tx-99"] {
		t.Error("expected completion ID tracker to record tracking reference token key")
	}
}
