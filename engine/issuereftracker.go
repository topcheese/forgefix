package engine

import (
	"sync"
)

// IssueRefTracker tracks GitHub issue references for tests.
// Single Responsibility: Map test names to GitHub issue numbers/URLs.
type IssueRefTracker struct {
	mu        sync.RWMutex
	issueRefs map[string]*IssueInfo
}

func NewIssueRefTracker() *IssueRefTracker {
	return &IssueRefTracker{
		issueRefs: make(map[string]*IssueInfo),
	}
}

func (t *IssueRefTracker) SetIssueRef(testName string, ref *IssueInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.issueRefs == nil {
		t.issueRefs = make(map[string]*IssueInfo)
	}
	t.issueRefs[testName] = ref
}

func (t *IssueRefTracker) GetIssueRef(testName string) *IssueInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.issueRefs[testName]
}

func (t *IssueRefTracker) HasIssueRef(testName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.issueRefs[testName]
	return exists
}

func (t *IssueRefTracker) GetAllIssueRefs() map[string]*IssueInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := make(map[string]*IssueInfo, len(t.issueRefs))
	for k, v := range t.issueRefs {
		cp[k] = v
	}
	return cp
}

func (t *IssueRefTracker) DeleteIssueRef(testName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.issueRefs, testName)
}
