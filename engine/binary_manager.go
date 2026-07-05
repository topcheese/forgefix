package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// BinaryManager handles installing and updating the ff binary for both
// project-local (dev) and global usage. Consolidates copy logic that was
// previously duplicated across Bootstrap, EnsureDevBinary, and InstallGlobal.
type BinaryManager struct{}

// NewBinaryManager creates a BinaryManager.
func NewBinaryManager() *BinaryManager {
	return &BinaryManager{}
}

// EnsureDev copies the local ./ff binary to .ff/bin/ff if it is stale or
// missing. Also ensures the .ff/ directory and .ff/bin/ subdirectory exist.
// This is called during bootstrap and sync to keep the development binary
// in sync with the working tree.
func (bm *BinaryManager) EnsureDev(configDir string) error {
	projectRoot := FindProjectRoot(configDir)

	if err := ensureFFDir(projectRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(FFBinDir(projectRoot), 0755); err != nil {
		return fmt.Errorf("creating .ff/bin/: %w", err)
	}

	// Check for binary in configDir first, then fall back to projectRoot
	srcDir := configDir
	src := filepath.Join(srcDir, localBinaryName())
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			if projectRoot != configDir {
				srcDir = projectRoot
				src = filepath.Join(srcDir, localBinaryName())
				srcInfo, err = os.Stat(src)
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("local ff binary not found at %s or %s. Run 'go build -o ff .' in project root (%s)",
							filepath.Join(configDir, localBinaryName()), src, projectRoot)
					}
					return fmt.Errorf("checking local binary: %w", err)
				}
			} else {
				return nil
			}
		} else {
			return fmt.Errorf("checking local binary: %w", err)
		}
	}

	dst := FFBinPath(projectRoot)
	dstInfo, err := os.Stat(dst)
	if err == nil && srcInfo.Size() == dstInfo.Size() {
		return nil
	}

	return copyFile(src, dst, srcInfo.Mode())
}

// InstallGlobal copies the ff binary to the user's global bin directory
// (typically ~/.local/bin/ff) and ensures that directory is on PATH.
// It prefers a local ./ff binary over the running binary so that
// `ff --install` installs the latest local build.
func (bm *BinaryManager) InstallGlobal() (binDir string, warning string, err error) {
	binDir, err = getSystemBinDir()
	if err != nil {
		return "", "", err
	}

	src := bm.preferLocalBinary()
	if src == "" {
		src, err = os.Executable()
		if err != nil {
			return "", "", fmt.Errorf("cannot determine executable path: %w", err)
		}
	}

	if err := bm.copyBinary(src, binDir); err != nil {
		return "", "", err
	}

	if err := ensureInPath(binDir); err != nil {
		warning = fmt.Sprintf("Binary installed to %s but could not update PATH automatically: %v", binDir, err)
		return binDir, warning, nil
	}

	profilePath := DetectShellProfile()
	if profilePath != "" {
		warning = fmt.Sprintf("Restart your shell or run: source %s", profilePath)
	}

	return binDir, warning, nil
}

// copyBinary copies the source binary into binDir as both "ff" and "FF"
// (or "ff.exe"/"FF.exe" on Windows). It skips self-copy (src == dst).
func (bm *BinaryManager) copyBinary(src, binDir string) error {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", src, err)
	}

	names := []string{"ff", "FF"}
	if runtime.GOOS == "windows" {
		names = []string{"ff.exe", "FF.exe"}
	}

	for _, name := range names {
		dst := filepath.Join(binDir, name)
		if src == dst {
			continue
		}
		if err := os.WriteFile(dst, srcData, 0755); err != nil {
			return fmt.Errorf("cannot write %s: %w", dst, err)
		}
	}

	return nil
}

// preferLocalBinary checks for ./ff in the current working directory.
// Returns the path if it exists and is a regular binary distinct from
// the running executable, empty string otherwise.
func (bm *BinaryManager) preferLocalBinary() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	localPath := filepath.Join(wd, "ff")
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() && fi.Mode().IsRegular() {
		running, err := os.Executable()
		if err == nil {
			runningAbs, _ := filepath.Abs(running)
			localAbs, _ := filepath.Abs(localPath)
			if runningAbs == localAbs {
				return ""
			}
		}
		return localPath
	}
	return ""
}

// copyFile copies a file from src to dst, preserving the provided mode.
func copyFile(src, dst string, mode os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying to %s: %w", dst, err)
	}
	return nil
}
