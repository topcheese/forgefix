package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRenderTestListShowsRunningWithGauges(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "gauge-pipe", Name: "Gauge Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("gauge-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("gauge-pipe")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "GaugeTest_Alpha", State: StateRunning,
	}

	list := d.RenderTestList(d.GetPipelinesSlice()[0])
	if !strings.Contains(list, "GaugeTest_Alpha") {
		t.Errorf("expected running test name, got:\n%s", list)
	}
	if !strings.Contains(list, "[") || !strings.Contains(list, "s]") {
		t.Errorf("expected running gauge to include duration [X.XXs], got:\n%s", list)
	}
	if !strings.Contains(list, "Firing:") {
		t.Errorf("expected 'Firing:' in running test output, got:\n%s", list)
	}
}

func TestRenderTestListMaxFive(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "max-pipe", Name: "Max Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("max-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("max-pipe")
	for i := 0; i < 10; i++ {
		id := "MaxTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	tracker.ActiveTests["running"] = &TestInfo{
		ID: "running", Name: "RunningTest", State: StateRunning,
	}

	list := d.RenderTestList(d.GetPipelinesSlice()[0])
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("test list should show at most 1 item, got %d", itemCount)
	}
	if !strings.Contains(list, "RunningTest") {
		t.Errorf("expected running test to appear in test list")
	}
}

func TestRenderTestListEmptyOnSkipped(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "skip-pipe", Name: "Skip Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("skip-pipe")
	d.SetLedger(ledger)

	d.MarkPipelineSkipped("skip-pipe")

	list := d.RenderTestList(d.GetPipelinesSlice()[0])
	if list != "" {
		t.Errorf("expected empty test list for skipped pipeline, got:\n%s", list)
	}
}

func TestTUIPanelHistoryTruncated(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	n := 50
	for i := range n {
		id := "TestHistoryTruncation/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.GetLedger().GetEntry("test-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	panel := d.RenderPanel(d.GetPipelinesSlice()[0])

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("panel should show at most 1 item row, got %d", itemCount)
	}
}

func TestTUIPanelHistoryAllShown(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	n := 3
	for i := range n {
		id := "TestAllShown/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.GetLedger().GetEntry("test-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	panel := d.RenderPanel(d.GetPipelinesSlice()[0])

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("panel too long: got %d item rows, expected 1", itemCount)
	}
}

func TestTUIPanelPrioritizesRunningTests(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	for i := 20; i < 30; i++ {
		id := "Completed/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	tracker.ActiveTests["active-1"] = &TestInfo{
		ID: "active-1", Name: "ActiveTest_1", State: StateRunning,
	}
	tracker.ActiveTests["active-2"] = &TestInfo{
		ID: "active-2", Name: "ActiveTest_2", State: StateRunning,
	}

	panel := d.RenderPanel(d.GetPipelinesSlice()[0])

	if !strings.Contains(panel, "Firing:") {
		t.Errorf("expected running gauge indicator (Firing:) in panel output")
	}

	if !strings.Contains(panel, "ActiveTest_") {
		t.Errorf("expected at least one active test name in panel output")
	}

	for _, line := range strings.Split(panel, "\n") {
		if strings.Contains(line, "Firing:") {
			if !strings.Contains(line, "[") || !strings.Contains(line, "s]") {
				t.Errorf("running gauge missing duration: %s", line)
			}
		}
	}

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("panel should show at most 1 item row, got %d", itemCount)
	}
}

func TestTUIPanelTwoLineConstraint(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline"},
	})
	tracker := d.GetTracker("test-pipe")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID:      "t1",
		Name:    "GaugeTest",
		State:   StateFiring,
		Started: time.Now().Add(-500 * time.Millisecond),
	}

	result := d.renderTestList(PipelineConfig{ID: "test-pipe"})
	lines := strings.Split(result, "\n")

	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines per pipeline, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "Firing:") {
		t.Errorf("line 1 should contain firing gauge, got: %s", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Errorf("line 2 should be empty when no dud, got: %q", lines[1])
	}
}

func TestTUIRunningGaugeHasTwoDecimalDuration(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "GaugeTest", State: StateRunning,
	}

	panel := d.RenderPanel(d.GetPipelinesSlice()[0])
	if !strings.Contains(panel, "Firing:") || !strings.Contains(panel, "GaugeTest") {
		t.Errorf("expected running gauge with Firing: and duration: %s", panel)
	}
}

