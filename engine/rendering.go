package engine

import (
	"fmt"
	"strings"
	"time"
)

var (
	Red       = "\033[31m"
	Reset     = "\033[0m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	White     = "\033[37m"
	Bold      = "\033[1m"
	Underline = "\033[4m"
)

const (
	FirecrackerEmoji = "🧨"
	DudEmoji         = "💥"
	SmokeEmoji       = "💨"
	PopEmoji         = "✨"
)

var smokeCloudFrames = []string{
	"    ░░░░░    ",
	"  ░░░░░░░░░  ",
	" ░░░░░░░░░░░ ",
	"░░░░░░░░░░░░░",
}

var firecrackerFrames = []string{"🧨", "💫", "✨", "💥"}

var musketFrames = []string{
	"[▄︻┳═一] 💥",
	"[▄︻┳═一] 💨",
	"[▄︻┳═一] 💫",
	"[▄︻┳═一] ✨",
}

var dudSmokeCloudFrames = []string{
	"  ░░░░░  ",
	" ░░░░░░░ ",
	"░░░░░░░░░",
	" ░░░░░░░ ",
	"  ░░░░░  ",
}

func truncateLabel(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func (d *Dashboard) renderHeader(pipeline PipelineConfig) string {
	name := pipeline.Name
	if d.SkippedPipelines[pipeline.ID] {
		return fmt.Sprintf("%s %s %s[SKIPPED]%s\n", truncateLabel(name, 22), Bold, Yellow, Reset)
	}

	entry := d.Ledger.GetEntry(pipeline.ID)
	if entry == nil {
		return fmt.Sprintf("%s%s%s\n", Bold, truncateLabel(name, 22), Reset)
	}

	effectiveFloor := pipeline.LedgerFloor
	if effectiveFloor == 0 {
		effectiveFloor = entry.HistoricalFloor
	}
	floorBroken := effectiveFloor > 0 && entry.TotalRan > 0 && entry.TotalPassed < effectiveFloor
	metricsColor := Reset
	if floorBroken {
		metricsColor = Red
	}

	if floorBroken {
		return fmt.Sprintf(
			"%s%s  Ran: %s%d%s | Pass: %s%d%s | Fail: %s%d%s  %s%s(⚠️ BROKEN: %d→%d)%s\n",
			Bold, truncateLabel(name, 22),
			metricsColor, entry.TotalRan, Reset,
			metricsColor, entry.TotalPassed, Reset,
			metricsColor, entry.TotalFailed, Reset,
			Bold, Red, effectiveFloor, entry.TotalPassed, Reset,
		)
	}
	return fmt.Sprintf(
		"%s%s  Ran: %s%d%s | Pass: %s%d%s | Fail: %s%d%s%s\n",
		Bold, truncateLabel(name, 22),
		metricsColor, entry.TotalRan, Reset,
		metricsColor, entry.TotalPassed, Reset,
		metricsColor, entry.TotalFailed, Reset, Reset,
	)
}

func (d *Dashboard) RenderHeader(pipeline PipelineConfig) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.renderHeader(pipeline)
}

func (d *Dashboard) renderTestList(pipeline PipelineConfig) string {
	if d.SkippedPipelines[pipeline.ID] {
		return ""
	}

	tracker := d.TestTrackers[pipeline.ID]
	if tracker == nil {
		return "\n\n"
	}

	var list strings.Builder

	activeCount := len(tracker.ActiveTests)
	hasActiveTests := d.Bomb != BombDetonated && !d.TimeoutFired && activeCount > 0

	var newestDud *TestInfo
	for _, info := range tracker.Completed {
		if info.State == StateDud {
			if newestDud == nil || info.Elapsed > newestDud.Elapsed {
				newestDud = info
			}
		}
	}

	hasDud := newestDud != nil

	// Line 1: Active test firing gauge OR empty line
	if hasActiveTests {
		var earliest *TestInfo
		for _, info := range tracker.ActiveTests {
			if info.State == StateRunning {
				info.State = StateFiring
			}
			if earliest == nil || info.Started.Before(earliest.Started) {
				earliest = info
			}
		}
		if earliest != nil {
			musketIdx := int(time.Now().UnixMilli()/100) % len(musketFrames)
			elapsed := time.Since(earliest.Started)
			elapsedMs := float64(elapsed.Milliseconds()) / 1000.0
			musketChar := musketFrames[musketIdx]
			list.WriteString(fmt.Sprintf("   %s%s %s Firing: %s [%.2fs]%s\n", Yellow, musketChar, Bold, truncateLabel(earliest.Name, 48), elapsedMs, Reset))
		} else {
			list.WriteString("\n")
		}
	} else {
		list.WriteString("\n")
	}

	// Line 2: Dud info on one line OR history/popped OR empty
	if hasDud {
		smokeIdx := int(time.Now().UnixMilli()/200) % len(dudSmokeCloudFrames)
		smokeLine := dudSmokeCloudFrames[smokeIdx]
		list.WriteString(fmt.Sprintf("   %s%s %s DUD: %s [%dms] %s %s%s\n", Red+Bold, DudEmoji, Reset, truncateLabel(newestDud.Name, 48), newestDud.Elapsed, smokeLine, SmokeEmoji, Reset))
	} else if !hasActiveTests {
		if len(tracker.History) > 0 {
			line := tracker.History[len(tracker.History)-1]
			color := Green
			if strings.HasPrefix(line, "✗") {
				color = Red
			} else if strings.HasPrefix(line, "⏹") {
				color = Yellow
			}
			list.WriteString(fmt.Sprintf("   %s%s%s\n", color, line, Reset))
		} else {
			for _, info := range tracker.Completed {
				list.WriteString(fmt.Sprintf("   %s✓ %s [%dms]%s\n", Green, info.Name, info.Elapsed, Reset))
				break
			}
		}
	} else {
		list.WriteString("\n")
	}

	return list.String()
}

func (d *Dashboard) RenderTestList(pipeline PipelineConfig) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.renderTestList(pipeline)
}

