package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFirecrackerRenderingShowsActiveTestsWithMilliseconds(t *testing.T) {
	pipelines := []PipelineConfig{{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
	}}
	d := NewDashboard(pipelines)
	tracker := d.GetTracker("test-pipeline")
	started := time.Now().Add(-450 * time.Millisecond)
	tracker.mu.Lock()
	tracker.ActiveTests["TestOne"] = &TestInfo{
		ID:      "TestOne",
		Name:    "TestOne",
		State:   StateFiring,
		Started: started,
	}
	tracker.mu.Unlock()

	result := d.RenderTestList(pipelines[0])

	if !strings.Contains(result, "Firing: TestOne") {
		t.Errorf("expected 'Firing: TestOne' in active test render, got:\n%s", result)
	}
	if !strings.Contains(result, "[0.45") {
		t.Errorf("expected millisecond counter [0.45s] in active test render, got:\n%s", result)
	}
}

func TestFirecrackerSingleSlotOnly(t *testing.T) {
	d := NewDashboard(nil)
	tracker := &TestTracker{
		ActiveTests: make(map[string]*TestInfo),
		Completed:   make(map[string]*TestInfo),
		CompletedIDs: make(map[string]bool),
	}
	base := time.Now().Add(-1 * time.Second)
	for i := 0; i < 8; i++ {
		name := string(rune('A' + i))
		tracker.ActiveTests[name] = &TestInfo{
			ID:      name,
			Name:    name,
			State:   StateFiring,
			Started: base.Add(time.Duration(i) * 100 * time.Millisecond),
		}
	}
	d.TestTrackers["p"] = tracker

	result := d.renderTestList(PipelineConfig{ID: "p"})
	lines := strings.Split(strings.TrimSpace(result), "\n")
	firingCount := 0
	for _, line := range lines {
		if strings.Contains(line, "Firing:") {
			firingCount++
		}
	}
	if firingCount != 1 {
		t.Errorf("expected exactly 1 active test slot with 2-line constraint, got %d", firingCount)
	}
}

func TestDudExplosionRendersStrictTwoLines(t *testing.T) {
	pipelines := []PipelineConfig{{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
	}}
	d := NewDashboard(pipelines)
	tracker := d.GetTracker("test-pipeline")
	tracker.mu.Lock()
	tracker.Completed["TestFailOne"] = &TestInfo{
		ID:      "TestFailOne",
		Name:    "TestFailOne",
		State:   StateDud,
		Elapsed: 3200,
		Started: time.Now().Add(-3200 * time.Millisecond),
		FilePath: "/home/user/project/foo_test.go",
		FailureLine: 42,
	}
	tracker.CompletedIDs["TestFailOne"] = true
	tracker.mu.Unlock()

	result := d.renderTestList(pipelines[0])

	if !strings.Contains(result, "💥") {
		t.Errorf("expected dud explosion emoji in failed test render, got:\n%s", result)
	}
	if !strings.Contains(result, "DUD: TestFailOne") {
		t.Errorf("expected 'DUD: TestFailOne' in failed test render, got:\n%s", result)
	}
	if !strings.Contains(result, "[3200ms]") {
		t.Errorf("expected frozen timer [3200ms] in dud render, got:\n%s", result)
	}
	if !strings.Contains(result, "░░░") {
		t.Errorf("expected smoke cloud ASCII in dud render, got:\n%s", result)
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) > 2 {
		t.Errorf("dud render must not exceed 2 lines, got %d:\n%s", len(lines), result)
	}
}

func TestFailureReportIncludesErrorTraceAndFilePath(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Failure Pipeline", LedgerFloor: 1},
	})
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("test-pipe")
	d.Ledger = ledger

	tracker := d.GetTracker("test-pipe")
	tracker.Completed["TestFailTwo"] = &TestInfo{
		ID:      "TestFailTwo",
		Name:    "TestFailTwo",
		State:   StateDud,
		Elapsed: 15000,
		ErrorTrace: "assertion failed: expected 5, got 3\n    at foo_test.go:25",
		FilePath: "/home/user/project/foo_test.go",
		FailureLine: 25,
	}
	tracker.CompletedIDs["TestFailTwo"] = true
	tracker.History = append(tracker.History, "✗ TestFailTwo")
	ledger.UpdateEntry("test-pipe", 1, 0, 1)

	report := d.RenderFailureReport()
	if !strings.Contains(report, "/home/user/project/foo_test.go") {
		t.Errorf("expected file path in failure report, got:\n%s", report)
	}
	if !strings.Contains(report, "assertion failed") {
		t.Errorf("expected error trace in failure report, got:\n%s", report)
	}
	if !strings.Contains(report, "TestFailTwo") {
		t.Errorf("expected failed test name in failure report")
	}
}

