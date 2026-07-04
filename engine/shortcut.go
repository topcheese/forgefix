package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func getSystemBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	var binDir string
	if runtime.GOOS == "windows" {
		binDir = filepath.Join(home, "AppData", "Local", "bin")
	} else {
		binDir = filepath.Join(home, ".local", "bin")
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", binDir, err)
	}

	return binDir, nil
}

func copyBinary(src, binDir string) error {
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

func ensureInPath(binDir string) error {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		clean, err := filepath.EvalSymlinks(dir)
		if err == nil && clean == binDir {
			return nil
		}
		if dir == binDir {
			return nil
		}
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf(`[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path","User") + ";%s", "User")`, binDir))
		return cmd.Run()
	}

	profilePath := DetectShellProfile()
	if profilePath == "" {
		return fmt.Errorf("cannot detect shell profile")
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", profilePath, err)
	}
	if strings.Contains(string(data), binDir) {
		return nil
	}

	exportLine := fmt.Sprintf("\nexport PATH=\"%s:$PATH\"\n", binDir)

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", profilePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(exportLine); err != nil {
		return fmt.Errorf("cannot write to %s: %w", profilePath, err)
	}

	return nil
}

func InstallGlobal() (binDir string, warning string, err error) {
	binDir, err = getSystemBinDir()
	if err != nil {
		return "", "", err
	}

	// Prefer a local ./ff in the current directory over the running binary.
	// This ensures `ff --install` (from PATH) finds and installs the latest
	// local build rather than being a no-op (copying the global binary to itself).
	src := preferLocalBinary()
	if src == "" {
		src, err = os.Executable()
		if err != nil {
			return "", "", fmt.Errorf("cannot determine executable path: %w", err)
		}
	}

	if err := copyBinary(src, binDir); err != nil {
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

// preferLocalBinary checks for ./ff in the current working directory.
// Returns the path if it exists and looks like a binary, empty string otherwise.
func preferLocalBinary() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	localPath := filepath.Join(wd, "ff")
	if fi, err := os.Stat(localPath); err == nil && !fi.IsDir() && fi.Mode().IsRegular() {
		// Make sure it's not the same as the running binary
		running, err := os.Executable()
		if err == nil {
			runningAbs, _ := filepath.Abs(running)
			localAbs, _ := filepath.Abs(localPath)
			if runningAbs == localAbs {
				return "" // same file, no benefit
			}
		}
		return localPath
	}
	return ""
}

func DetectShellProfile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}

	shell := os.Getenv("SHELL")

	switch {
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.Contains(shell, "bash"):
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".bashrc")
	}
}