func (d *Dashboard) RenderPanel(pipeline PipelineConfig) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	list := d.renderTestList(pipeline)
	listStr := "\n" + list

	return d.renderHeader(pipeline) + listStr
}

func (d *Dashboard) RenderSummary() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalRan, totalPassed, totalFailed int
	var totalFloor int
	if d.Ledger != nil {
		totalRan = d.Ledger.GetTotalRan()
		totalPassed = d.Ledger.GetTotalPassed()
		totalFailed = d.Ledger.GetTotalFailed()
		totalFloor = d.Ledger.GetTotalFloor()
	}

	allNonSkippedOK := true
	var brokenFloors []struct {
		id    string
		name  string
		floor int
		got   int
	}
	if d.Ledger != nil {
		for _, p := range d.Pipelines {
			skipped := d.SkippedPipelines[p.ID]
			e := d.Ledger.GetEntry(p.ID)
			if !skipped && (e == nil || e.TotalRan == 0 || e.TotalFailed > 0) {
				allNonSkippedOK = false
			}
			ef := p.LedgerFloor
			if ef == 0 && e != nil {
				ef = e.HistoricalFloor
			}
			if !skipped && e != nil && e.TotalRan > 0 && ef > 0 && e.TotalPassed < ef {
				brokenFloors = append(brokenFloors, struct {
					id    string
					name  string
					floor int
					got   int
				}{p.ID, p.Name, ef, e.TotalPassed})
			}
		}
	} else {
		allNonSkippedOK = false
	}

	anyFloorBroken := len(brokenFloors) > 0

	var statusLine string
	if totalRan == 0 {
		statusLine = fmt.Sprintf("%s❌ SYSTEM ERROR: No test execution streams were detected or processed.%s\n", Red, Reset)
	} else if anyFloorBroken {
		statusLine = fmt.Sprintf("%s❌ BASELINE REGRESSION: %d pipeline(s) below configured floor%s\n", Red, len(brokenFloors), Reset)
	} else if totalFailed > 0 {
		statusLine = fmt.Sprintf("%s❌ FAILURE: %d test(s) failed%s\n", Red, totalFailed, Reset)
	} else if totalFloor > 0 && totalPassed < totalFloor {
		statusLine = fmt.Sprintf("%s❌ REGRESSION: passed=%d below baseline=%d%s\n", Red, totalPassed, totalFloor, Reset)
	} else if !allNonSkippedOK {
		statusLine = fmt.Sprintf("%s❌ CRITICAL FAILURE: Some pipeline(s) did not execute any tests or failed entirely%s\n", Red, Reset)
	} else {
		statusLine = fmt.Sprintf("%s✅ ALL SYSTEMS NOMINAL: ALL TESTS PASSED CLEANLY%s\n", Green, Reset)
	}

	result := fmt.Sprintf("\n%s========================================\n", Bold)
	result += statusLine
	result += fmt.Sprintf("%sTotal Tests: %d\n", White, totalRan) +
		fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset) +
		fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset) +
		fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset) +
		fmt.Sprintf("%s========================================\n", Bold)

	for _, pipeline := range d.Pipelines {
		result += "\n" + d.RenderPanel(pipeline)
	}

	for _, errMsg := range d.SystemErrors {
		result += fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset)
	}

	if anyFloorBroken {
		prompt := fmt.Sprintf("\n%s%s🤖 === FORGEFIX AUTOMATED AGENT CONTEXT PROMPT ===%s\n", Bold, Yellow, Reset)
		prompt += fmt.Sprintf("%sCopy and paste this segment into your AI coding agent interface to initiate self-healing:%s\n", Yellow, Reset)
		prompt += "-----------------------------------------------------------------\n"
		prompt += "You are an expert autonomous software engineer patching a test coverage floor regression.\n"
		for _, bf := range brokenFloors {
			prompt += fmt.Sprintf("- Pipeline ID: '%s'\n", bf.id)
			prompt += fmt.Sprintf("  - Baseline Floor Required: %d passing tests\n", bf.floor)
			prompt += fmt.Sprintf("  - Active Run passing Count: %d passing tests\n", bf.got)
		}
		prompt += "Analyze the git diff or file history inside the workspace. Determine if tests were deleted, commented out, or muted to mask a failure. Restore or rewrite the missing tests immediately.\n"
		prompt += "-----------------------------------------------------------------\n"
		result += prompt
	}

	return result
}

