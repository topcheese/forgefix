package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// PROJECT DISCOVERY
// ============================================================================

func isKnownAnchor(name string) bool {
	for _, a := range knownProjectAnchors {
		if a == name {
			return true
		}
	}
	return false
}

func DiscoverProjectRoot(startDir string) (string, error) {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isKnownAnchor(entry.Name()) {
			return filepath.Join(startDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no project root found in %s", startDir)
}

func FindProjectRoots(startDir string) ([]string, error) {
	var roots []string
	err := filepath.Walk(startDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if isKnownAnchor(info.Name()) {
			dir := filepath.Dir(path)
			if !contains(roots, dir) {
				roots = append(roots, dir)
			}
		}
		return nil
	})
	return roots, err
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ============================================================================
// COMMAND EXECUTION
// ============================================================================

func ExecuteCommand(command CommandConfig, paths []string) (string, error) {
	// Defense-in-depth: reject any command configuration that contains unambiguous
	// shell-injection indicators before it reaches exec.Command("bash", "-c", ...).
	// This guards against configs built or modified outside loadConfigFromPath
	// (e.g. the -run flag path) as well as direct callers.
	if err := validateCommandConfig(command.Type, command); err != nil {
		return "", err
	}

	// Clear Go's internal test cache to prevent stale buffered logs
	if strings.HasPrefix(command.Type, "go_") {
		exec.Command("go", "clean", "-testcache").Run()
	}

	var cmd string
	var args []string

	switch {
	case strings.HasPrefix(command.Type, "go_"):
		cmd = "go"
		args = append(args, "test", "-json", "-timeout", "120s")
		if len(command.Args) > 0 {
			args = append(args, command.Args...)
		} else {
			args = append(args, "./...")
		}
	case strings.HasPrefix(command.Type, "flutter_") || strings.HasPrefix(command.Type, "pubspec_"):
		cmd = "flutter"
		args = append(args, "test", "--machine")
	default:
		if len(command.Args) > 0 {
			return strings.Join(command.Args, " "), nil
		}
		return "", fmt.Errorf("unsupported command type: %s", command.Type)
	}

	if len(paths) > 0 {
		args = append(args, paths...)
	}

	return fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), nil
}

// ============================================================================
// TOKEN PATTERN MATCHING
// ============================================================================

func MatchTokenPatterns(rawLine string, patterns TokenPatterns) (string, string) {
	if patterns.TokenRun != "" {
		if re, err := regexp.Compile(patterns.TokenRun); err == nil {
			if re.MatchString(rawLine) {
				return patterns.TokenRun, "run"
			}
		} else if strings.Contains(rawLine, patterns.TokenRun) {
			return patterns.TokenRun, "run"
		}
	}
	if patterns.TokenPass != "" {
		if re, err := regexp.Compile(patterns.TokenPass); err == nil {
			if re.MatchString(rawLine) {
				return patterns.TokenPass, "pass"
			}
		} else if strings.Contains(rawLine, patterns.TokenPass) {
			return patterns.TokenPass, "pass"
		}
	}
	if patterns.TokenFail != "" {
		if re, err := regexp.Compile(patterns.TokenFail); err == nil {
			if re.MatchString(rawLine) {
				return patterns.TokenFail, "fail"
			}
		} else if strings.Contains(rawLine, patterns.TokenFail) {
			return patterns.TokenFail, "fail"
		}
	}
	return "", ""
}

// ============================================================================
// PATH RESOLUTION
// ============================================================================

func ResolveProjectPath(startDir string, anchor string) (string, error) {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.Name() == anchor {
			return filepath.Join(startDir, anchor), nil
		}
	}
	return "", fmt.Errorf("anchor not found: %s", anchor)
}
