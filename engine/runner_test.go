package engine

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestRunnerStreamStdoutCapturesLines(t *testing.T) {
	// Test the streamStdout logic directly with a mock reader
	pipeCfg := PipelineConfig{
		ID:   "runner-test",
		Name: "Runner Test",
		TokenPatterns: TokenPatterns{
			TokenRun:  "Action.*run",
			TokenPass: "Action.*pass",
			TokenFail: "Action.*fail",
		},
	}

	dashboard := NewDashboard([]PipelineConfig{pipeCfg})
	runner := NewRunner(pipeCfg, dashboard)

	// Create a mock reader with test data
	testData := "line1\nline2\nline3\n"
	reader := bytes.NewBufferString(testData)

	var wg sync.WaitGroup
	var lines []string
	wg.Add(1)

	go func() {
		defer wg.Done()
		for line := range runner.StdoutChan {
			lines = append(lines, line)
		}
	}()

	// Call streamStdout directly with our mock reader
	runner.streamStdout(reader)
	wg.Wait()

	expected := []string{"line1", "line2", "line3"}
	if len(lines) != len(expected) {
		t.Errorf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, exp := range expected {
		if i >= len(lines) || lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}
}

func TestRunnerStreamStderrCapturesLines(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "runner-test",
		Name: "Runner Test",
		TokenPatterns: TokenPatterns{
			TokenRun:  "Action.*run",
			TokenPass: "Action.*pass",
			TokenFail: "Action.*fail",
		},
	}

	dashboard := NewDashboard([]PipelineConfig{pipeCfg})
	runner := NewRunner(pipeCfg, dashboard)

	testData := "error1\nerror2\n"
	reader := bytes.NewBufferString(testData)

	var wg sync.WaitGroup
	var lines []string
	wg.Add(1)

	go func() {
		defer wg.Done()
		for line := range runner.StderrChan {
			lines = append(lines, line)
		}
	}()

	runner.streamStderr(reader)
	wg.Wait()

	expected := []string{"error1", "error2"}
	if len(lines) != len(expected) {
		t.Errorf("expected %d lines, got %d: %v", len(expected), len(lines), lines)
	}
	for i, exp := range expected {
		if i >= len(lines) || lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}
}

func TestRunnerKillCleansUpChannels(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "runner-kill-test",
		Name: "Runner Kill Test",
		Command: CommandConfig{
			Type: "go_stack",
			Args: []string{"-c", "sleep 10"},
		},
		TokenPatterns: TokenPatterns{
			TokenRun:  "Action.*run",
			TokenPass: "Action.*pass",
			TokenFail: "Action.*fail",
		},
	}

	dashboard := NewDashboard([]PipelineConfig{pipeCfg})
	runner := NewRunner(pipeCfg, dashboard)
	runner.SetWorkDir(".")

	if err := runner.Start(); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	runner.Kill()

	// Wait should return without hanging
	exitCode := runner.Wait()
	if exitCode < 0 {
		t.Errorf("exit code should not be negative: %d", exitCode)
	}

	// Channels should be closed after wait
	select {
	case <-runner.StdoutChan:
		// channel closed, ok
	case <-time.After(100 * time.Millisecond):
		t.Error("StdoutChan not closed after wait")
	}

	select {
	case <-runner.StderrChan:
		// channel closed, ok
	case <-time.After(100 * time.Millisecond):
		t.Error("StderrChan not closed after wait")
	}

	// Raw capture methods should not panic
	_ = runner.GetRawStdout()
	_ = runner.GetRawStderr()
}