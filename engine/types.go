package engine

import (
	"sync"
	"time"
)

// ============================================================================
// GENERIC TEST EVENT STRUCTURES
// ============================================================================

type GenericTestEvent struct {
	RawJSON       map[string]interface{}
	MatchedToken  string
	TokenType     string
	TestID        string
	TestName      string
	Elapsed       int
	ErrorTrace    string
	FilePath      string
	FailureLine   int
	FailureColumn int
}

// ============================================================================
// TEST STATE TRACKING
// ============================================================================

type TestState int

const (
	StatePending TestState = iota
	StateRunning
	StateFiring
	StateCompleted
	StateDud
	StatePopped
	StateSkipped
)

type TestInfo struct {
	ID            string
	TestID        string
	Name          string
	TestName      string
	State         TestState
	Started       time.Time
	Elapsed       int
	ErrorTrace    string
	FilePath      string
	FailureLine   int
	FailureColumn int
}

type TestTracker struct {
	mu           sync.RWMutex
	ActiveTests  map[string]*TestInfo
	Completed    map[string]*TestInfo
	CompletedIDs map[string]bool
	History      []string
	Ran          int
	Passed       int
}

type TestResult struct {
	Name    string
	State   TestState
	Elapsed int
	Success bool
}

// ============================================================================
// DASHBOARD SUPPORT
// ============================================================================

type IssueInfo struct {
	Number  int
	URL     string
	Existed bool
}

type ErrorLog struct {
	Timestamp time.Time
	Message   string
	ExitCode  int
}