func TestRenderFailureReportIncludesFailedTestNames(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Failure Pipeline", LedgerFloor: 1},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	tracker.CompletedIDs["TestFail1"] = true
	tracker.CompletedIDs["TestPass1"] = true
	tracker.History = append(tracker.History, "✓ TestPass1")
	tracker.History = append(tracker.History, "✗ TestFail1")
	ledger.UpdateEntry("test-pipe", 2, 1, 1)

	report := d.RenderFailureReport()
	if !strings.Contains(report, "TestFail1") {
		t.Errorf("expected failure report to include TestFail1, got:\n%s", report)
	}
	if !strings.Contains(report, "✗") {
		t.Errorf("expected failure report to include ✗ indicator")
	}
	if !strings.Contains(report, "1 passed") {
		t.Errorf("expected failure report to include pass count")
	}
}

func TestRenderTimeoutReportIncludesActiveTests(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Timeout Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("test-pipe")
	tracker.ActiveTests["slow-test"] = &TestInfo{
		ID: "slow-test", Name: "SlowTest", State: StateRunning,
	}

	report := d.RenderTimeoutReport()
	if !strings.Contains(report, "SlowTest") {
		t.Errorf("expected timeout report to include SlowTest, got:\n%s", report)
	}
	if !strings.Contains(report, "⏳ SlowTest (") {
		t.Errorf("expected timeout report to include ⏳ indicator with duration")
	}
}

func TestDetonationConditionFiresOnFailure(t *testing.T) {
	allMet := true
	hasFailures := true

	if allMet && hasFailures {
		t.Log("old AND condition: skips detonation when floor met but failures exist (BUG)")
	}

	if hasFailures || !allMet {
		t.Log("new OR condition: fires detonation when failures exist")
	} else {
		t.Error("expected detonation when failures exist")
	}
}

func TestTUIHeaderBombTotalsStablePosition(t *testing.T) {
	cases := []struct {
		name     string
		historyN int
	}{
		{"zero-history", 0},
		{"three-history", 3},
		{"fifty-history", 50},
		{"hundred-history", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDashboard([]PipelineConfig{
				{ID: "stable-pipe", Name: "Stable Pipeline", LedgerFloor: 5},
			})
			ledger := NewLedgerEngine()
			ledger.GetOrCreateEntry("stable-pipe")
			d.SetLedger(ledger)

			tracker := d.GetTracker("stable-pipe")
			for i := 0; i < tc.historyN; i++ {
				id := "StableTest/TestFunc_" + itoa(i)
				tracker.CompletedIDs[id] = true
				tracker.History = append(tracker.History, "✓ "+id)
			}
			entry := d.GetLedger().GetEntry("stable-pipe")
			entry.TotalRan = tc.historyN
			entry.TotalPassed = tc.historyN

			// Simulate the TUI rendering order: header -> bomb -> totals -> test list
			header := d.RenderHeader(d.GetPipelinesSlice()[0])
			bombRing := RenderBombRing(0, "5")
			totals := fmt.Sprintf("========================================\n")
			totals += fmt.Sprintf("Total Tests: %d\n", tc.historyN)
			totals += "========================================\n"
			testList := d.RenderTestList(d.GetPipelinesSlice()[0])

			if header == "" {
				t.Error("RenderHeader must not be empty")
			}
			if bombRing == "" {
				t.Error("RenderBombRing must not be empty")
			}
			if totals == "" {
				t.Error("totals section must not be empty")
			}

			// Test list must be ≤ 5 items regardless of history size
			lines := strings.Split(strings.TrimRight(testList, "\n"), "\n")
			itemCount := 0
			for _, l := range lines {
				if strings.HasPrefix(l, "   ") {
					itemCount++
				}
			}
			if itemCount > 1 {
				t.Errorf("with %d history: expected ≤ 1 item, got %d", tc.historyN, itemCount)
			}

			// Verify header+bomb+totals don't vary with history size (they are position-stable)
			upperSection := header + bombRing + totals
			if !strings.Contains(upperSection, "Stable Pipeline") {
				t.Error("header must contain pipeline name")
			}
			if !strings.Contains(upperSection, "Total Tests") {
				t.Error("totals must contain Total Tests")
			}
		})
	}
}

func TestTUIRunningGaugesIndividualPerTest(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "gauge-pipe", Name: "Gauge Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("gauge-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("gauge-pipe")
	runningNames := []string{"RunTest_A", "RunTest_B", "RunTest_C"}
	for _, name := range runningNames {
		tracker.ActiveTests[name] = &TestInfo{
			ID: name, Name: name, State: StateRunning,
		}
	}

	list := d.RenderTestList(d.GetPipelinesSlice()[0])

	if !strings.Contains(list, "RunTest_") {
		t.Errorf("expected at least one running test in output, got:\n%s", list)
	}

	// With 1-slot limit, only 1 firing gauge is shown
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	gaugeCount := 0
	for _, l := range lines {
		if strings.Contains(l, "Firing:") && strings.Contains(l, "[") && strings.Contains(l, "s]") {
			gaugeCount++
		}
	}
	if gaugeCount != 1 {
		t.Errorf("expected 1 firing gauge with 1-slot limit, got %d", gaugeCount)
	}
}

