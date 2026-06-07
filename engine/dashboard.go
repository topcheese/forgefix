package engine

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const TestTimeoutSecs = 15

type BombState int

const (
	BombActive   BombState = iota
	BombDefused
	BombDetonated
)

type UI struct {
	dashboard    *Dashboard
	mu           sync.Mutex
	renderTicker *time.Ticker
}

func NewUI(dashboard *Dashboard) *UI {
	return &UI{
		dashboard: dashboard,
	}
}

func (u *UI) StartRenderLoop(quit chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			u.render()
		}
	}
}

func (u *UI) render() {
	u.mu.Lock()
	defer u.mu.Unlock()

	var output strings.Builder

	fmt.Fprint(&output, "\033[H\033[2J")

	for _, pipeline := range u.dashboard.GetActivePipelines() {
		panel := u.dashboard.RenderPanel(pipeline)

		e := u.dashboard.Ledger.GetEntry(pipeline.ID)
		ef := pipeline.LedgerFloor
		if e != nil && e.HistoricalFloor > ef {
			ef = e.HistoricalFloor
		}

		if u.dashboard.Bomb == BombDefused {
			floorStr := fmt.Sprintf("%d", ef)
			panel += "\n" + Green + Bold + "   >> BOMB DEFUSED <<" + Reset + "\n"
			panel += Green + RenderBombDefused(floorStr) + Reset
		} else if u.dashboard.Bomb != BombDetonated {
			floorStr := fmt.Sprintf("%d", ef)
			panel += "\n" + RenderBombRing(u.dashboard.BombFrame, floorStr)
		}

		output.WriteString(panel)
		output.WriteString("\n")
	}

	if u.dashboard.Bomb == BombDetonated {
		output.WriteString(RenderDetonation())
	} else {
		totalRan := u.dashboard.Ledger.GetTotalRan()
		totalPassed := u.dashboard.Ledger.GetTotalPassed()
		totalFailed := u.dashboard.Ledger.GetTotalFailed()
		totalFloor := u.dashboard.Ledger.GetTotalFloor()

		var statusLine string
		anyFailure := false
		for _, p := range u.dashboard.Pipelines {
			if e := u.dashboard.Ledger.GetEntry(p.ID); e != nil && e.TotalFailed > 0 {
				anyFailure = true
				break
			}
		}
		if totalRan == 0 {
			statusLine = fmt.Sprintf("%s❌ SYSTEM ERROR: No test execution streams were detected.%s\n", Red, Reset)
		} else if anyFailure {
			statusLine = fmt.Sprintf("%s❌ FAILURE: %d test(s) failed%s\n", Red, totalFailed, Reset)
		} else if totalFloor > 0 && totalPassed < totalFloor {
			statusLine = fmt.Sprintf("%s❌ REGRESSION: passed=%d below baseline=%d%s\n", Red, totalPassed, totalFloor, Reset)
		} else if totalPassed > 0 {
			statusLine = fmt.Sprintf("%s✅ RUNNING: %d passed / %d failed / floor %d%s\n", Green, totalPassed, totalFailed, totalFloor, Reset)
		}

		output.WriteString(fmt.Sprintf("\n%s========================================\n", Bold))
		output.WriteString(statusLine)
		output.WriteString(fmt.Sprintf("%sTotal Tests: %d%s\n", White, totalRan, Reset))
		output.WriteString(fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset))
		output.WriteString(fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset))
		output.WriteString(fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset))
		output.WriteString(fmt.Sprintf("%s========================================\n", Bold))

		for _, errMsg := range u.dashboard.GetSystemErrors() {
			output.WriteString(fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset))
		}
	}

	fmt.Print(output.String())

	u.dashboard.BombFrame++
}

func (u *UI) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dashboard.SetPipelineActive(false)
}

type ConcurrentRenderer struct {
	dashboard *Dashboard
	mu        sync.Mutex
}

func NewConcurrentRenderer(dashboard *Dashboard) *ConcurrentRenderer {
	return &ConcurrentRenderer{
		dashboard: dashboard,
	}
}

func (cr *ConcurrentRenderer) Render() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fmt.Print("\033[H\033[2J")

	for _, pipeline := range cr.dashboard.GetActivePipelines() {
		fmt.Printf("%s\n", cr.dashboard.RenderPanel(pipeline))
	}

	fmt.Print(cr.dashboard.RenderSummary())
}

func (cr *ConcurrentRenderer) RenderError() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fmt.Print("\033[H\033[2J")
	fmt.Println(Red + "========================================" + Reset)
	fmt.Println(Red + "ERROR: Pipeline execution failed" + Reset)
	fmt.Println(Red + "========================================" + Reset)

	for _, log := range cr.dashboard.GetErrorLogs() {
		fmt.Printf("%s\n", log.Message)
	}
}

