package engine

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// UI RENDERER
// ============================================================================

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

func (u *UI) StartRenderLoop() {
	u.mu.Lock()
	defer u.mu.Unlock()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for u.dashboard.PipelineActive {
		select {
		case <-ticker.C:
			u.render()
		default:
			logs := u.dashboard.GetErrorLogs()
			if len(logs) > 0 {
				u.dashboard.SetPipelineActive(false)
			}
		}
	}
}

func (u *UI) render() {
	u.mu.Lock()
	defer u.mu.Unlock()

	fmt.Print("\033[H\033[2J")

	for _, pipeline := range u.dashboard.GetActivePipelines() {
		fmt.Printf("%s\n", u.dashboard.RenderPanel(pipeline))
	}

	fmt.Print(u.dashboard.RenderSummary())
}

func (u *UI) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dashboard.SetPipelineActive(false)
}

// ============================================================================
// CONCURRENT RENDERER
// ============================================================================

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

// ============================================================================
// OUTPUT STREAMER
// ============================================================================

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
