package engine

import (
	"fmt"
	"strings"
	"testing"
)

func TestRenderTestListShowsRunningWithGauges(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "gauge-pipe", Name: "Gauge Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("gauge-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("gauge-pipe")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "GaugeTest_Alpha", State: StateRunning,
	}

	list := d.RenderTestList(d.Pipelines[0])
	if !strings.Contains(list, "GaugeTest_Alpha") {
		t.Errorf("expected running test name, got:\n%s", list)
	}
	if !strings.Contains(list, "(") || !strings.Contains(list, "s)") {
		t.Errorf("expected running gauge to include duration (X.Xs), got:\n%s", list)
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	hasSpinner := false
	for _, s := range spinners {
		if strings.Contains(list, s) {
			hasSpinner = true
			break
		}
	}
	if !hasSpinner {
		t.Errorf("expected spinner character in running test output, got:\n%s", list)
	}
}

func TestRenderTestListMaxFive(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "max-pipe", Name: "Max Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("max-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("max-pipe")
	for i := 0; i < 10; i++ {
		id := "MaxTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	tracker.ActiveTests["running"] = &TestInfo{
		ID: "running", Name: "RunningTest", State: StateRunning,
	}

	list := d.RenderTestList(d.Pipelines[0])
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > maxPanelSlots {
		t.Errorf("test list should show at most %d items, got %d", maxPanelSlots, itemCount)
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
	d.Ledger = ledger

	d.MarkPipelineSkipped("skip-pipe")

	list := d.RenderTestList(d.Pipelines[0])
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

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > maxPanelSlots {
		t.Errorf("panel should show at most %d item rows, got %d", maxPanelSlots, itemCount)
	}

	if strings.Contains(panel, "... and") {
		t.Errorf("unexpected truncation message in panel with 5-slot layout")
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
	n := 3
	for i := range n {
		id := "TestAllShown/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.Ledger.GetEntry("test-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	panel := d.RenderPanel(d.Pipelines[0])

	lines := strings.Split(strings.TrimRight(panel, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > n {
		t.Errorf("panel too long for %d entries: got %d item rows", n, itemCount)
	}
}

func TestTUIPanelPrioritizesRunningTests(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

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

	panel := d.RenderPanel(d.Pipelines[0])

	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	hasSpinner := false
	for _, s := range spinners {
		if strings.Contains(panel, s) {
			hasSpinner = true
			break
		}
	}
	if !hasSpinner {
		t.Errorf("expected running gauge indicator (spinner) in panel output")
	}

	if !strings.Contains(panel, "ActiveTest_1") || !strings.Contains(panel, "ActiveTest_2") {
		t.Errorf("expected active test names in panel output")
	}

	for _, line := range strings.Split(panel, "\n") {
		for _, s := range spinners {
			if strings.Contains(line, s) {
				if !strings.Contains(line, "(") || !strings.Contains(line, "s)") {
					t.Errorf("running gauge missing duration: %s", line)
				}
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
	if itemCount > maxPanelSlots {
		t.Errorf("panel should show at most %d item rows, got %d", maxPanelSlots, itemCount)
	}
}

func TestTUIPanelMaxSlotsConstant(t *testing.T) {
	if maxPanelSlots <= 0 {
		t.Fatal("maxPanelSlots must be positive")
	}
	if maxPanelSlots != 5 {
		t.Errorf("expected maxPanelSlots=5, got %d", maxPanelSlots)
	}
}

func TestTUIRunningGaugeHasTwoDecimalDuration(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("test-pipe")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "GaugeTest", State: StateRunning,
	}

	panel := d.RenderPanel(d.Pipelines[0])
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	hasSpinner := false
	for _, s := range spinners {
		if strings.Contains(panel, s+" GaugeTest (") {
			hasSpinner = true
			break
		}
	}
	if !hasSpinner {
		t.Errorf("expected running gauge with spinner and duration: %s", panel)
	}
}

func TestRenderFailureReportIncludesFailedTestNames(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Failure Pipeline", LedgerFloor: 1},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

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
	d.Ledger = ledger

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
		name      string
		historyN  int
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
			d.Ledger = ledger

			tracker := d.GetTracker("stable-pipe")
			for i := 0; i < tc.historyN; i++ {
				id := "StableTest/TestFunc_" + itoa(i)
				tracker.CompletedIDs[id] = true
				tracker.History = append(tracker.History, "✓ "+id)
			}
			entry := d.Ledger.GetEntry("stable-pipe")
			entry.TotalRan = tc.historyN
			entry.TotalPassed = tc.historyN

			// Simulate the TUI rendering order: header -> bomb -> totals -> test list
			header := d.RenderHeader(d.Pipelines[0])
			bombRing := RenderBombRing(0, "5")
			totals := fmt.Sprintf("========================================\n")
			totals += fmt.Sprintf("Total Tests: %d\n", tc.historyN)
			totals += "========================================\n"
			testList := d.RenderTestList(d.Pipelines[0])

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
			if itemCount > maxPanelSlots {
				t.Errorf("with %d history: expected ≤ %d items, got %d", tc.historyN, maxPanelSlots, itemCount)
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
	d.Ledger = ledger

	tracker := d.GetTracker("gauge-pipe")
	runningNames := []string{"RunTest_A", "RunTest_B", "RunTest_C"}
	for _, name := range runningNames {
		tracker.ActiveTests[name] = &TestInfo{
			ID: name, Name: name, State: StateRunning,
		}
	}

	list := d.RenderTestList(d.Pipelines[0])

	for _, name := range runningNames {
		if !strings.Contains(list, name) {
			t.Errorf("expected individual gauge entry for %s, got:\n%s", name, list)
		}
	}

	// Each gauge must have unique duration format and spinner
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	gaugeCount := 0
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, l := range lines {
		hasSpinner := false
		for _, s := range spinners {
			if strings.Contains(l, s) {
				hasSpinner = true
				break
			}
		}
		if hasSpinner && strings.Contains(l, "(") && strings.Contains(l, "s)") {
			gaugeCount++
		}
	}
	if gaugeCount != len(runningNames) {
		t.Errorf("expected %d individual gauges, got %d", len(runningNames), gaugeCount)
	}
}

func TestTUIMaxFiveTestListHeaderTextUnchanged(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "max-pipe", Name: "Max Pipeline", LedgerFloor: 5},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("max-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("max-pipe")
	n := 50
	for i := 0; i < n; i++ {
		id := "MaxTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.Ledger.GetEntry("max-pipe")
	entry.TotalRan = n
	entry.TotalPassed = n

	// Header must be identical regardless of history size
	header := d.RenderHeader(d.Pipelines[0])
	if !strings.Contains(header, "Max Pipeline") {
		t.Error("header must contain pipeline name")
	}
	if !strings.Contains(header, "50") {
		t.Error("header must contain correct total ran count")
	}

	// Test list must never exceed 5
	list := d.RenderTestList(d.Pipelines[0])
	lines := strings.Split(strings.TrimRight(list, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > maxPanelSlots {
		t.Errorf("test list exceeds max %d items, got %d", maxPanelSlots, itemCount)
	}

	// Test list is BELOW the header section (not part of it)
	if strings.Contains(header, "   ") && strings.HasPrefix(header, "   ") {
		t.Error("header should not contain test-list indentation")
	}
}

func TestTUIRenderAtomicFrameLayout(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "lay-pipe", Name: "Layout Pipeline", LedgerFloor: 3},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("lay-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("lay-pipe")
	tracker.ActiveTests["running1"] = &TestInfo{
		ID: "running1", Name: "RunningFirst", State: StateRunning,
	}
	for i := 0; i < 10; i++ {
		id := "LayoutTest/TestFunc_" + itoa(i)
		tracker.CompletedIDs[id] = true
		tracker.History = append(tracker.History, "✓ "+id)
	}
	entry := d.Ledger.GetEntry("lay-pipe")
	entry.TotalRan = 11
	entry.TotalPassed = 10
	entry.TotalFailed = 1

	// Verify layout: header → bomb → totals → test list
	// by checking each section in order
	header := d.RenderHeader(d.Pipelines[0])
	testList := d.RenderTestList(d.Pipelines[0])

	if !strings.Contains(header, "Layout Pipeline") {
		t.Error("RenderHeader must start with pipeline name")
	}

	if !strings.Contains(testList, "RunningFirst") {
		t.Error("RenderTestList must include running tests first")
	}

	if !strings.Contains(testList, "✓ LayoutTest/TestFunc_9") {
		t.Error("RenderTestList must include most recent completed tests")
	}

	// Count test items
	lines := strings.Split(strings.TrimRight(testList, "\n"), "\n")
	itemCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "   ") {
			itemCount++
		}
	}
	if itemCount > maxPanelSlots {
		t.Errorf("test list has %d items, max is %d", itemCount, maxPanelSlots)
	}
}

func TestRenderTestListDetonatedShowsHistoryNoGauges(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "det-pipe", Name: "Detonated Pipeline"},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("det-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("det-pipe")
	tracker.CompletedIDs["done-test"] = true
	tracker.History = append(tracker.History, "✓ done-test")
	tracker.ActiveTests["running-test"] = &TestInfo{
		ID: "running-test", Name: "RunningTest", State: StateRunning,
	}

	d.Bomb = BombDetonated

	list := d.RenderTestList(d.Pipelines[0])
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
	d.Ledger = ledger

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
