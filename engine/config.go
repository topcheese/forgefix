package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// DASHBOARD - Use DashboardFacade from dashboard_facade.go
// ============================================================================
//
// The Dashboard implementation has been decomposed into separate services
// following the Single Responsibility Principle:
//
//   - TestTrackerService (testtracker.go): Test state management (run/pass/fail)
//   - PipelineManager (pipelinemanager.go): Pipeline lifecycle and skip tracking
//   - LedgerEngine (ledger.go): Metrics aggregation, persistence, and formatting
//   - ErrorTracker (errortracker.go): System errors and execution logs
//   - DashboardState (dashboardstate.go): Runtime state (bomb, timeout, config dir)
//   - IssueRefTracker (issuereftracker.go): GitHub issue references
//
// DashboardFacade (dashboard_facade.go) composes these services and provides
// backward-compatible API for existing consumers.
//
// New code should prefer using the individual services directly.

// Dashboard is an alias for DashboardFacade for backward compatibility
type Dashboard = DashboardFacade

func LoadPipelineConfig(targetPath string) (*LoadedConfig, error) {
	wd := targetPath
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %v", err)
		}
	}
	folderName := filepath.Base(wd)
	target := fmt.Sprintf("%s_ff.yaml", folderName)

	// Check for project-specific config in the target directory
	if _, err := os.Stat(filepath.Join(wd, target)); err == nil {
		return loadConfigFromPath(filepath.Join(wd, target))
	}

	// Fall back to discovering any _ff.yaml in the target directory
	if found, err := FindAnyConfig(wd); err == nil {
		return loadConfigFromPath(found)
	}

	// Fall back to scanning subdirectories one level deep for _ff.yaml configs
	entries, _ := os.ReadDir(wd)
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(wd, entry.Name())
			if found, err := FindAnyConfig(subPath); err == nil {
				return loadConfigFromPath(found)
			}
		}
	}

	return nil, fmt.Errorf("%s not found from %s", target, wd)
}

// knownProjectAnchors lists well-known project metadata filenames used to
// discover project roots during initialization. This is a data-only list,
// not a language schema — no commands, token patterns, or language names
// are associated with these filenames in the engine source.
var knownProjectAnchors = []string{
	"go.mod",
	"pubspec.yaml",
	"package.json",
	"Cargo.toml",
	"Gemfile",
	"setup.py",
	"pyproject.toml",
	"pom.xml",
	"build.gradle",
	"mix.exs",
	"composer.json",
	"CMakeLists.txt",
	"Makefile",
	"Rakefile",
	"cabal.project",
	"Package.swift",
	"project.json",
	"deno.json",
	"bun.lock",
	"yarn.lock",
	"package-lock.json",
}

