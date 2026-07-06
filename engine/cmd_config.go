package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (d *CommandDispatcher) handleConfig(args []string) (CommandResult, error) {
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintf(d.Stderr, "usage: ff config validate\n")
		return CommandResult{ExitCode: 1}, nil
	}

	loaded, err := LoadPipelineConfig(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: loading config: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	cfg := loaded.Config
	configFile := findConfigFile(d.ConfigDir)
	var errors []string

	if cfg.GlobalTimeoutSeconds <= 0 {
		errors = append(errors, "global_timeout_seconds must be positive")
	}
	if cfg.FailureDecaySeconds <= 0 {
		errors = append(errors, "failure_decay_seconds must be positive")
	}

	if cfg.GitHub == nil {
		errors = append(errors, "missing required section: github")
	} else {
		if cfg.GitHub.Owner == "" {
			errors = append(errors, "github.owner is required")
		}
		if cfg.GitHub.Repo == "" {
			errors = append(errors, "github.repo is required")
		}
		if cfg.GitHub.Token == "" {
			errors = append(errors, "github.token is required")
		}
		if cfg.GitHub.BaseURL != "" &&
			!strings.HasPrefix(cfg.GitHub.BaseURL, "http://") &&
			!strings.HasPrefix(cfg.GitHub.BaseURL, "https://") {
			errors = append(errors, "github.base_url must start with http:// or https://")
		}
	}

	if len(cfg.Pipelines) == 0 {
		errors = append(errors, "at least one pipeline is required")
	}
	for i, p := range cfg.Pipelines {
		if p.ID == "" {
			errors = append(errors, fmt.Sprintf("pipelines[%d].id is required", i))
		}
		if p.Name == "" {
			errors = append(errors, fmt.Sprintf("pipelines[%d].name is required", i))
		}
		if p.Type == "" {
			errors = append(errors, fmt.Sprintf("pipelines[%d].type is required", i))
		}
	}

	if len(cfg.Languages) == 0 {
		errors = append(errors, "at least one language config is required")
	}
	for name, lang := range cfg.Languages {
		if lang.RootAnchor == "" {
			errors = append(errors, fmt.Sprintf("languages.%s.root_anchor is required", name))
		}
		if lang.TestCommand == "" {
			errors = append(errors, fmt.Sprintf("languages.%s.test_command is required", name))
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintf(d.Stderr, "  - %s\n", e)
		}
		if configFile != "" {
			fmt.Fprintf(d.Stderr, "\nfile: %s\n", configFile)
		}
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("%d validation error(s)", len(errors))}, nil
	}

	fmt.Fprintln(d.Stdout, "config is valid")
	return CommandResult{ExitCode: 0, Message: "config is valid"}, nil
}

func findConfigFile(configDir string) string {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_ff.yaml") {
			return filepath.Join(configDir, e.Name())
		}
	}
	return ""
}