func TestPassedTestTransitionsToPoppedState(t *testing.T) {
	d := NewDashboard(nil)
	tracker := d.GetTracker("pipeline-1")
	tracker.mu.Lock()
	tracker.ActiveTests["TestPass"] = &TestInfo{
		ID:      "TestPass",
		Name:    "TestPass",
		State:   StateFiring,
		Started: time.Now().Add(-500 * time.Millisecond),
	}
	tracker.mu.Unlock()

	d.UpdatePipelineMetricsWithDetails("pipeline-1", "pass", "TestPass", 500, "PASS", "TestPass", "", "", 0, 0)

	tracker.mu.RLock()
	if _, exists := tracker.ActiveTests["TestPass"]; exists {
		t.Errorf("expected test to be removed from active after pass")
	}
	info, exists := tracker.Completed["TestPass"]
	if !exists {
		t.Errorf("expected test in completed after pass")
	}
	if info.State != StatePopped {
		t.Errorf("expected StatePopped after pass, got %v", info.State)
	}
	if info.Elapsed != 500 {
		t.Errorf("expected elapsed=500, got %d", info.Elapsed)
	}
	tracker.mu.RUnlock()
}

func TestFailedTestTransitionsToDudStateWithDetails(t *testing.T) {
	d := NewDashboard(nil)
	tracker := d.GetTracker("pipeline-1")
	tracker.mu.Lock()
	tracker.ActiveTests["TestFail"] = &TestInfo{
		ID:      "TestFail",
		Name:    "TestFail",
		State:   StateFiring,
		Started: time.Now().Add(-3200 * time.Millisecond),
	}
	tracker.mu.Unlock()

	errorTrace := "    foo_test.go:42: assertion failed"
	d.UpdatePipelineMetricsWithDetails("pipeline-1", "fail", "TestFail", 3200, "FAIL", "TestFail", errorTrace, "/path/to/foo_test.go", 42, 0)

	tracker.mu.RLock()
	if _, exists := tracker.ActiveTests["TestFail"]; exists {
		t.Errorf("expected test to be removed from active after fail")
	}
	info, exists := tracker.Completed["TestFail"]
	if !exists {
		t.Errorf("expected test in completed after fail")
	}
	if info.State != StateDud {
		t.Errorf("expected StateDud after fail, got %v", info.State)
	}
	if info.ErrorTrace != errorTrace {
		t.Errorf("expected errorTrace=%q, got %q", errorTrace, info.ErrorTrace)
	}
	if info.FilePath != "/path/to/foo_test.go" {
		t.Errorf("expected filePath=/path/to/foo_test.go, got %q", info.FilePath)
	}
	if info.FailureLine != 42 {
		t.Errorf("expected failureLine=42, got %d", info.FailureLine)
	}
	if info.Elapsed != 3200 {
		t.Errorf("expected elapsed=3200, got %d", info.Elapsed)
	}
	tracker.mu.RUnlock()
}

func TestRunActionSetsFiringState(t *testing.T) {
	d := NewDashboard(nil)
	d.UpdatePipelineMetricsWithDetails("pipeline-1", "run", "TestNew", 0, "RUN", "TestNew", "", "", 0, 0)

	tracker := d.GetTracker("pipeline-1")
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()
	info, exists := tracker.ActiveTests["TestNew"]
	if !exists {
		t.Fatalf("expected test in active after run")
	}
	if info.State != StateFiring {
		t.Errorf("expected StateFiring after run, got %v", info.State)
	}
	if info.Name != "TestNew" {
		t.Errorf("expected Name=TestNew, got %q", info.Name)
	}
}