// defaultTestCommands maps anchor files to their default test commands and token patterns.
// Users can override these in the generated ff.yaml — the engine treats test_command as a raw shell string.
var defaultTestCommands = map[string]struct {
	Command       string
	TokenPatterns TokenPatterns
}{
	"go.mod": {
		Command: "go test -json ./...",
		TokenPatterns: TokenPatterns{TokenRun: `"Action":"run"`, TokenPass: `"Action":"pass"`, TokenFail: `"Action":"fail"`},
	},
	"pubspec.yaml": {
		Command: "flutter test --machine",
		TokenPatterns: TokenPatterns{TokenRun: `"type":"testStart"`, TokenPass: `"result":"success"`, TokenFail: `"result":"failure"`},
	},
	"package.json": {
		Command: "npm test -- --json 2>/dev/null || npm test",
		TokenPatterns: TokenPatterns{TokenRun: `"type":"test"`, TokenPass: `"pass":true`, TokenFail: `"fail":true`},
	},
	"Cargo.toml": {
		Command: "cargo test -- --format=json 2>/dev/null || cargo test",
		TokenPatterns: TokenPatterns{TokenRun: `"event":"started"`, TokenPass: `"event":"ok"`, TokenFail: `"event":"failed"`},
	},
	"Gemfile": {
		Command: "bundle exec rspec --format json 2>/dev/null || bundle exec rspec",
		TokenPatterns: TokenPatterns{TokenRun: `"status":"pending"`, TokenPass: `"status":"passed"`, TokenFail: `"status":"failed"`},
	},
	"setup.py": {
		Command: "python -m pytest --json-report 2>/dev/null || python -m pytest",
		TokenPatterns: TokenPatterns{TokenRun: `"when":"call"`, TokenPass: `"outcome":"passed"`, TokenFail: `"outcome":"failed"`},
	},
	"pyproject.toml": {
		Command: "python -m pytest --json-report 2>/dev/null || python -m pytest",
		TokenPatterns: TokenPatterns{TokenRun: `"when":"call"`, TokenPass: `"outcome":"passed"`, TokenFail: `"outcome":"failed"`},
	},
	"pom.xml": {
		Command: "mvn test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Running ", TokenPass: "Tests run:", TokenFail: "FAILURE"},
	},
	"build.gradle": {
		Command: "./gradlew test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "> Task :test", TokenPass: "BUILD SUCCESSFUL", TokenFail: "BUILD FAILED"},
	},
	"mix.exs": {
		Command: "mix test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Running ", TokenPass: "PASS", TokenFail: "FAIL"},
	},
	"composer.json": {
		Command: "composer test 2>&1 || vendor/bin/phpunit 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Running ", TokenPass: "OK", TokenFail: "FAILURES"},
	},
	"CMakeLists.txt": {
		Command: "ctest --output-on-failure 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Start ", TokenPass: "Passed", TokenFail: "Failed"},
	},
	"Makefile": {
		Command: "make test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Running ", TokenPass: "PASS", TokenFail: "FAIL"},
	},
	"Rakefile": {
		Command: "rake test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Running ", TokenPass: "PASS", TokenFail: "FAIL"},
	},
	"cabal.project": {
		Command: "cabal test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Test suite ", TokenPass: "PASS", TokenFail: "FAIL"},
	},
	"Package.swift": {
		Command: "swift test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: "Test Case ", TokenPass: "passed", TokenFail: "failed"},
	},
	"project.json": {
		Command: "dotnet test --logger:json 2>/dev/null || dotnet test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: `"DisplayName"`, TokenPass: `"Outcome":"Passed"`, TokenFail: `"Outcome":"Failed"`},
	},
	"deno.json": {
		Command: "deno test --reporter=json 2>/dev/null || deno test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: `"type":"test"`, TokenPass: `"result":"passed"`, TokenFail: `"result":"failed"`},
	},
	"bun.lock": {
		Command: "bun test --reporter=json 2>/dev/null || bun test 2>&1",
		TokenPatterns: TokenPatterns{TokenRun: `"type":"test"`, TokenPass: `"passed":true`, TokenFail: `"failed":true`},
	},
}

const genericConfigTemplate = `# =============================================================================
# ForgeFix Pipeline Configuration (Agnostic Template)
# =============================================================================
global_timeout_seconds: 60
failure_decay_seconds: 15
# auto_issue_management: true  # Uncomment to enable automatic issue creation/closure in --ai mode

languages:
  your_language_stack:
    root_anchor: "your_project_anchor_file"
    test_command: "your_test_command_here"
    token_patterns:
      token_run:   "run"
      token_pass:  "pass"
      token_fail:  "fail"

pipelines:
  - id: "your-pipeline-id"
    name: "[ USER PIPELINE ]"
    type: "your_language_stack"
    panel_color: "blue"
    timeout_seconds: 30
    ledger_floor: 1

# Add project-specific exclusions here; VCS and build noise are handled via auto-discovery
exclude_dirs: []

github:
  owner: ""
  repo:  ""
  token: ""
  # For Repo, use: "https://your-repo.com/api/v1"
  # For GitHub, use: "https://api.github.com"
  base_url: "https://api.github.com"

sync_schedule:
  max_age_days: 7
  retry_interval_hours: 1
`

func findInSubtree(root, target string, maxDepth int) (string, error) {
	found := ""
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && d.Name() == target {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func FindWorkspaceConfig(targetSCN string) (string, error) {
	targetName := fmt.Sprintf("%s_ff.yaml", targetSCN)
	startDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %v", err)
	}
	currentDir := startDir
	for {
		found, err := findInSubtree(currentDir, targetName, 3)
		if err != nil {
			return "", err
		}
		if found != "" {
			return found, nil
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			return "", fmt.Errorf("workspace config %s not found from %s", targetName, startDir)
		}
		currentDir = parent
	}
}

func FindAnyConfig(startDir string) (string, error) {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_ff.yaml") {
			return filepath.Join(startDir, entry.Name()), nil
		}
	}
	return "", fmt.Errorf("no _ff.yaml config found in %s", startDir)
}

func loadConfigFromPath(path string) (*LoadedConfig, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %v", err)
	}
	configDir := filepath.Dir(absPath)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	if len(config.Pipelines) == 0 {
		return nil, fmt.Errorf("empty workspace: no pipelines defined in configuration")
	}

	for i, p := range config.Pipelines {
		lang, ok := config.Languages[p.Type]
		if !ok {
			return nil, fmt.Errorf("configuration error: pipeline '%s' specifies unknown type '%s' (missing from languages configuration block)", p.ID, p.Type)
		}
		config.Pipelines[i].Command.Type = p.Type
		config.Pipelines[i].TokenPatterns = lang.TokenPatterns
	}

	return &LoadedConfig{
		Config:    &config,
		ConfigDir: configDir,
	}, nil
}

