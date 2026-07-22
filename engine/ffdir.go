package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const ffDirName = ".ff"
const binDirName = "bin"

const (
	legacyLedgerFile  = ".forgefix_ledger.json"
	ffLedgerFile      = "forgefix_ledger.json"
	legacyHistoryFile = ".forgefix_history.log"
	ffHistoryFile     = ".forgefix_history.log"
)

func localBinaryName() string {
	name := "ff"
	if runtime.GOOS == "windows" {
		name = "ff.exe"
	}
	return name
}

func FFDir(configDir string) string {
	if configDir == "" {
		if wd, err := os.Getwd(); err == nil {
			configDir = wd
		} else {
			configDir = "."
		}
	}
	return filepath.Join(configDir, ffDirName)
}

func ensureFFDir(configDir string) error {
	ffDir := FFDir(configDir)
	if _, err := os.Stat(ffDir); os.IsNotExist(err) {
		if err := os.MkdirAll(ffDir, 0755); err != nil {
			return fmt.Errorf("creating .ff/ directory: %w", err)
		}
	}
	return nil
}

func migrateFileToFF(configDir, legacyName, ffName string) error {
	legacyPath := filepath.Join(configDir, legacyName)
	ffPath := filepath.Join(FFDir(configDir), ffName)

	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(ffPath); err == nil {
		if err := os.Remove(legacyPath); err != nil {
			return fmt.Errorf("removing stale legacy %s: %w", legacyName, err)
		}
		return nil
	}
	if err := ensureFFDir(configDir); err != nil {
		return err
	}
	if err := os.Rename(legacyPath, ffPath); err != nil {
		return fmt.Errorf("migrating %s to .ff/: %w", legacyName, err)
	}
	return nil
}

func FFLedgerPath(configDir string) string {
	return filepath.Join(FFDir(configDir), ffLedgerFile)
}

func FFHistoryLogPath(configDir string) string {
	return filepath.Join(FFDir(configDir), ffHistoryFile)
}

func FFBinDir(configDir string) string {
	return filepath.Join(FFDir(configDir), binDirName)
}

func FFBinPath(configDir string) string {
	return filepath.Join(FFBinDir(configDir), localBinaryName())
}

func localBinaryPath(configDir string) string {
	return filepath.Join(configDir, localBinaryName())
}

func FindProjectRoot(startDir string) string {
	dir := startDir

	// Walk up checking for {dirbasename}_ff.yaml at each level. Return the
	// nearest ancestor that has one matching its own directory name — this
	// is the authoritative project root.  We match by directory name so that
	// a stray james_ff.yaml in $HOME is never picked up when working in a
	// project beneath it.
	for {
		folderName := filepath.Base(dir)
		targetConfig := fmt.Sprintf("%s_ff.yaml", folderName)
		if _, err := os.Stat(filepath.Join(dir, targetConfig)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fall back to heuristic markers. Refuse to match on .ff/ alone (it can
	// be a stale artifact from a subdirectory).
	dir = startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "templates")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, localBinaryName())); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ffDirName)); err == nil {
			if hasYaml, _ := hasConfigFile(dir); hasYaml {
				// Confirm the yaml matches this directory name
				folderName := filepath.Base(dir)
				targetConfig := fmt.Sprintf("%s_ff.yaml", folderName)
				if hasYaml, _ = hasConfigFileNamed(dir, targetConfig); hasYaml {
					return dir
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return startDir
}

func hasConfigFile(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_ff.yaml") {
			return true, nil
		}
	}
	return false, nil
}

func hasConfigFileNamed(dir, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, name))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func findNamedConfig(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("config %s not found in %s", name, dir)
}

func FindMatchingConfig(startDir string) (string, error) {
	dir := startDir
	for {
		folderName := filepath.Base(dir)
		target := fmt.Sprintf("%s_ff.yaml", folderName)
		found, err := findNamedConfig(dir, target)
		if err == nil {
			return found, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no matching _ff.yaml found from %s", startDir)
}

func EnsureDevBinary(configDir string) error {
	return NewBinaryManager().EnsureDev(configDir)
}

func MigrateToFF(configDir string) error {
	projectRoot := FindProjectRoot(configDir)
	// Pre-release: just ensure the .ff/ directory exists.
	// Legacy file migration and binary management are deferred
	// until after the first stable release.
	return ensureFFDir(projectRoot)
}

func Bootstrap(configDir string) error {
	return NewBinaryManager().EnsureDev(configDir)
}
