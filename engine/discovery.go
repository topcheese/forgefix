package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================================
// PROJECT DISCOVERY
// ============================================================================

func DiscoverProjectRoot(startDir string) (string, error) {
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "go.mod" || name == "pubspec.yaml" {
			return filepath.Join(startDir, name), nil
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
		name := info.Name()
		if name == "go.mod" || name == "pubspec.yaml" {
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
	var cmd string
	var args []string

	switch command.Type {
	case "go_stack":
		cmd = "go"
		args = append(args, "test", "-json", "-timeout", "120s", "./...")
	case "flutter_stack":
		cmd = "flutter"
		args = append(args, "test", "--machine")
	default:
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
