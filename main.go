package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ForgeFix/engine"
)

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

func main() {
	aiMode := flag.Bool("ai", false, "output structured JSON for AI agent consumption")
	flag.Parse()

	loaded, err := engine.LoadPipelineConfig()
	if err != nil {
		if *aiMode {
			emitAIError("CONFIG_LOAD_FAILURE", err.Error())
		} else {
			fmt.Println(engine.Red + "Error loading config: " + err.Error() + engine.Reset)
		}
		os.Exit(1)
	}
	config := loaded.Config

	globalTimeout := 2 * time.Minute
	if config.GlobalTimeoutSeconds > 0 {
		globalTimeout = time.Duration(config.GlobalTimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()

	if !*aiMode {
		fmt.Printf("Loaded config from: %s\n", loaded.ConfigDir)
		fmt.Printf("Global timeout: %v\n", globalTimeout)
	}

	dashboard := engine.NewDashboard(config.Pipelines)
	ledger, _ := engine.LoadLedger()
	ledger.ResetCurrentRun()
	dashboard.Ledger = ledger

	for _, pipeline := range config.Pipelines {
		workDir := loaded.ConfigDir
		if lang, ok := config.Languages[pipeline.Type]; ok {
			if found := findAnchorDir(loaded.ConfigDir, lang.RootAnchor, config.ExcludeDirs); found != "" {
				workDir = found
			} else {
				dashboard.MarkPipelineSkipped(pipeline.ID)
				dashboard.AddSystemError("Pipeline " + pipeline.ID + " skipped: " + lang.RootAnchor + " not found in tree under " + loaded.ConfigDir)
				dashboard.AddErrorCode(0)
				continue
			}
		}
		p := pipeline
		runner := engine.NewRunner(p, dashboard)
		runner.SetWorkDir(workDir)
		parser := engine.NewParser(p)

		go func(r *engine.Runner) {
			if err := r.Start(); err != nil {
				dashboard.AddErrorCode(1)
			}
		}(runner)

		go func(r *engine.Runner, p *engine.Parser) {
			for line := range r.StdoutChan {
				event, _ := p.ParseLine(line)
				if event.MatchedToken != "" {
					dashboard.UpdatePipelineMetrics(p.Config().ID, event.TokenType, event.TestID, event.Elapsed, event.MatchedToken)
				}
			}
		}(runner, parser)

		go func(r *engine.Runner) {
			for range r.StderrChan {
			}
		}(runner)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	completed := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for completed < len(config.Pipelines) {
		select {
		case <-ctx.Done():
			dashboard.TimeoutFired = true
			dashboard.SetPipelineActive(false)
			if *aiMode {
				emitJSON(dashboard)
			} else {
				fmt.Println(engine.Red + "\nTimeout reached. Shutting down..." + engine.Reset)
				fmt.Print(dashboard.RenderSummary())
			}
			os.Exit(1)
		case <-ticker.C:
			exitCodes := dashboard.GetExitCodes()
			completed = len(exitCodes)
			for _, exitCode := range exitCodes {
				if exitCode != 0 {
					if *aiMode {
						emitJSON(dashboard)
					} else {
						fmt.Println(engine.Red + "\nPipeline failed with exit code: " + fmt.Sprintf("%d", exitCode) + engine.Reset)
						fmt.Print(dashboard.RenderSummary())
					}
					os.Exit(exitCode)
				}
			}
		case <-sigChan:
			dashboard.SetPipelineActive(false)
			return
		}
	}

	if err := engine.SaveLedger(dashboard.Ledger); err != nil {
		if !*aiMode {
			fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", err)
		}
	}

	if *aiMode {
		emitJSON(dashboard)
	} else {
		fmt.Print(dashboard.RenderSummary())
	}

	totalRan := dashboard.Ledger.GetTotalRan()
	if totalRan == 0 {
		os.Exit(1)
	}
}

func emitJSON(d *engine.Dashboard) {
	payload := d.ToAIPayload()
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func emitAIError(code, detail string) {
	payload := engine.AIResponsePayload{
		Status:  "error",
		Version: "forgefix/v1",
		Metrics: engine.AIMetricsSummary{},
		Pipelines: []engine.AIPipelineResult{
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
