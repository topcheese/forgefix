package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"sync"
)

// ============================================================================
// PIPELINE RUNNER
// ============================================================================

type Runner struct {
	config     PipelineConfig
	dashboard  *Dashboard
	workDir    string
	mu         sync.Mutex
	StdoutChan chan string
	StderrChan chan string
	ExitChan   chan int
}

func NewRunner(config PipelineConfig, dashboard *Dashboard) *Runner {
	return &Runner{
		config:     config,
		dashboard:  dashboard,
		StdoutChan: make(chan string, 100),
		StderrChan: make(chan string, 100),
		ExitChan:   make(chan int, 1),
	}
}

func (r *Runner) SetWorkDir(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workDir = dir
}

func (r *Runner) Start() error {
	cmdStr, err := ExecuteCommand(r.config.Command, r.config.Command.Paths)
	if err != nil {
		return fmt.Errorf("failed to execute command: %v", err)
	}

	cmd := exec.Command("bash", "-c", cmdStr)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			r.StdoutChan <- scanner.Text()
		}
		close(r.StdoutChan)
	}()

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			r.StderrChan <- scanner.Text()
		}
		close(r.StderrChan)
	}()

	go func() {
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				r.dashboard.AddErrorCode(exitErr.ExitCode())
				r.ExitChan <- exitErr.ExitCode()
			} else {
				r.dashboard.AddErrorCode(1)
				r.ExitChan <- 1
			}
		} else {
			r.dashboard.AddErrorCode(0)
			r.ExitChan <- 0
		}
		close(r.ExitChan)
	}()

	return nil
}

func (r *Runner) Wait() int {
	exitCode := <-r.ExitChan
	return exitCode
}

func (r *Runner) ProcessLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	patterns := r.config.TokenPatterns
	matchedToken, tokenType := MatchTokenPatterns(line, patterns)

	if matchedToken != "" {
		r.dashboard.UpdatePipelineMetrics(r.config.ID, tokenType, "test", 0, matchedToken)
	}
}

func (r *Runner) GetExitCode() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return <-r.ExitChan
}

// ============================================================================
// SUBPROCESS MANAGER
// ============================================================================

type Subprocess struct {
	cmd      *exec.Cmd
	stdout   *outputWriter
	stderr   *outputWriter
	exitCode chan int
}

func NewSubprocess(command string) *Subprocess {
	return &Subprocess{
		cmd:      exec.Command("bash", "-c", command),
		stdout:   &outputWriter{buf: &bytes.Buffer{}},
		stderr:   &outputWriter{buf: &bytes.Buffer{}},
		exitCode: make(chan int, 1),
	}
}

func (s *Subprocess) Start() {
	s.cmd.Stdout = s.stdout
	s.cmd.Stderr = s.stderr
	go func() {
		if err := s.cmd.Run(); err != nil {
			s.exitCode <- s.cmd.ProcessState.ExitCode()
		}
		close(s.exitCode)
	}()
}

func (s *Subprocess) Wait() int {
	return <-s.exitCode
}

type outputWriter struct {
	buf *bytes.Buffer
}

func (w *outputWriter) Write(p []byte) (n int, err error) {
	return w.buf.Write(p)
}
