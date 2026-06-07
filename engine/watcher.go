package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const DebounceDelay = 400 * time.Millisecond

type Watcher struct {
	config     *Config
	dashboard  *Dashboard
	watcher    *fsnotify.Watcher
	debounceMu sync.Mutex
	debounce   map[string]*time.Timer
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewWatcher(config *Config, dashboard *Dashboard) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		config:    config,
		dashboard: dashboard,
		watcher:   w,
		debounce:  make(map[string]*time.Timer),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

func (w *Watcher) Start(rootPath string) error {
	if err := w.walkAndWatch(rootPath); err != nil {
		return err
	}

	go w.eventLoop()
	return nil
}

func (w *Watcher) walkAndWatch(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if w.shouldExclude(path) {
				return filepath.SkipDir
			}
			if err := w.watcher.Add(path); err != nil {
				return err
			}
		}
		return nil
	})
}

func (w *Watcher) shouldExclude(path string) bool {
	base := filepath.Base(path)
	for _, excluded := range w.config.ExcludeDirs {
		if base == excluded {
			return true
		}
	}
	return false
}

func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Op&fsnotify.Write == 0 && event.Op&fsnotify.Create == 0 {
		return
	}
	if w.shouldExclude(event.Name) {
		return
	}
	if isTestOrSourceFile(event.Name) {
		w.debounceAndTrigger(event.Name)
	}
}

func isTestOrSourceFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".go", "_test.go", ".dart", ".ts", ".tsx", ".js", ".jsx":
		return true
	}
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	return false
}

func (w *Watcher) debounceAndTrigger(path string) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	if t, exists := w.debounce[path]; exists {
		t.Stop()
	}
	w.debounce[path] = time.AfterFunc(DebounceDelay, func() {
		w.debounceMu.Lock()
		delete(w.debounce, path)
		w.debounceMu.Unlock()
		w.triggerRun()
	})
}

func (w *Watcher) triggerRun() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("\n🔄 File change detected — re-running test suite...")

	w.dashboard.ResetTrackers()
	for _, pipeline := range w.config.Pipelines {
		w.dashboard.Ledger.GetOrCreateEntry(pipeline.ID)
	}
	w.dashboard.Bomb = BombActive
	w.dashboard.BombFrame = 0

	wg := runPipelines(w.config, w.dashboard, w.config.Pipelines, w.dashboard)
	go func() {
		wg.Wait()
		w.EmitLSPDiagnostics()
	}()
}

func (w *Watcher) Stop() {
	w.cancel()
	w.watcher.Close()
	w.debounceMu.Lock()
	for _, t := range w.debounce {
		t.Stop()
	}
	w.debounceMu.Unlock()
}

func runPipelines(config *Config, dashboard *Dashboard, pipelines []PipelineConfig, dash *Dashboard) *sync.WaitGroup {
	var wg sync.WaitGroup
	for _, pipeline := range pipelines {
		if dashboard.IsPipelineSkipped(pipeline.ID) {
			continue
		}
		wg.Add(1)
		go func(p PipelineConfig) {
			defer wg.Done()
			runner := NewRunner(p, dashboard)
			parser := NewParser(p)
			if err := runner.Start(); err != nil {
				dashboard.AddErrorCode(1)
				return
			}
			for line := range runner.StdoutChan {
				event, _ := parser.ParseLine(line)
				if event.MatchedToken != "" {
					dashboard.UpdatePipelineMetrics(p.ID, event.TokenType, event.TestID, event.Elapsed, event.MatchedToken, event.TestName)
				}
			}
		}(pipeline)
	}
	return &wg
}

func (w *Watcher) EmitLSPDiagnostics() {
	for _, pipeline := range w.config.Pipelines {
		tracker := w.dashboard.TestTrackers[pipeline.ID]
		if tracker == nil {
			continue
		}
		for _, info := range tracker.Completed {
			if info.State == StateCompleted && info.Elapsed > 0 {
				if info.Elapsed >= TestTimeoutSecs*1000 {
					uri := fmt.Sprintf("file://%s", info.ID)
					publish := LSPPublishDiagnostics{
						JSONRPC: "2.0",
						Method:  "textDocument/publishDiagnostics",
						Params: LSPDiagnosticsParams{
							URI:         uri,
							Diagnostics: []LSPDiagnostic{{
								Range: LSPRange{
									Start: LSPPosition{Line: 0, Character: 0},
									End:   LSPPosition{Line: 0, Character: 0},
								},
								Severity: 1,
								Source:    "forgefix",
								Message:   fmt.Sprintf("TEST TIMEOUT: test '%s' exceeded %ds hard limit", info.Name, TestTimeoutSecs),
								Code:      "TEST_TIMEOUT",
							}},
						},
					}
					data, _ := json.Marshal(publish)
					fmt.Println(string(data))
				}
			}
		}
	}
}

type LSPPublishDiagnostics struct {
	JSONRPC string               `json:"jsonrpc"`
	Method  string               `json:"method"`
	Params  LSPDiagnosticsParams `json:"params"`
}

type LSPDiagnosticsParams struct {
	URI         string         `json:"uri"`
	Diagnostics []LSPDiagnostic `json:"diagnostics"`
}

type LSPDiagnostic struct {
	Range    LSPRange     `json:"range"`
	Severity int          `json:"severity"`
	Source   string       `json:"source"`
	Message  string       `json:"message"`
	Code     string       `json:"code"`
}

type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}