func (d *Dashboard) RenderFailureReport() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result strings.Builder

	result.WriteString(fmt.Sprintf("\n%s🔥 FAILED TESTS%s\n", Bold+Red, Reset))
	result.WriteString(fmt.Sprintf("%s══════════════════════════════════════════%s\n", Bold, Reset))

	for _, pipeline := range d.Pipelines {
		skipped := d.SkippedPipelines[pipeline.ID]
		if skipped {
			continue
		}
		tracker := d.TestTrackers[pipeline.ID]
		if tracker == nil {
			continue
		}

		hasFailures := false
		for _, line := range tracker.History {
			if strings.HasPrefix(line, "✗") {
				hasFailures = true
				break
			}
		}
		if !hasFailures {
			continue
		}

		result.WriteString(fmt.Sprintf("\n%s%s%s\n", Bold, pipeline.Name, Reset))
		for _, line := range tracker.History {
			if strings.HasPrefix(line, "✗") {
				testID := strings.TrimPrefix(line, "✗ ")
				result.WriteString(fmt.Sprintf("   %s%s%s\n", Red, line, Reset))
				if info, ok := tracker.Completed[testID]; ok {
					if info.FilePath != "" {
						loc := info.FilePath
						if info.FailureLine > 0 {
							loc += fmt.Sprintf(":%d", info.FailureLine)
							if info.FailureColumn > 0 {
								loc += fmt.Sprintf(":%d", info.FailureColumn)
							}
						}
						result.WriteString(fmt.Sprintf("      %s📍 %s%s\n", Red, loc, Reset))
					}
					if info.ErrorTrace != "" {
						for _, traceLine := range strings.Split(info.ErrorTrace, "\n") {
							if strings.TrimSpace(traceLine) != "" {
								result.WriteString(fmt.Sprintf("      %s░░ %s%s\n", Red, traceLine, Reset))
							}
						}
					}
				}
			}
		}
	}

	for _, errMsg := range d.SystemErrors {
		result.WriteString(fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset))
	}

	if d.Ledger != nil {
		totalRan := d.Ledger.GetTotalRan()
		totalPassed := d.Ledger.GetTotalPassed()
		totalFailed := d.Ledger.GetTotalFailed()
		totalFloor := d.Ledger.GetTotalFloor()
		result.WriteString(fmt.Sprintf("\n%s══════════════════════════════════════════%s\n", Bold, Reset))
		result.WriteString(fmt.Sprintf("%sTotal: %d passed, %d failed, %d ran, floor %d%s\n", White, totalPassed, totalFailed, totalRan, totalFloor, Reset))
	}

	return result.String()
}