type OutputStreamer struct {
	mu       sync.Mutex
	lines    []string
	maxLines int
}

func NewOutputStreamer(maxLines int) *OutputStreamer {
	return &OutputStreamer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

func (os *OutputStreamer) AddLine(line string) {
	os.mu.Lock()
	defer os.mu.Unlock()

	os.lines = append(os.lines, line)
	if len(os.lines) > os.maxLines {
		os.lines = os.lines[len(os.lines)-os.maxLines:]
	}
}

func (os *OutputStreamer) GetLines() []string {
	os.mu.Lock()
	defer os.mu.Unlock()

	clone := make([]string, len(os.lines))
	copy(clone, os.lines)
	return clone
}

func (os *OutputStreamer) Clear() {
	os.mu.Lock()
	defer os.mu.Unlock()

	os.lines = os.lines[:0]
}

var bombRing = []string{"█", "▄", "▀", "░"}

// 5x5 radial circular fuse matrix positions (clockwise from top)
var bombMatrixPositions = []int{
	0, 1, 2, 3, 4,      // top row
	5, 6, 7, 8, 9,      // second row
	10, 11, 12, 13, 14, // third row (center is 12)
	15, 16, 17, 18, 19, // fourth row
	20, 21, 22, 23, 24, // bottom row
}

func getBombChar(pos, frame int) string {
	return bombRing[(pos+frame)%4]
}

func RenderBombRing(frame int, floorStr string) string {
	ch := func(pos int) string { return getBombChar(pos, frame) }
	return fmt.Sprintf("  %s %s %s %s %s  \n%s ┌───┐ %s \n%s │%2s│ %s\n%s └───┘ %s \n  %s %s %s %s %s  ",
		ch(0), ch(1), ch(2), ch(3), ch(4),
		ch(5), ch(9),
		ch(6), floorStr, ch(8),
		ch(10), ch(14),
		ch(11), ch(12), ch(13), ch(15), ch(16),
	)
}

func RenderBombDefused(floorStr string) string {
	return fmt.Sprintf(
		"%s %s %s %s %s\n"+
			"%s ┌───┐ %s\n"+
			"%s │%2s│ %s\n"+
			"%s └───┘ %s\n"+
			"%s %s %s %s %s",
		Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset,
		Green+"█"+Reset, Green+"█"+Reset,
		Green+"█"+Reset, floorStr, Green+"█"+Reset,
		Green+"█"+Reset, Green+"█"+Reset,
		Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset, Green+"█"+Reset,
	)
}

func RenderDetonation() string {
	explosion := `
` + Red + `      ▄▄▄▄▄▄▄▄▄▄▄
   ▄█████████████████▄
 ▄█████████████████████▄
███████████████████████████
███████  ` + Yellow + `BOMB DETONATED` + Red + `  ███████
███████ ` + Yellow + `SYSTEM SHATTERED` + Red + ` ███████
███████████████████████████
 ▀███████████████████████▀
   ▀███████████████████▀
     ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀` + Reset + `

` + Bold + Red + `💥 !!! BOMB DETONATED: SYSTEM SHATTERED !!! 💥` + Reset + `
`
	return explosion
}

func (d *Dashboard) GetActiveTestDurations(pipelineID string) []struct {
	Name     string
	Duration time.Duration
} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tracker := d.TestTrackers[pipelineID]
	if tracker == nil {
		return nil
	}
	var result []struct {
		Name     string
		Duration time.Duration
	}
	for _, info := range tracker.ActiveTests {
		result = append(result, struct {
			Name     string
			Duration time.Duration
		}{info.Name, time.Since(info.Started)})
	}
	return result
}

// GetTimeoutTests returns tests that have exceeded the timeout threshold
func (d *Dashboard) GetTimeoutTests(pipelineID string, timeoutSecs int) []struct {
	Name  string
	Elapsed int
} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tracker := d.TestTrackers[pipelineID]
	if tracker == nil {
		return nil
	}
	var timeoutTests []struct {
		Name  string
		Elapsed int
	}
	for _, info := range tracker.ActiveTests {
		elapsed := time.Since(info.Started).Seconds()
		if elapsed >= float64(timeoutSecs) {
			timeoutTests = append(timeoutTests, struct {
				Name  string
				Elapsed int
			}{info.Name, int(elapsed)})
		}
	}
	return timeoutTests
}

// TriggerDetonation sets the bomb state to detonated
func (d *Dashboard) TriggerDetonation() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Bomb = BombDetonated
	d.SystemErrors = append(d.SystemErrors, "🛑 TIMEOUT: Test stream exceeded 15s hard limit")
}
