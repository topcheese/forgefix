package engine

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

const TestTimeoutSecs = 15

type BombState int

const (
	BombIdle     BombState = iota
	BombActive
	BombDefused
	BombDetonated
)

type FocusPanel int

const (
	FocusPanelFirst FocusPanel = iota
	FocusPanelSecond
	FocusPanelThird
	FocusPanelFourth
)

type UI struct {
	dashboard     *Dashboard
	mu            sync.Mutex
	renderTicker  *time.Ticker
	focusPanel    FocusPanel
	focusFailed   int
	showTitle     bool
	exitSignal    chan struct{}
	inFinalScreen bool
}

func (u *UI) StartKeyboardListener() {
	defer func() {
		recover()
	}()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	buf := make([]byte, 6)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}

		if n == 1 {
			switch {
			case buf[0] == 'q' || buf[0] == 'Q' || buf[0] == 'x' || buf[0] == 'X':
				if u.inFinalScreen {
					close(u.exitSignal)
					return
				}
				u.cleanExit()
				return
			case buf[0] == '\t':
				u.cycleFocus()
			case buf[0] == 3:
				return
			}
		} else if n >= 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65:
				if n >= 6 && buf[3] == 49 && buf[4] == 59 && buf[5] == 53 {
					u.focusPreviousFailed()
				}
			case 66:
				if n >= 6 && buf[3] == 49 && buf[4] == 59 && buf[5] == 53 {
					u.focusNextFailed()
				}
			case 67:
				if n >= 6 && buf[3] == 49 && buf[4] == 59 && buf[5] == 53 {
					u.focusRightPanel()
				}
			case 68:
				if n >= 6 && buf[3] == 49 && buf[4] == 59 && buf[5] == 53 {
					u.focusLeftPanel()
				}
			}
		}
	}
}

func (u *UI) cleanExit() {
	term.Restore(int(os.Stdin.Fd()), nil)
	fmt.Print("\033[H\033[2J\033[?25h")
	os.Exit(0)
}

func (u *UI) waitForExit() {
	<-u.exitSignal
	u.exitAltScreen()
}

func (u *UI) cycleFocus() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.focusPanel = (u.focusPanel + 1) % 4
}

func (u *UI) focusNextFailed() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.focusFailed++
}

func (u *UI) focusPreviousFailed() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.focusFailed > 0 {
		u.focusFailed--
	}
}

func (u *UI) focusRightPanel() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.showTitle = false
}

func (u *UI) focusLeftPanel() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.showTitle = true
}



func NewUI(dashboard *Dashboard) *UI {
	return &UI{
		dashboard:  dashboard,
		exitSignal: make(chan struct{}),
	}
}

func (u *UI) StartRenderLoop(quit chan struct{}) {
	u.enterAltScreen()

	u.render()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-quit:
			return
		case <-u.dashboard.StopCh():
			return
		case <-ticker.C:
			if u.dashboard.IsDirty() {
				u.dashboard.ClearDirty()
				u.render()
			}
		}
	}
}

func (u *UI) enterAltScreen() {
	fmt.Print("\033[?1049h\033[?25l")
}

func (u *UI) exitAltScreen() {
	fmt.Print("\033[?1049l\033[?25h\033[0m")
}

func (u *UI) cleanupTerminal() {
	u.exitAltScreen()
}