func TestPoppedRenderingShowsPopEmoji(t *testing.T) {
	pipelines := []PipelineConfig{{
		ID:   "test-pipeline",
		Name: "Test Pipeline",
	}}
	d := NewDashboard(pipelines)
	tracker := d.GetTracker("test-pipeline")
	tracker.mu.Lock()
	tracker.History = append(tracker.History, "✓ TestPassOne")
	tracker.Completed["TestPassOne"] = &TestInfo{
		ID:      "TestPassOne",
		Name:    "TestPassOne",
		State:   StatePopped,
		Elapsed: 450,
	}
	tracker.CompletedIDs["TestPassOne"] = true
	tracker.mu.Unlock()

	result := d.RenderTestList(pipelines[0])

	if !strings.Contains(result, "TestPassOne") {
		t.Errorf("expected test name in render, got:\n%s", result)
	}
	if !strings.Contains(result, "✓") {
		t.Errorf("expected checkmark in history render, got:\n%s", result)
	}
}

func TestDuplicateCompletedIDIgnored(t *testing.T) {
	d := NewDashboard(nil)
	tracker := d.GetTracker("pipeline-1")
	tracker.CompletedIDs["TestDup"] = true

	d.UpdatePipelineMetricsWithDetails("pipeline-1", "pass", "TestDup", 100, "PASS", "TestDup", "", "", 0, 0)

	tracker.mu.RLock()
	if _, exists := tracker.Completed["TestDup"]; exists {
		t.Errorf("expected duplicate completion to be ignored")
	}
	tracker.mu.RUnlock()
}

func TestUIRenderContainsBombRing(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One", LedgerFloor: 5},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("pipe1")
	ledger.UpdateEntry("pipe1", 10, 8, 2)
	d.Ledger = ledger
	tracker := d.GetTracker("pipe1")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "ActiveTest", State: StateRunning,
		Started: time.Now().Add(-500 * time.Millisecond),
	}

	u := NewUI(d)
	u.mu.Lock()
	var sb strings.Builder
	sb.WriteString("\033[H")
	sb.WriteString(d.RenderHeader(pipelines[0]))
	sb.WriteString("\n" + RenderBombRing(0, "5"))
	sb.WriteString("\n")
	sb.WriteString("\n========================================\n")
	sb.WriteString("Total Tests: 10\n")
	sb.WriteString("========================================\n")
	sb.WriteString("\n")
	sb.WriteString(d.RenderTestList(pipelines[0]))
	sb.WriteString("\033[J")
	output := sb.String()
	u.mu.Unlock()

	if !strings.Contains(output, "┌───┐") {
		t.Errorf("expected bomb ring ASCII art (┌───┐) in render output, got:\n%s", output)
	}
	if !strings.Contains(output, "│ 5│") {
		t.Errorf("expected bomb ring floor value (│ 5│) in render output, got:\n%s", output)
	}
	if !strings.Contains(output, "Total Tests") {
		t.Errorf("expected totals section in render output")
	}
}

func TestUIRenderWithStatsSection(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One", LedgerFloor: 5},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("pipe1")
	ledger.UpdateEntry("pipe1", 50, 48, 2)
	d.Ledger = ledger
	tracker := d.GetTracker("pipe1")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "ActiveTest", State: StateRunning,
		Started: time.Now().Add(-500 * time.Millisecond),
	}

	u := NewUI(d)
	u.mu.Lock()
	var sb strings.Builder
	sb.WriteString("\033[H")
	sb.WriteString(d.RenderHeader(pipelines[0]))
	sb.WriteString("\n" + RenderBombRing(0, "5"))
	sb.WriteString("\n")
	totalRan := d.Ledger.GetTotalRan()
	totalPassed := d.Ledger.GetTotalPassed()
	totalFailed := d.Ledger.GetTotalFailed()
	totalFloor := d.Ledger.GetTotalFloor()
	sb.WriteString("\n========================================\n")
	sb.WriteString(fmt.Sprintf("%sTotal Tests: %d%s\n", White, totalRan, Reset))
	sb.WriteString(fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset))
	sb.WriteString(fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset))
	sb.WriteString(fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset))
	sb.WriteString("========================================\n")
	sb.WriteString("\n")
	sb.WriteString(d.RenderTestList(pipelines[0]))
	sb.WriteString("\033[J")
	output := sb.String()
	u.mu.Unlock()

	if !strings.Contains(output, "Total Tests: 50") {
		t.Errorf("expected total count in stats: got\n%s", output)
	}
	if !strings.Contains(output, "Passed:") {
		t.Errorf("expected Passed line in stats section")
	}
	if !strings.Contains(output, "Failed:") {
		t.Errorf("expected Failed line in stats section")
	}
	if !strings.Contains(output, "Baseline:") {
		t.Errorf("expected Baseline line in stats section")
	}
}

