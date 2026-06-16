package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const ffDirName = ".ff"
const binDirName = "bin"

const (
	legacyLedgerFile   = ".forgefix_ledger.json"
	ffLedgerFile       = "forgefix_ledger.json"
	legacyHistoryFile  = ".forgefix_history.log"
	ffHistoryFile      = ".forgefix_history.log"
)

func localBinaryName() string {
	name := "ff"
	if runtime.GOOS == "windows" {
		name = "ff.exe"
	}
	return name
}

func FFDir(configDir string) string {
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

	// Walk up checking for _ff.yaml at each level. Return the nearest
	// ancestor that has one — this is the authoritative project root.
	// We check every level because _ff.yaml is definitive; other heuristics
	// (templates/, ff binary, .ff/) can produce false positives from stale
	// subdirectory artifacts.
	for {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_ff.yaml") {
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

	// If no _ff.yaml found, try heuristic markers. Refuse to match on .ff/
	// alone (it can be a stale artifact from a subdirectory).
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
				return dir
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

func EnsureDevBinary(configDir string) error {
	projectRoot := FindProjectRoot(configDir)
	src := filepath.Join(projectRoot, localBinaryName())
	dst := FFBinPath(projectRoot)

	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			if projectRoot != configDir {
				return fmt.Errorf("local ff binary not found at %s. Run 'go build -o ff .' in project root (%s)", src, projectRoot)
			}
			return nil
		}
		return fmt.Errorf("checking local binary: %w", err)
	}

	dstInfo, err := os.Stat(dst)
	if err == nil && srcInfo.Size() == dstInfo.Size() {
		return nil
	}

	if err := os.MkdirAll(FFBinDir(projectRoot), 0755); err != nil {
		return fmt.Errorf("creating .ff/bin/: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening local binary: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	return nil
}

func MigrateToFF(configDir string) error {
	projectRoot := FindProjectRoot(configDir)
	if err := migrateFileToFF(projectRoot, legacyLedgerFile, ffLedgerFile); err != nil {
		return err
	}
	if err := migrateFileToFF(projectRoot, legacyHistoryFile, ffHistoryFile); err != nil {
		return err
	}
	return EnsureDevBinary(configDir)
}

func Bootstrap(configDir string) error {
	projectRoot := FindProjectRoot(configDir)

	if err := ensureFFDir(projectRoot); err != nil {
		return err
	}

	if err := os.MkdirAll(FFBinDir(projectRoot), 0755); err != nil {
		return fmt.Errorf("creating .ff/bin/: %w", err)
	}

	src := filepath.Join(projectRoot, localBinaryName())
	dst := FFBinPath(projectRoot)

	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			if projectRoot != configDir {
				return fmt.Errorf("local ff binary not found at %s. Run 'go build -o ff .' in project root (%s)", src, projectRoot)
			}
			return nil
		}
		return fmt.Errorf("checking local binary: %w", err)
	}

	dstInfo, err := os.Stat(dst)
	if err == nil && srcInfo.Size() == dstInfo.Size() {
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening local binary: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	return nil
}


