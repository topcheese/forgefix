package engine

import (
	"sync"
	"time"
)

// ErrorTracker tracks system errors and error logs.
// Single Responsibility: Track and provide access to system errors and execution logs.
type ErrorTracker struct {
	mu        sync.RWMutex
	systemErr []string
	errorLogs []ErrorLog
}

func NewErrorTracker() *ErrorTracker {
	return &ErrorTracker{
		systemErr: make([]string, 0),
		errorLogs: make([]ErrorLog, 0),
	}
}

func (t *ErrorTracker) AddSystemError(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.systemErr = append(t.systemErr, msg)
}

func (t *ErrorTracker) GetSystemErrors() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	clone := make([]string, len(t.systemErr))
	copy(clone, t.systemErr)
	return clone
}

func (t *ErrorTracker) AddErrorLog(exitCode int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errorLogs = append(t.errorLogs, ErrorLog{
		Timestamp: time.Now(),
		Message:   "Pipeline execution failed",
		ExitCode:  exitCode,
	})
}

func (t *ErrorTracker) GetErrorLogs() []ErrorLog {
	t.mu.RLock()
	defer t.mu.RUnlock()
	clone := make([]ErrorLog, len(t.errorLogs))
	copy(clone, t.errorLogs)
	return clone
}

func (t *ErrorTracker) AddErrorCode(code int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Could track error codes here if needed
}