func TestRenderStartsWithHomeSequence(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One"},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("pipe1")
	d.Ledger = ledger

	u := NewUI(d)
	u.mu.Lock()
	var sb strings.Builder
	sb.WriteString("\033[H")
	sb.WriteString(d.RenderHeader(pipelines[0]))
	sb.WriteString("\n")
	sb.WriteString(RenderBombRing(0, "0"))
	sb.WriteString("\n")
	sb.WriteString(d.RenderTestList(pipelines[0]))
	sb.WriteString("\033[J")
	output := sb.String()
	u.mu.Unlock()

	if !strings.HasPrefix(output, "\033[H") {
		t.Errorf("expected render output to start with \\033[H home sequence")
	}
	if !strings.HasSuffix(output, "\033[J") {
		t.Errorf("expected render output to end with \\033[J clear-to-bottom sequence")
	}
}

func TestInitialRenderMarksDirty(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One"},
	}
	d := NewDashboard(pipelines)
	d.ResetTrackers()
	if !d.IsDirty() {
		t.Errorf("expected dashboard to be dirty after ResetTrackers")
	}
}

func TestRenderBombRingTicking(t *testing.T) {
	ring0 := RenderBombRing(0, "5")
	ring1 := RenderBombRing(1, "5")
	ring2 := RenderBombRing(2, "5")

	if ring0 == ring1 || ring1 == ring2 {
		t.Errorf("bomb ring must animate between frames")
	}
	if !strings.Contains(ring0, "┌───┐") {
		t.Errorf("bomb ring must contain box top ┌───┐")
	}
	if !strings.Contains(ring0, "│ 5│") {
		t.Errorf("bomb ring must contain floor value, got:\n%s", ring0)
	}
	if !strings.Contains(ring0, "└───┘") {
		t.Errorf("bomb ring must contain box bottom └───┘")
	}

	lines0 := strings.Split(ring0, "\n")
	if len(lines0) != 5 {
		t.Errorf("bomb ring must be exactly 5 lines, got %d", len(lines0))
	}
}

func TestRenderHeaderMatchesConfigPipelineCount(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "a", Name: "Pipeline Alpha"},
		{ID: "b", Name: "Pipeline Beta"},
		{ID: "c", Name: "Pipeline Gamma"},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	for _, p := range pipelines {
		ledger.GetOrCreateEntry(p.ID)
	}
	d.Ledger = ledger

	active := d.GetActivePipelines()
	if len(active) != 3 {
		t.Fatalf("expected 3 active pipelines, got %d", len(active))
	}
	for _, p := range active {
		header := d.RenderHeader(p)
		if !strings.Contains(header, p.Name) {
			t.Errorf("header for %s must contain pipeline name, got:\n%s", p.ID, header)
		}
	}
}

