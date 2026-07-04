package engine

// ============================================================================
// DECLARATIVE PIPELINE SCHEMA
// ============================================================================

type PipelineConfig struct {
	ID             string        `yaml:"id"`
	Name           string        `yaml:"name"`
	Description    string        `yaml:"description"`
	PanelColor     string        `yaml:"panel_color"`
	Type           string        `yaml:"type"`
	Command        CommandConfig `yaml:"command"`
	TokenPatterns  TokenPatterns `yaml:"token_patterns"`
	TimeoutSeconds int           `yaml:"timeout_seconds"`
	LedgerFloor    int           `yaml:"ledger_floor"`
}

type LanguageConfig struct {
	RootAnchor     string        `yaml:"root_anchor"`
	TestCommand    string        `yaml:"test_command"`
	TokenPatterns  TokenPatterns `yaml:"token_patterns"`
	PanelColor     string        `yaml:"panel_color,omitempty"`
	TimeoutSeconds int           `yaml:"timeout_seconds,omitempty"`
	LedgerFloor    int           `yaml:"ledger_floor,omitempty"`
}

type LanguageMap map[string]LanguageConfig

type GitHubConfig struct {
	Owner   string `yaml:"owner"`
	Repo    string `yaml:"repo"`
	Token   string `yaml:"token"`
	BaseURL string `yaml:"base_url"`
}

type SyncScheduleConfig struct {
	MaxAgeDays         int `yaml:"max_age_days"`
	RetryIntervalHours int `yaml:"retry_interval_hours"`
}

type Config struct {
	Pipelines            []PipelineConfig    `yaml:"pipelines"`
	Languages            LanguageMap         `yaml:"languages"`
	ExcludeDirs          []string            `yaml:"exclude_dirs"`
	GlobalTimeoutSeconds int                 `yaml:"global_timeout_seconds"`
	FailureDecaySeconds  int                 `yaml:"failure_decay_seconds"`
	AutoIssueManagement  bool                `yaml:"auto_issue_management,omitempty"`
	GitPassthrough       *bool               `yaml:"git_passthrough,omitempty"`
	GitHub               *GitHubConfig       `yaml:"github,omitempty"`
	SyncSchedule         *SyncScheduleConfig `yaml:"sync_schedule,omitempty"`
}

type CommandConfig struct {
	Type  string   `yaml:"type"`
	Args  []string `yaml:"args"`
	Paths []string `yaml:"paths"`
}

type TokenPatterns struct {
	TokenRun  string `yaml:"token_run"`
	TokenPass string `yaml:"token_pass"`
	TokenFail string `yaml:"token_fail"`
}

// ============================================================================
// CONFIGURATION LOADING
// ============================================================================

type LoadedConfig struct {
	Config    *Config
	ConfigDir string
}
