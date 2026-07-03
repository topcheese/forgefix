package engine

import (
	"sync"
)

// PipelineManager manages pipeline lifecycle state.
// Single Responsibility: Track which pipelines are skipped/active.
type PipelineManager struct {
	mu              sync.RWMutex
	pipelines       []PipelineConfig
	skippedPipelines map[string]bool
}

func NewPipelineManager(pipelines []PipelineConfig) *PipelineManager {
	return &PipelineManager{
		pipelines:        pipelines,
		skippedPipelines: make(map[string]bool),
	}
}

func (m *PipelineManager) GetPipelines() []PipelineConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pipelines
}

func (m *PipelineManager) MarkSkipped(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.skippedPipelines == nil {
		m.skippedPipelines = make(map[string]bool)
	}
	m.skippedPipelines[id] = true
}

func (m *PipelineManager) IsSkipped(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skippedPipelines[id]
}

func (m *PipelineManager) GetActivePipelines() []PipelineConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var active []PipelineConfig
	for _, p := range m.pipelines {
		if !m.skippedPipelines[p.ID] {
			active = append(active, p)
		}
	}
	return active
}