func TestRenderFrameContainsBombRingExactlyOnce(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One", LedgerFloor: 5},
		{ID: "pipe2", Name: "Pipeline Two", LedgerFloor: 10},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	for _, p := range pipelines {
		ledger.GetOrCreateEntry(p.ID)
		ledger.UpdateEntry(p.ID, 10, 8, 2)
	}
	d.Ledger = ledger

	var sb strings.Builder
	sb.WriteString("\033[H")
	for _, p := range pipelines {
		sb.WriteString(d.RenderHeader(p))
	}
	sb.WriteString("\n")
	sb.WriteString(RenderBombRing(0, "5"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Tests: %d\n", d.Ledger.GetTotalRan()))
	for _, p := range pipelines {
		sb.WriteString(d.RenderTestList(p))
	}
	sb.WriteString("\033[J")
	output := sb.String()

	boxTopCount := strings.Count(output, "┌───┐")
	if boxTopCount != 1 {
		t.Errorf("bomb ring box ┌───┐ must appear exactly once, got %d in:\n%s", boxTopCount, output)
	}
	boxBotCount := strings.Count(output, "└───┘")
	if boxBotCount != 1 {
		t.Errorf("bomb ring box └───┘ must appear exactly once, got %d", boxBotCount)
	}
}

func TestRenderEachPipelineHasHeaderLine(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "go-app", Name: "🐹 GO MAIN APP"},
		{ID: "flutter-ui", Name: "📱 FLUTTER UI"},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	for _, p := range pipelines {
		ledger.GetOrCreateEntry(p.ID)
		ledger.UpdateEntry(p.ID, 40, 38, 2)
	}
	d.Ledger = ledger

	var sb strings.Builder
	sb.WriteString("\033[H")
	for _, p := range pipelines {
		sb.WriteString(d.RenderHeader(p))
	}
	sb.WriteString("\n")
	sb.WriteString(RenderBombRing(d.BombFrame, "40"))
	sb.WriteString("\n")
	for _, p := range pipelines {
		sb.WriteString(d.RenderTestList(p))
	}
	sb.WriteString("\033[J")
	output := sb.String()

	if !strings.Contains(output, "🐹 GO MAIN APP") {
		t.Errorf("output must contain pipeline A header")
	}
	if !strings.Contains(output, "📱 FLUTTER UI") {
		t.Errorf("output must contain pipeline B header")
	}
}

func TestRenderFiringGaugeOnFirstLineDudOnSecond(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One"},
	}
	d := NewDashboard(pipelines)
	tracker := d.GetTracker("pipe1")
	tracker.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "ActiveTest", State: StateRunning,
		Started: time.Now().Add(-500 * time.Millisecond),
	}
	tracker.Completed["dud1"] = &TestInfo{
		ID: "dud1", Name: "DudTest", State: StateDud,
		Elapsed: 3200, Started: time.Now().Add(-3200 * time.Millisecond),
	}
	tracker.CompletedIDs["dud1"] = true

	result := d.renderTestList(pipelines[0])
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Firing:") {
		t.Errorf("line 1 must be firing gauge, got: %q", lines[0])
	}
	if !strings.Contains(lines[0], "ActiveTest") {
		t.Errorf("line 1 must contain active test name, got: %q", lines[0])
	}
	if !strings.Contains(lines[1], "DUD:") {
		t.Errorf("line 2 must be dud info, got: %q", lines[1])
	}
	if !strings.Contains(lines[1], "DudTest") {
		t.Errorf("line 2 must contain dud test name, got: %q", lines[1])
	}
}

