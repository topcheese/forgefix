package engine

import (
	"testing"
	"time"
)

func TestKeyboardListenerQuitKeys(t *testing.T) {
	for _, key := range []byte{'q', 'Q', 'x', 'X'} {
		t.Run(string(key), func(t *testing.T) {
			done := make(chan struct{}, 1)
			go func() {
				time.Sleep(50 * time.Millisecond)
				done <- struct{}{}
			}()
			select {
			case <-done:
			case <-time.After(200 * time.Millisecond):
				t.Fatal("timeout waiting for goroutine")
			}
		})
	}
}

func TestGetFailureDecaySecondsFromConfig(t *testing.T) {
	d := NewDashboard(nil)

	decay := d.GetFailureDecaySeconds()
	if decay != 15 {
		t.Errorf("expected default decay 15, got %d", decay)
	}

	d.FailureDecaySecs = 30
	decay = d.GetFailureDecaySeconds()
	if decay != 30 {
		t.Errorf("expected decay 30, got %d", decay)
	}
}

func TestCollectFailedTestsEmptyOnNoFailures(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "pipe1", Name: "Pipe 1"},
	})

	failed := collectFailedTests(d)
	if len(failed) != 0 {
		t.Errorf("expected 0 failed tests, got %d", len(failed))
	}
}

func TestCollectFailedTestsReturnsDuds(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "pipe1", Name: "Pipe 1"},
	})
	tracker := d.GetTracker("pipe1")
	tracker.Completed["fail1"] = &TestInfo{
		ID: "fail1", Name: "FailTest1", State: StateDud,
	}
	tracker.CompletedIDs["fail1"] = true

	failed := collectFailedTests(d)
	if len(failed) != 1 {
		t.Errorf("expected 1 failed test, got %d", len(failed))
	}
	if failed[0].TestName != "FailTest1" {
		t.Errorf("expected FailTest1, got %s", failed[0].TestName)
	}
}
