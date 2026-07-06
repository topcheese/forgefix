package engine

import (
	"sync"
	"sync/atomic"
)

// DashboardState manages the dashboard's runtime state.
// Single Responsibility: Track bomb state, pipeline active flag, timeout, dirty flag, config dir, failure decay.
type DashboardState struct {
	mu                   sync.RWMutex
	PipelineActive       bool
	Bomb                 BombState
	BombFrame            int
	TimeoutFired         bool
	TestCommandCompleted bool
	ConfigDir            string
	FailureDecaySecs     int
	stopCh               chan struct{}
	stopOnce             sync.Once
	dirty                atomic.Int32
}

func NewDashboardState(pipelines []PipelineConfig) *DashboardState {
	return &DashboardState{
		PipelineActive:       true,
		Bomb:                 BombIdle,
		BombFrame:            0,
		TimeoutFired:         false,
		TestCommandCompleted: false,
		FailureDecaySecs:     15,
		stopCh:               make(chan struct{}),
	}
}

func (s *DashboardState) MarkDirty() {
	s.dirty.Store(1)
}

func (s *DashboardState) IsDirty() bool {
	return s.dirty.Load() == 1
}

func (s *DashboardState) ClearDirty() {
	s.dirty.Store(0)
}

func (s *DashboardState) StopCh() <-chan struct{} {
	return s.stopCh
}

func (s *DashboardState) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *DashboardState) GetFailureDecaySeconds() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.FailureDecaySecs > 0 {
		return s.FailureDecaySecs
	}
	return 15
}

func (s *DashboardState) SetPipelineActive(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PipelineActive = active
}

func (s *DashboardState) IsPipelineActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PipelineActive
}

func (s *DashboardState) SetBomb(state BombState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Bomb = state
}

func (s *DashboardState) GetBomb() BombState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Bomb
}

func (s *DashboardState) IncrementBombFrame() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BombFrame++
}

func (s *DashboardState) GetBombFrame() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.BombFrame
}

func (s *DashboardState) SetTimeoutFired(fired bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TimeoutFired = fired
}

func (s *DashboardState) IsTimeoutFired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TimeoutFired
}

func (s *DashboardState) SetTestCommandCompleted(completed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TestCommandCompleted = completed
}

func (s *DashboardState) IsTestCommandCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TestCommandCompleted
}

func (s *DashboardState) SetConfigDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConfigDir = dir
}

func (s *DashboardState) GetConfigDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConfigDir
}

func (s *DashboardState) SetFailureDecaySecs(secs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailureDecaySecs = secs
}