func BuildInitConfig(wd string) ([]byte, error) {
	excludeDirs := discoverExcludeDirs(wd)

	cfg := &Config{
		GlobalTimeoutSeconds: 120,
		FailureDecaySeconds:  30,
		ExcludeDirs:          excludeDirs,
		Languages:            make(LanguageMap),
		GitHub: &GitHubConfig{
			Owner:   "",
			Repo:    "",
			Token:   "",
			BaseURL: "https://api.github.com",
		},
	}

	seen := make(map[string]bool)
	filepath.WalkDir(wd, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		for _, anchor := range knownProjectAnchors {
			if d.Name() != anchor {
				continue
			}
			dir := filepath.Dir(path)
			if seen[dir] {
				return nil
			}
			seen[dir] = true

			dirName := filepath.Base(dir)
			langKey := strings.ReplaceAll(anchor, ".", "_")

			defaults := defaultTestCommands[anchor]
			tokenPatterns := defaults.TokenPatterns
			if tokenPatterns.TokenRun == "" {
				tokenPatterns = TokenPatterns{TokenRun: "RUN", TokenPass: "PASS", TokenFail: "FAIL"}
			}

			langCfg := LanguageConfig{
				RootAnchor:     anchor,
				TestCommand:    defaults.Command,
				TokenPatterns:  tokenPatterns,
				PanelColor:     "blue",
				TimeoutSeconds: 300,
				LedgerFloor:    0,
			}
			cfg.Languages[langKey] = langCfg

			pipe := PipelineConfig{
				ID:             dirName,
				Name:           fmt.Sprintf("[%s]", dirName),
				Type:           langKey,
				Command:        CommandConfig{Type: langKey},
				TokenPatterns:  langCfg.TokenPatterns,
				PanelColor:     langCfg.PanelColor,
				TimeoutSeconds: langCfg.TimeoutSeconds,
				LedgerFloor:    langCfg.LedgerFloor,
			}
			cfg.Pipelines = append(cfg.Pipelines, pipe)
		}
		return nil
	})

	if len(cfg.Pipelines) == 0 {
		return []byte(genericConfigTemplate), nil
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %v", err)
	}
	yamlStr := string(data)
	failureDecayLine := "failure_decay_seconds:"
	if idx := strings.Index(yamlStr, failureDecayLine); idx != -1 {
		endIdx := idx + len(failureDecayLine)
		for endIdx < len(yamlStr) && yamlStr[endIdx] != '\n' {
			endIdx++
		}
		if endIdx < len(yamlStr) {
			endIdx++
		}
		comment := "# auto_issue_management: true  # Uncomment to enable automatic issue creation/closure in --ai mode\n"
		yamlStr = yamlStr[:endIdx] + comment + yamlStr[endIdx:]
	}
	excludeDirsLine := "exclude_dirs:"
	if idx := strings.Index(yamlStr, excludeDirsLine); idx != -1 {
		comment := "# Add project-specific exclusions here; VCS and build noise are handled via auto-discovery\n"
		yamlStr = yamlStr[:idx] + comment + yamlStr[idx:]
	}

	return []byte(yamlStr), nil
}

func discoverExcludeDirs(wd string) []string {
	var excludeDirs []string
	seen := make(map[string]bool)

	ignoreFiles := []string{".gitignore", ".hgignore", ".svnignore"}
	for _, ignoreFile := range ignoreFiles {
		path := filepath.Join(wd, ignoreFile)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.Trim(line, "/")
			if line == "" {
				continue
			}
			if strings.Contains(line, "*") || strings.Contains(line, "?") || strings.Contains(line, "[") {
				continue
			}
			if !seen[line] {
				seen[line] = true
				excludeDirs = append(excludeDirs, line)
			}
		}
	}

	if _, err := os.Stat(filepath.Join(wd, ".svn")); err == nil {
		if !seen[".svn"] {
			excludeDirs = append(excludeDirs, ".svn")
		}
	}

	return excludeDirs
}

func InitConfig(wd string) (string, error) {
	folderName := filepath.Base(wd)
	target := fmt.Sprintf("%s_ff.yaml", folderName)
	targetPath := filepath.Join(wd, target)

	if _, err := os.Stat(targetPath); err == nil {
		return target, fmt.Errorf("%s already exists", target)
	}

	data, err := BuildInitConfig(wd)
	if err != nil {
		return "", fmt.Errorf("error building config: %v", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return "", fmt.Errorf("error writing config: %v", err)
	}

	return target, nil
}
