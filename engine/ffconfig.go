package engine

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type StatusDef struct {
	Name       string `yaml:"name"`
	Color      string `yaml:"color"`
	Active     bool   `yaml:"active"`
	RepoLabel  string `yaml:"repo_label,omitempty"`
}

type LabelCategory struct {
	Defaults []string `yaml:"defaults"`
}

type WorkflowConfig struct {
	Statuses        []StatusDef               `yaml:"statuses"`
	LabelCategories map[string]LabelCategory  `yaml:"label_categories,omitempty"`
}

func FFWorkflowConfigPath(configDir string) string {
	return filepath.Join(FFDir(configDir), "forgefix.yaml")
}

func DefaultWorkflowConfig() *WorkflowConfig {
	return &WorkflowConfig{
		Statuses: []StatusDef{
			{Name: "backlog", Color: "blue", Active: true, RepoLabel: "status/backlog"},
			{Name: "in-progress", Color: "cyan", Active: true, RepoLabel: "status/in-progress"},
			{Name: "review", Color: "yellow", Active: true, RepoLabel: "status/review"},
			{Name: "ship", Color: "green", Active: true, RepoLabel: "status/ship"},
			{Name: "closed", Color: "gray", Active: false, RepoLabel: "status/closed"},
		},
		LabelCategories: map[string]LabelCategory{
			"status": {
				Defaults: []string{"status/backlog", "status/in-progress", "status/review", "status/ship", "status/closed"},
			},
			"type": {
				Defaults: []string{"type/feature", "type/bug", "type/chore"},
			},
			"version": {
				Defaults: []string{"version/v0.8.0", "version/v0.9.0"},
			},
		},
	}
}

func LoadWorkflowConfig(configDir string) *WorkflowConfig {
	path := FFWorkflowConfigPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultWorkflowConfig()
	}

	var cfg WorkflowConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultWorkflowConfig()
	}

	if len(cfg.Statuses) == 0 {
		return DefaultWorkflowConfig()
	}

	return &cfg
}

func (wc *WorkflowConfig) IsActiveStatus(status string) bool {
	if wc == nil {
		return status != "closed"
	}
	for _, sd := range wc.Statuses {
		if sd.Name == status {
			return sd.Active
		}
	}
	return true
}

func (wc *WorkflowConfig) GetStatusDef(name string) (StatusDef, bool) {
	if wc == nil {
		return StatusDef{}, false
	}
	for _, sd := range wc.Statuses {
		if sd.Name == name {
			return sd, true
		}
	}
	return StatusDef{}, false
}

func (wc *WorkflowConfig) CategoryLabelFor(category, value string) string {
	if wc == nil || wc.LabelCategories == nil || value == "" {
		return ""
	}
	cat, ok := wc.LabelCategories[category]
	if !ok {
		return ""
	}
	valueLower := strings.ToLower(value)
	for _, d := range cat.Defaults {
		dLower := strings.ToLower(d)
		if dLower == valueLower || dLower == category+"/"+valueLower {
			return d
		}
	}
	return ""
}

func (wc *WorkflowConfig) AllCategoryLabelNames() map[string]bool {
	m := make(map[string]bool)
	if wc == nil || wc.LabelCategories == nil {
		return m
	}
	for _, cat := range wc.LabelCategories {
		for _, d := range cat.Defaults {
			m[d] = true
		}
	}
	return m
}

func (wc *WorkflowConfig) IsArchivedStatus(status string) bool {
	return !wc.IsActiveStatus(status)
}
