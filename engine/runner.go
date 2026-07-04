package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const readBufSize = 64 * 1024

type Runner struct {
	config      PipelineConfig
	dashboard   *Dashboard
	workDir     string
	mu          sync.Mutex
	StdoutChan  chan string
	StderrChan  chan string
	ExitChan    chan int
	rawStdout   bytes.Buffer
	rawStderr   bytes.Buffer
	cmd         *exec.Cmd
	timeoutOnce sync.Once
}

func NewRunner(config PipelineConfig, dashboard *Dashboard) *Runner {
	return &Runner{
		config:      config,
		dashboard:   dashboard,
		StdoutChan:  make(chan string, 10000),
		StderrChan:  make(chan string, 10000),
		ExitChan:    make(chan int, 1),
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
		return fmt.Errorf("failed to execute command: %w", err)
	}

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	r.cmd = cmd

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	go func() { defer func() { recover() }(); r.streamStdout(stdoutPipe) }()
	go func() { defer func() { recover() }(); r.streamStderr(stderrPipe) }()
	go func() { defer func() { recover() }(); r.waitForExit(cmd) }()
	go func() { defer func() { recover() }(); r.monitorTestTimeouts(cmd) }()

	return nil
}

func (r *Runner) monitorTestTimeouts(cmd *exec.Cmd) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if r.dashboard.Bomb == BombDetonated {
				return
			}
			// Check for per-test timeouts (15-second hard limit)
			timeoutTests := r.dashboard.GetTimeoutTests(r.config.ID, TestTimeoutSecs)
			for _, tt := range timeoutTests {
				r.timeoutOnce.Do(func() {
					errMsg := fmt.Sprintf("🛑 TIMEOUT: test '%s' exceeded %ds hard limit", tt.Name, TestTimeoutSecs)
					r.dashboard.AddSystemError(errMsg)
					r.dashboard.UpdatePipelineMetrics(r.config.ID, "fail", tt.Name, TestTimeoutSecs*1000, r.config.TokenPatterns.TokenFail, tt.Name)
					r.dashboard.TriggerDetonation()
					if cmd.Process != nil {
						syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					}
				})
				return
			}
		case <-r.ExitChan:
			return
		}
	}
}

func (r *Runner) streamStdout(reader io.Reader) {
	defer close(r.StdoutChan)

	dec := json.NewDecoder(reader)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return
			}
			return
		}
		select {
		case r.StdoutChan <- string(raw):
		default:
		}
	}
}

func (r *Runner) streamStderr(reader io.Reader) {
	defer close(r.StderrChan)

	buf := make([]byte, readBufSize)
	var partial bytes.Buffer

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := buf[:n]
			r.rawStderr.Write(data)

			start := 0
			for i := 0; i < n; i++ {
				if data[i] == '\n' {
					partial.Write(data[start:i])
					line := partial.String()
					partial.Reset()
					select {
					case r.StderrChan <- line:
					default:
					}
					start = i + 1
				}
			}
			if start < n {
				partial.Write(data[start:n])
			}
		}
		if err != nil {
			if partial.Len() > 0 {
				select {
				case r.StderrChan <- partial.String():
				default:
				}
			}
			return
		}
	}
}

func (r *Runner) waitForExit(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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
}

func (r *Runner) Wait() int {
	exitCode := <-r.ExitChan
	return exitCode
}

func (r *Runner) GetRawStdout() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rawStdout.String()
}

func (r *Runner) GetRawStderr() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rawStderr.String()
}

func (r *Runner) Kill() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		syscall.Kill(-r.cmd.Process.Pid, syscall.SIGKILL)
	}
}

type Subprocess struct {
	cmd      *exec.Cmd
	stdout   bytes.Buffer
	stderr   bytes.Buffer
	exitCode chan int
}

func NewSubprocess(command string) *Subprocess {
	return &Subprocess{
		cmd:      exec.Command("bash", "-c", command),
		exitCode: make(chan int, 1),
	}
}

func (s *Subprocess) Start() {
	s.cmd.Stdout = &s.stdout
	s.cmd.Stderr = &s.stderr
	go func() {
		defer func() { recover() }()
		if err := s.cmd.Run(); err != nil {
			s.exitCode <- s.cmd.ProcessState.ExitCode()
		} else {
			s.exitCode <- 0
		}
		close(s.exitCode)
	}()
}

func (s *Subprocess) Wait() int {
	return <-s.exitCode
}
