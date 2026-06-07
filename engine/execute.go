package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func ExecuteSuite(config *Config, configDir string, aiMode bool, watchMode bool) {
	dashboard := NewDashboard(config.Pipelines)
	ledger, _ := LoadLedger(configDir)
	ledger.ResetCurrentRun()
	dashboard.Ledger = ledger
	dashboard.ResetTrackers()

	globalTimeout := 2 * time.Minute
	if config.GlobalTimeoutSeconds > 0 {
		globalTimeout = time.Duration(config.GlobalTimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()

	var ui *UI
	var uiQuit chan struct{}
	if !aiMode {
		fmt.Printf("Loaded config from: %s\n", configDir)
		if watchMode {
			fmt.Println("🔭 Watch mode active — monitoring for file changes...")
		} else {
			fmt.Printf("Global timeout: %v\n", globalTimeout)
		}

		ui = NewUI(dashboard)
		uiQuit = make(chan struct{})
		go ui.StartRenderLoop(uiQuit)
	}

	type pipelineRunner struct {
		Runner *Runner
		Parser *Parser
	}
	runners := make(map[string]*pipelineRunner)

	var wg sync.WaitGroup
	var parseWG sync.WaitGroup

	for _, pipeline := range config.Pipelines {
		workDir := configDir
		if lang, ok := config.Languages[pipeline.Type]; ok {
			if found := findAnchorDir(configDir, lang.RootAnchor, config.ExcludeDirs); found != "" {
				workDir = found
			} else {
				dashboard.MarkPipelineSkipped(pipeline.ID)
				dashboard.AddSystemError("Pipeline " + pipeline.ID + " skipped: " + lang.RootAnchor + " not found in tree under " + configDir)
				dashboard.AddErrorCode(0)
				continue
			}
		}
		p := pipeline
		runner := NewRunner(p, dashboard)
		runner.SetWorkDir(workDir)
		parser := NewParser(p)

		runners[pipeline.ID] = &pipelineRunner{Runner: runner, Parser: parser}

		wg.Add(1)
		go func(r *Runner) {
			defer wg.Done()
			if err := r.Start(); err != nil {
				dashboard.AddErrorCode(1)
			}
		}(runner)

		parseWG.Add(1)
		go func(r *Runner, p *Parser) {
			defer parseWG.Done()
			for line := range r.StdoutChan {
				event, _ := p.ParseLine(line)
				if event.MatchedToken != "" {
					dashboard.UpdatePipelineMetrics(p.Config().ID, event.TokenType, event.TestID, event.Elapsed, event.MatchedToken, event.TestName)
				}
			}
		}(runner, parser)

		go func(r *Runner) {
			for range r.StderrChan {
			}
		}(runner)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		parseWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		if ui != nil {
			close(uiQuit)
		}
		dashboard.SetPipelineActive(false)

		// Only evaluate detonation AFTER all tests have completed
		allMet := true
		for _, p := range config.Pipelines {
			if dashboard.IsPipelineSkipped(p.ID) {
				continue
			}
			e := dashboard.Ledger.GetEntry(p.ID)
			ef := p.LedgerFloor
			if e != nil && ef == 0 {
				ef = e.HistoricalFloor
			}
			if e == nil || e.TotalPassed < ef {
				allMet = false
				break
			}
		}

		// Check for any test failures
		hasFailures := dashboard.GetTotalFailures() > 0

		if hasFailures || !allMet {
			dashboard.Bomb = BombDetonated
			for _, pr := range runners {
				pr.Runner.Kill()
			}
		} else {
			dashboard.Bomb = BombDefused
		}

		if dashboard.Bomb == BombDetonated {
			if aiMode {
				EmitDetonated(dashboard)
			} else {
				time.Sleep(200 * time.Millisecond)
				var sb strings.Builder
				sb.WriteString("\033[H\033[2J")
				for _, p := range config.Pipelines {
					sb.WriteString(dashboard.RenderHeader(p))
					sb.WriteString("\n" + RenderDetonation() + "\n")
				}
				for _, p := range config.Pipelines {
					list := dashboard.RenderTestList(p)
					if list != "" {
						sb.WriteString(list)
					}
				}
				sb.WriteString(dashboard.RenderFailureReport())
				fmt.Print(sb.String())
			}
			os.Exit(1)
		}

	case <-ctx.Done():
		if ui != nil {
			close(uiQuit)
		}
		dashboard.TimeoutFired = true
		dashboard.SetPipelineActive(false)
		time.Sleep(200 * time.Millisecond)
		if aiMode {
			EmitJSON(dashboard)
		} else {
			var sb strings.Builder
			sb.WriteString("\033[H\033[2J")
			for _, p := range config.Pipelines {
				sb.WriteString(dashboard.RenderHeader(p))
				sb.WriteString("\n")
			}
			for _, p := range config.Pipelines {
				list := dashboard.RenderTestList(p)
				if list != "" {
					sb.WriteString(list)
				}
			}
			sb.WriteString(Red + Bold + "\n❌ TIMEOUT: pipeline execution exceeded global timeout\n" + Reset)
			sb.WriteString(dashboard.RenderTimeoutReport())
			fmt.Print(sb.String())
		}
		os.Exit(1)
	case <-sigChan:
		if ui != nil {
			close(uiQuit)
		}
		dashboard.SetPipelineActive(false)
		return
	}

	time.Sleep(200 * time.Millisecond)

	if err := SaveLedger(dashboard.Ledger, configDir); err != nil {
		if !aiMode {
			fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", err)
		}
	}

	if aiMode {
		EmitJSON(dashboard)
	} else {
		if dashboard.Bomb == BombDefused {
			time.Sleep(200 * time.Millisecond)
			var sb strings.Builder
			sb.WriteString("\033[H\033[2J")
			for _, p := range config.Pipelines {
				sb.WriteString(dashboard.RenderHeader(p))
				e := dashboard.Ledger.GetEntry(p.ID)
				ef := p.LedgerFloor
				if e != nil && e.HistoricalFloor > ef {
					ef = e.HistoricalFloor
				}
				floorStr := fmt.Sprintf("%d", ef)
				sb.WriteString("\n" + Green + Bold + "   >> BOMB DEFUSED <<" + Reset + "\n")
				sb.WriteString(Green + RenderBombDefused(floorStr) + Reset + "\n")
			}
			totalPassed := dashboard.Ledger.GetTotalPassed()
			totalFailed := dashboard.Ledger.GetTotalFailed()
			totalRan := dashboard.Ledger.GetTotalRan()
			totalFloor := dashboard.Ledger.GetTotalFloor()
			sb.WriteString(fmt.Sprintf("\n%s========================================\n", Bold))
			sb.WriteString(fmt.Sprintf("%s✅ ALL SYSTEMS NOMINAL: ALL TESTS PASSED CLEANLY%s\n", Green, Reset))
			sb.WriteString(fmt.Sprintf("%sTotal Tests: %d%s\n", White, totalRan, Reset))
			sb.WriteString(fmt.Sprintf("%sPassed: %s%d%s\n", White, Green, totalPassed, Reset))
			sb.WriteString(fmt.Sprintf("%sFailed: %s%d%s\n", White, Red, totalFailed, Reset))
			sb.WriteString(fmt.Sprintf("%sBaseline: %s%d%s\n", White, White, totalFloor, Reset))
			sb.WriteString(fmt.Sprintf("%s========================================\n", Bold))
			for _, p := range config.Pipelines {
				list := dashboard.RenderTestList(p)
				if list != "" {
					sb.WriteString("\n")
					sb.WriteString(list)
				}
			}
			sb.WriteString(fmt.Sprintf("\n%s   ▶▶▶ 🟢 [SUCCESS] BOMB DEFUSED: ALL SYSTEMS SECURE ◀◀◀%s\n", Green+Bold, Reset))
			fmt.Print(sb.String())
		}
	}

	totalRan := dashboard.Ledger.GetTotalRan()
	if totalRan == 0 {
		os.Exit(1)
	}
}

func findAnchorDir(root, anchor string, excludeDirs []string) string {
	if _, err := os.Stat(filepath.Join(root, anchor)); err == nil {
		return root
	}
	var found string
	_ = filepath.WalkDir(root, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			for _, excluded := range excludeDirs {
				if info.Name() == excluded {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if found != "" {
			return filepath.SkipAll
		}
		if info.Name() == anchor {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

func EmitJSON(d *Dashboard) {
	payload := d.ToAIPayload()
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func EmitDetonated(d *Dashboard) {
	payload := d.ToAIPayload()
	payload.Status = "DETONATED"
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func EmitAIError(code, detail string) {
	payload := AIResponsePayload{
		Status:  "error",
		Version: "forgefix/v1",
		Metrics: AIMetricsSummary{},
		Pipelines: []AIPipelineResult{
			{
				ID:              "system",
				Name:            "System",
				Status:          "error",
				SuggestedAction: "CONFIG_LOAD_FAILURE: " + detail + ". Verify that forgefix.yaml exists in the working directory.",
				ErrorDetails:    code + ": " + detail,
			},
		},
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(out))
}