func (u *UI) render() {
	var r DashboardRenderer
	u.mu.Lock()
	defer u.mu.Unlock()

	var sb strings.Builder

	termHeight := 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && h > 0 {
		termHeight = h
		_ = w
	}

	sb.WriteString("\033[2J\033[H")

	pipelines := u.dashboard.GetActivePipelines()

	for _, pipeline := range pipelines {
		sb.WriteString(u.dashboard.RenderHeader(pipeline))
	}

	sb.WriteString("\n")

	r.WriteBombLive(&sb, u.dashboard, pipelines)

	fixedLines := len(pipelines) + 1 + 6 + 7
	availableForTests := termHeight - fixedLines
	if availableForTests < 0 {
		availableForTests = 0
	}

	var totalRan, totalPassed, totalFailed, totalFloor int
	if ledger := u.dashboard.GetLedger(); ledger != nil {
		totalRan = ledger.GetTotalRan()
		totalPassed = ledger.GetTotalPassed()
		totalFailed = ledger.GetTotalFailed()
		totalFloor = ledger.GetTotalFloor()
	}
	if u.dashboard.GetBomb() != BombDetonated {

		var statusLine string
		anyFailure := false
		for _, p := range u.dashboard.GetPipelinesSlice() {
			if e := u.dashboard.GetLedgerEntry(p.ID); e != nil && e.TotalFailed > 0 {
				anyFailure = true
				break
			}
		}
		if totalRan == 0 && u.dashboard.TestCommandCompleted {
			statusLine = fmt.Sprintf("%s❌ SYSTEM ERROR: No test execution streams were detected.%s\n", Red, Reset)
		} else if totalRan == 0 {
			statusLine = fmt.Sprintf("%s⏳ WAITING FOR TEST EXECUTION...%s\n", Yellow, Reset)
		} else if anyFailure {
			statusLine = fmt.Sprintf("%s❌ FAILURE: %d test(s) failed%s\n", Red, totalFailed, Reset)
		} else if totalFloor > 0 && totalPassed < totalFloor {
			statusLine = fmt.Sprintf("%s❌ REGRESSION: passed=%d below baseline=%d%s\n", Red, totalPassed, totalFloor, Reset)
		} else if totalPassed > 0 {
			statusLine = fmt.Sprintf("%s✅ RUNNING: %d passed / %d failed / floor %d%s\n", Green, totalPassed, totalFailed, totalFloor, Reset)
		}

		sb.WriteString(fmt.Sprintf("%s========================================\n", Bold))
		sb.WriteString(statusLine)
		sb.WriteString(fmt.Sprintf("%sTotal Tests: %d%s\n", White, totalRan, Reset))
		sb.WriteString(fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset))
		sb.WriteString(fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset))
		sb.WriteString(fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset))
		sb.WriteString(fmt.Sprintf("%s========================================\n", Bold))
	}

	sb.WriteString("\n")
	linesWritten := 0
	for _, pipeline := range pipelines {
		if linesWritten >= availableForTests {
			break
		}
		testList := u.dashboard.RenderTestList(pipeline)
		if testList != "" {
			sb.WriteString(testList)
			linesWritten += strings.Count(testList, "\n")
		}
	}

	for _, errMsg := range u.dashboard.GetSystemErrors() {
		if linesWritten >= availableForTests {
			break
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", Red, errMsg, Reset))
		linesWritten++
	}

	sb.WriteString("\033[J")
	fmt.Fprint(os.Stdout, sb.String())

	u.dashboard.BombFrame++
}

func (u *UI) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dashboard.SetPipelineActive(false)
}

func (u *UI) renderFinal(d *Dashboard, config *Config) {
	var r DashboardRenderer
	u.mu.Lock()
	defer u.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\033[H\033[2J")

	for _, p := range config.Pipelines {
		sb.WriteString(d.RenderHeader(p))
		r.WriteBombFinal(&sb, d, p)

		if list := d.RenderTestList(p); list != "" {
			sb.WriteString(list)
		}
	}

	if d.Bomb == BombDetonated {
		sb.WriteString("\n" + d.RenderFailureReport())
	}
	r.WriteTimeoutSection(&sb, d)
	sb.WriteString(d.FormatLedgerSummary(Bold, White, Green, Red, Reset))
	r.WriteSuccessFooter(&sb, d)

	sb.WriteString(Reset + "\n")
	sb.WriteString(Yellow + "Press 'q' to exit..." + Reset)

	fmt.Fprint(os.Stdout, sb.String())
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
	tracker := d.GetTestTrackersMap()[pipelineID]
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
	tracker := d.GetTestTrackersMap()[pipelineID]
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
	d.markDirty()
	if d.Bomb == BombDetonated {
		return
	}
	d.Bomb = BombDetonated
	d.drainOrphanedTests()
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
}

// drainOrphanedTests moves all remaining ActiveTests to History with "⏹" prefix.
// These are tests that were killed/interrupted by the detonation and never completed.
// Called under d.mu.Lock() from TriggerDetonation.
func (d *Dashboard) drainOrphanedTests() {
	for _, pipeline := range d.GetPipelinesSlice() {
		tracker := d.GetTestTrackersMap()[pipeline.ID]
		if tracker == nil {
			continue
		}
		var orphanIDs []string
		for id := range tracker.ActiveTests {
			if !tracker.CompletedIDs[id] {
				orphanIDs = append(orphanIDs, id)
			}
		}
		sawOrphan := false
		for _, id := range orphanIDs {
			info := tracker.ActiveTests[id]
			tracker.CompletedIDs[id] = true
			info.State = StateCompleted
			tracker.Completed[id] = info
			tracker.History = append(tracker.History, "⏹ "+id)
			delete(tracker.ActiveTests, id)
			sawOrphan = true
		}
		if sawOrphan {
			d.AddSystemError("⏹ Tests still running when bomb detonated")
		}
	}
}
