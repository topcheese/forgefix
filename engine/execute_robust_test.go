package engine

import (
	"strings"
	"testing"
)

func TestTUIPanelHistoryTruncated(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("test-pipe")
	n := 50
	for i := range n {
		id := "TestHistoryTruncation/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.Ledger.GetEntry("test-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	panel := d.RenderPanel(d.Pipelines[0])

	lines := strings.Split(strings.TrimSpace(panel), "\n")
	if len(lines) < 2 {
		t.Fatalf("panel too short: got %d lines", len(lines))
	}

	if !strings.Contains(panel, "... and 35 more") {
		t.Errorf("expected truncation message '... and 35 more' in panel output")
	}

	lineCount := len(lines)
	maxExpected := maxDisplayHistory + 5
	if lineCount > maxExpected {
		t.Errorf("panel too long: %d lines, expected at most %d", lineCount, maxExpected)
	}

	lastHistoryLine := lines[len(lines)-1]
	if !strings.Contains(lastHistoryLine, "TestFunc_49") {
		t.Errorf("expected last history entry TestFunc_49, got: %s", lastHistoryLine)
	}
}

func TestTUIPanelHistoryAllShown(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("test-pipe")
	n := 10
	for i := range n {
		id := "TestAllShown/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.Ledger.GetEntry("test-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	panel := d.RenderPanel(d.Pipelines[0])

	if strings.Contains(panel, "... and") {
		t.Errorf("unexpected truncation message for %d entries", n)
	}

	lines := strings.Split(strings.TrimSpace(panel), "\n")
	if len(lines) > n+3 {
		t.Errorf("panel too long for %d entries: got %d lines", n, len(lines))
	}
}

func TestTUIRenderMaxHistoryConstant(t *testing.T) {
	if maxDisplayHistory <= 0 {
		t.Fatal("maxDisplayHistory must be positive")
	}
	if maxDisplayHistory > 50 {
		t.Logf("WARNING: maxDisplayHistory=%d is high, TUI may still scroll", maxDisplayHistory)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