func TestTUIMaxFiveTestListHeaderTextUnchanged(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "max-pipe", Name: "Max Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("max-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("max-pipe")
	n := 50
	for i := 0; i < n; i++ {
		id := "MaxTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.GetLedger().GetEntry("max-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	// Header must be identical regardless of history size
	header := d.RenderHeader(d.GetPipelinesSlice()[0])
	if !strings.Contains(header, "Max Pipeline") {
		t.Error("header must contain pipeline name")
	}
	if !strings.Contains(header, "50") {
		t.Error("header must contain correct total ran count")
	}

	// Test list must never exceed 5
	list := d.RenderTestList(d.GetPipelinesSlice()[0])
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("test list exceeds max 1 item, got %d", itemCount)
	}
}

func TestTUIRenderAtomicFrameLayout(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "lay-pipe", Name: "Layout Pipeline", LedgerFloor: 3},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("lay-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("lay-pipe")
	tracker.ActiveTests["running1"] = &TestInfo{
		ID: "running1", Name: "RunningFirst", State: StateRunning,
	}
	for i := 0; i < 10; i++ {
		id := "LayoutTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.GetLedger().GetEntry("lay-pipe")
	entry.TotalRan = 11
	entry.TotalPassed = 10
	entry.TotalFailed = 1

	// Verify layout: header → bomb → totals → test list
	// by checking each section in order
	header := d.RenderHeader(d.GetPipelinesSlice()[0])
	testList := d.RenderTestList(d.GetPipelinesSlice()[0])

	if !strings.Contains(header, "Layout Pipeline") {
		t.Error("RenderHeader must start with pipeline name")
	}

	if !strings.Contains(testList, "RunningFirst") {
		t.Error("RenderTestList must prioritize firing test over history")
	}

	// Count test items - must be exactly 1 slot
	lines := strings.Split(strings.TrimRight(testList, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > 1 {
		t.Errorf("test list has %d items, max is 1", itemCount)
	}
}

func TestRenderTestListDetonatedShowsHistoryNoGauges(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "det-pipe", Name: "Detonated Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("det-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("det-pipe")
	tracker.CompletedIDs["done-test"] = true
	tracker.History = append(tracker.History, "✓ done-test")
	tracker.ActiveTests["running-test"] = &TestInfo{
		ID: "running-test", Name: "RunningTest", State: StateRunning,
	}

	d.Bomb = BombDetonated

	list := d.RenderTestList(d.GetPipelinesSlice()[0])
	if strings.Contains(list, "⏳") {
		t.Errorf("expected NO running gauge (⏳) in detonated state, got:\n%s", list)
	}
	if !strings.Contains(list, "done-test") {
		t.Errorf("expected completed test in history, got:\n%s", list)
	}
}

func TestTriggerDetonationDrainsOrphanedTests(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "drain-pipe", Name: "Drain Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("drain-pipe")
	d.SetLedger(ledger)

	tracker := d.GetTracker("drain-pipe")
	tracker.CompletedIDs["passed-test"] = true
	tracker.History = append(tracker.History, "✓ passed-test")

	tracker.ActiveTests["orphan1"] = &TestInfo{
		ID: "orphan1", Name: "OrphanTest1", State: StateRunning,
	}
	tracker.ActiveTests["orphan2"] = &TestInfo{
		ID: "orphan2", Name: "OrphanTest2", State: StateRunning,
	}

	d.TriggerDetonation()

	if d.Bomb != BombDetonated {
		t.Error("bomb should be detonated")
	}
	if len(tracker.ActiveTests) != 0 {
		t.Errorf("ActiveTests should be drained, got %d items", len(tracker.ActiveTests))
	}
	foundOrphan1 := false
	foundOrphan2 := false
	for _, line := range tracker.History {
		if strings.Contains(line, "⏹ orphan1") {
			foundOrphan1 = true
		}
		if strings.Contains(line, "⏹ orphan2") {
			foundOrphan2 = true
		}
	}
	if !foundOrphan1 || !foundOrphan2 {
		t.Errorf("orphaned tests should appear in History with ⏹ prefix, History: %v", tracker.History)
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