func (d *Dashboard) RenderTimeoutReport() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result strings.Builder

	result.WriteString(fmt.Sprintf("\n%s⏰ TESTS STILL RUNNING AT TIMEOUT%s\n", Bold+Yellow, Reset))
	result.WriteString(fmt.Sprintf("%s══════════════════════════════════════════%s\n", Bold, Reset))

	for _, pipeline := range d.Pipelines {
		tracker := d.TestTrackers[pipeline.ID]
		if tracker == nil {
			continue
		}
		if len(tracker.ActiveTests) == 0 {
			continue
		}
		result.WriteString(fmt.Sprintf("\n%s%s%s\n", Bold, pipeline.Name, Reset))
		for _, info := range tracker.ActiveTests {
			elapsed := time.Since(info.Started).Seconds()
			result.WriteString(fmt.Sprintf("   %s⏳ %s (%.2fs)%s\n", Yellow, info.Name, elapsed, Reset))
		}
	}

	numFailed := 0
	for _, pipeline := range d.Pipelines {
		tracker := d.TestTrackers[pipeline.ID]
		if tracker == nil {
			continue
		}
		for _, line := range tracker.History {
			if strings.HasPrefix(line, "✗") {
				if numFailed == 0 {
					result.WriteString(fmt.Sprintf("\n%s❌ FAILED TESTS%s\n", Bold+Red, Reset))
				}
				result.WriteString(fmt.Sprintf("   %s%s%s\n", Red, line, Reset))
				numFailed++
			}
		}
	}

	for _, errMsg := range d.SystemErrors {
		result.WriteString(fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset))
	}

	if d.Ledger != nil {
		totalRan := d.Ledger.GetTotalRan()
		totalPassed := d.Ledger.GetTotalPassed()
		totalFailed := d.Ledger.GetTotalFailed()
		totalFloor := d.Ledger.GetTotalFloor()
		result.WriteString(fmt.Sprintf("\n%s══════════════════════════════════════════%s\n", Bold, Reset))
		result.WriteString(fmt.Sprintf("%sTotal: %d passed, %d failed, %d ran, floor %d%s\n", White, totalPassed, totalFailed, totalRan, totalFloor, Reset))
	}

	return result.String()
}

var SafeColorAllocator = []string{Red, Green, White}

func getSafeColor(index int) string {
	return SafeColorAllocator[index%len(SafeColorAllocator)]
}

// ---- DashboardRenderer: Bomb-state rendering ---------------------------------

type DashboardRenderer struct{}

func (DashboardRenderer) floorString(d *Dashboard, p PipelineConfig) string {
	ef := p.LedgerFloor
	if e := d.Ledger.GetEntry(p.ID); e != nil && e.HistoricalFloor > ef {
		ef = e.HistoricalFloor
	}
	return fmt.Sprintf("%d", ef)
}

func (DashboardRenderer) WriteBombDefused(sb *strings.Builder, floorStr string) {
	sb.WriteString("\n" + Green + Bold + "   >> BOMB DEFUSED <<" + Reset + "\n")
	sb.WriteString(Green + RenderBombDefused(floorStr) + Reset + "\n")
}

func (DashboardRenderer) WriteBombDetonated(sb *strings.Builder) {
	sb.WriteString("\n" + RenderDetonation() + "\n")
}

func (DashboardRenderer) WriteBombActive(sb *strings.Builder, d *Dashboard, floorStr string) {
	sb.WriteString(RenderBombRing(d.BombFrame, floorStr))
}

func (r DashboardRenderer) WriteBombLive(sb *strings.Builder, d *Dashboard, pipelines []PipelineConfig) {
	if d.Bomb == BombDetonated {
		r.WriteBombDetonated(sb)
		return
	}

	floorStr := ""
	if len(pipelines) > 0 {
		floorStr = r.floorString(d, pipelines[0])
	}

	if d.Bomb == BombDefused {
		r.WriteBombDefused(sb, floorStr)
	} else {
		r.WriteBombActive(sb, d, floorStr)
	}
	sb.WriteString("\n")
}

func (r DashboardRenderer) WriteBombFinal(sb *strings.Builder, d *Dashboard, p PipelineConfig) {
	if d.Bomb == BombDefused {
		floorStr := r.floorString(d, p)
		r.WriteBombDefused(sb, floorStr)
	} else if d.Bomb == BombDetonated {
		r.WriteBombDetonated(sb)
	} else {
		sb.WriteString("\n")
	}
}

func (DashboardRenderer) WriteTimeoutSection(sb *strings.Builder, d *Dashboard) {
	if !d.TimeoutFired {
		return
	}
	sb.WriteString(Red + Bold + "\n❌ TIMEOUT: pipeline execution exceeded global timeout\n" + Reset)
	sb.WriteString(d.RenderTimeoutReport())
}

func (DashboardRenderer) WriteSuccessFooter(sb *strings.Builder, d *Dashboard) {
	if d.Bomb != BombDefused {
		return
	}
	sb.WriteString(fmt.Sprintf("\n%s   ▶▶▶ 🟢 [SUCCESS] BOMB DEFUSED: ALL SYSTEMS SECURE ◀◀◀%s\n", Green+Bold, Reset))
}