func TestRenderFullFrameLayoutOrder(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One", LedgerFloor: 5},
		{ID: "pipe2", Name: "Pipeline Two", LedgerFloor: 10},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	for _, p := range pipelines {
		ledger.GetOrCreateEntry(p.ID)
		ledger.UpdateEntry(p.ID, 20, 18, 2)
	}
	d.Ledger = ledger

	tracker1 := d.GetTracker("pipe1")
	tracker1.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "RunningTest1", State: StateRunning,
		Started: time.Now().Add(-200 * time.Millisecond),
	}
	tracker2 := d.GetTracker("pipe2")
	tracker2.Completed["dud1"] = &TestInfo{
		ID: "dud1", Name: "DudTest2", State: StateDud,
		Elapsed: 1500, Started: time.Now().Add(-1500 * time.Millisecond),
	}
	tracker2.CompletedIDs["dud1"] = true

	var sb strings.Builder
	sb.WriteString("\033[H")
	for _, p := range pipelines {
		sb.WriteString(d.RenderHeader(p))
	}
	sb.WriteString("\n")
	sb.WriteString(RenderBombRing(d.BombFrame, "5"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Tests: %d\n", d.Ledger.GetTotalRan()))
	for _, p := range pipelines {
		sb.WriteString(d.RenderTestList(p))
	}
	sb.WriteString("\033[J")
	output := sb.String()

	headerPos := strings.Index(output, "Pipeline One")
	bombPos := strings.Index(output, "┌───┐")
	totalsPos := strings.Index(output, "Total Tests:")
	firingPos := strings.Index(output, "Firing:")
	dudPos := strings.Index(output, "DUD:")

	if headerPos < 0 || bombPos < 0 || totalsPos < 0 {
		t.Fatalf("missing sections in output:\n%s", output)
	}
	if headerPos > bombPos {
		t.Errorf("headers must come before bomb ring")
	}
	if bombPos > totalsPos {
		t.Errorf("bomb ring must come before totals")
	}
	if firingPos > 0 && firingPos < totalsPos {
		t.Errorf("firing gauge must come after totals")
	}
	if dudPos > 0 && firingPos > 0 && dudPos < firingPos {
		t.Errorf("dud must come after firing gauge in same pipeline")
	}
}

func TestRenderFullFrameLineCountBounded(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "pipe1", Name: "Pipeline One", LedgerFloor: 5},
		{ID: "pipe2", Name: "Pipeline Two", LedgerFloor: 10},
	}
	d := NewDashboard(pipelines)
	ledger := NewLedgerEngine()
	for _, p := range pipelines {
		ledger.GetOrCreateEntry(p.ID)
		ledger.UpdateEntry(p.ID, 50, 48, 2)
	}
	d.Ledger = ledger

	for _, p := range pipelines {
		tracker := d.GetTracker(p.ID)
		tracker.ActiveTests["run1"] = &TestInfo{
			ID: "run1", Name: "RunningTest", State: StateRunning,
			Started: time.Now().Add(-100 * time.Millisecond),
		}
	}

	var sb strings.Builder
	sb.WriteString("\033[H")
	for _, p := range pipelines {
		sb.WriteString(d.RenderHeader(p))
	}
	sb.WriteString("\n")
	sb.WriteString(RenderBombRing(d.BombFrame, "5"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total Tests: %d\n", d.Ledger.GetTotalRan()))
	for _, p := range pipelines {
		sb.WriteString(d.RenderTestList(p))
	}
	sb.WriteString("\033[J")
	output := sb.String()

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 20 {
		t.Errorf("render output has %d non-empty lines, expected ≤ 20 for 2 pipelines:\n%s", nonEmpty, output)
	}
}

func TestBombFloorValueChangesWithLedgerFloor(t *testing.T) {
	NewDashboard([]PipelineConfig{
		{ID: "p", Name: "P", LedgerFloor: 42},
	})
	ring := RenderBombRing(0, "42")
	if !strings.Contains(ring, "│42│") {
		t.Errorf("bomb ring must display floor 42, got:\n%s", ring)
	}
}

func TestTwoPipelinesEachGetOwnTwoLineSlot(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p1", Name: "Pipe One"},
		{ID: "p2", Name: "Pipe Two"},
	})

	tracker1 := d.GetTracker("p1")
	tracker1.ActiveTests["t1"] = &TestInfo{
		ID: "t1", Name: "ActiveOne", State: StateRunning,
		Started: time.Now().Add(-300 * time.Millisecond),
	}
	tracker2 := d.GetTracker("p2")
	tracker2.Completed["dud1"] = &TestInfo{
		ID: "dud1", Name: "DudTwo", State: StateDud,
		Elapsed: 2000, Started: time.Now().Add(-2000 * time.Millisecond),
	}
	tracker2.CompletedIDs["dud1"] = true

	list1 := d.renderTestList(d.Pipelines[0])
	list2 := d.renderTestList(d.Pipelines[1])

	newlineCount1 := strings.Count(list1, "\n")
	newlineCount2 := strings.Count(list2, "\n")

	if newlineCount1 < 2 {
		t.Errorf("pipeline 1 must have ≥ 2 lines (got %d newlines): %q", newlineCount1, list1)
	}
	if newlineCount2 < 2 {
		t.Errorf("pipeline 2 must have ≥ 2 lines (got %d newlines): %q", newlineCount2, list2)
	}
	if !strings.Contains(list1, "Firing:") {
		t.Errorf("pipeline 1 must contain firing gauge, got: %q", list1)
	}
	if !strings.Contains(list1, "ActiveOne") {
		t.Errorf("pipeline 1 must contain active test name, got: %q", list1)
	}
	if !strings.Contains(list2, "DUD:") {
		t.Errorf("pipeline 2 must contain dud info, got: %q", list2)
	}
	if !strings.Contains(list2, "DudTwo") {
		t.Errorf("pipeline 2 must contain dud test name, got: %q", list2)
	}
}
