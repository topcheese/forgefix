package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetSystemBinDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := getSystemBinDir()
	if err != nil {
		t.Fatalf("getSystemBinDir failed: %v", err)
	}

	var want string
	if runtime.GOOS == "windows" {
		want = filepath.Join(home, "AppData", "Local", "bin")
	} else {
		want = filepath.Join(home, ".local", "bin")
	}
	if dir != want {
		t.Errorf("expected %s, got %s", want, dir)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected directory to exist")
	}
}

func TestGetSystemBinDirCreatesParent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := getSystemBinDir()
	if err != nil {
		t.Fatalf("getSystemBinDir failed: %v", err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected directory %s to exist", dir)
	}
}

func TestCopyBinary(t *testing.T) {
	binDir := t.TempDir()

	src := filepath.Join(binDir, "forgefix")
	if runtime.GOOS == "windows" {
		src += ".exe"
	}
	if err := os.WriteFile(src, []byte("fake binary content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := copyBinary(src, binDir); err != nil {
		t.Fatalf("copyBinary failed: %v", err)
	}

	names := []string{"ff", "FF"}
	if runtime.GOOS == "windows" {
		names = []string{"ff.exe", "FF.exe"}
	}
	for _, name := range names {
		p := filepath.Join(binDir, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", p)
		}
	}
}

func TestCopyBinarySkipsSelf(t *testing.T) {
	binDir := t.TempDir()

	src := filepath.Join(binDir, "ff")
	if err := os.WriteFile(src, []byte("content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := copyBinary(src, binDir); err != nil {
		t.Fatalf("copyBinary failed: %v", err)
	}
}

func TestEnsureInPathAlreadyPresent(t *testing.T) {
	dir := "/usr/local/bin"
	t.Setenv("PATH", dir)
	if err := ensureInPath(dir); err != nil {
		t.Fatalf("ensureInPath should succeed when dir is already in PATH: %v", err)
	}
}

func TestEnsureInPathAppendsToProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows PATH update via powershell, not shell profile")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/zsh")

	profilePath := filepath.Join(tmpDir, ".zshrc")
	os.WriteFile(profilePath, []byte("export PATH=$PATH:/usr/bin\n"), 0644)

	binDir := filepath.Join(tmpDir, ".local", "bin")
	t.Setenv("PATH", "/usr/bin")

	if err := ensureInPath(binDir); err != nil {
		t.Fatalf("ensureInPath failed: %v", err)
	}

	data, _ := os.ReadFile(profilePath)
	if !strings.Contains(string(data), binDir) {
		t.Errorf("expected profile to contain PATH export for %s", binDir)
	}
}

func TestEnsureInPathDoesNotDuplicate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows PATH update via powershell, not shell profile")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("SHELL", "/bin/zsh")

	profilePath := filepath.Join(tmpDir, ".zshrc")
	os.WriteFile(profilePath, []byte("export PATH=$PATH:/usr/bin\n"), 0644)

	binDir := filepath.Join(tmpDir, ".local", "bin")
	t.Setenv("PATH", "/usr/bin")

	_ = ensureInPath(binDir)
	if err := ensureInPath(binDir); err != nil {
		t.Fatalf("second ensureInPath should succeed: %v", err)
	}

	data, _ := os.ReadFile(profilePath)
	count := strings.Count(string(data), binDir)
	if count > 1 {
		t.Errorf("expected binDir in profile only once, got %d occurrences", count)
	}
}

func TestDetectShellProfileZsh(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	profile := DetectShellProfile()
	if !strings.HasSuffix(profile, ".zshrc") {
		t.Errorf("expected .zshrc for zsh, got %s", profile)
	}
}

func TestDetectShellProfileBash(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")

	profile := DetectShellProfile()
	if !strings.HasSuffix(profile, ".bashrc") {
		t.Errorf("expected .bashrc for bash, got %s", profile)
	}
}

func TestDetectShellProfileDefaultToBash(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")

	profile := DetectShellProfile()
	if !strings.HasSuffix(profile, ".bashrc") {
		t.Errorf("expected .bashrc fallback for unknown shell, got %s", profile)
	}
}

func TestPreferLocalBinaryFindsLocalFF(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create a local ./ff
	localPath := filepath.Join(tmpDir, "ff")
	if err := os.WriteFile(localPath, []byte("local binary content"), 0755); err != nil {
		t.Fatal(err)
	}

	src := preferLocalBinary()
	if src == "" {
		t.Fatal("preferLocalBinary() returned empty for existing local ff")
	}

	// Use os.SameFile to compare file identity — this is robust against
	// macOS /var → /private/var symlink differences that make path-based
	// comparison unreliable.
	srcFi, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat of preferLocalBinary result %s: %v", src, err)
	}
	localFi, err := os.Stat(localPath)
	if err != nil {
		t.Fatalf("stat of local path %s: %v", localPath, err)
	}
	if !os.SameFile(srcFi, localFi) {
		t.Errorf("preferLocalBinary() = %q should reference same file as %q (different inode)", src, localPath)
	}
}

func TestPreferLocalBinaryReturnsEmptyWhenNoLocalFF(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	src := preferLocalBinary()
	if src != "" {
		t.Errorf("expected empty string when no local ff, got %q", src)
	}
}

func TestPreferLocalBinaryReturnsEmptyWhenSameAsRunning(t *testing.T) {
	running, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	runningDir := filepath.Dir(running)
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(runningDir)

	// preferLocalBinary should return empty because the running binary IS the local ff
	src := preferLocalBinary()
	if src != "" {
		t.Errorf("expected empty string when local ff IS the running binary, got %q", src)
	}
}

func TestInstallGlobalPrefersLocalBinary(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	t.Setenv("HOME", tmpDir)

	// Create a fresh local ./ff with distinct content
	localPath := filepath.Join(tmpDir, "ff")
	localContent := []byte("latest local build")
	if err := os.WriteFile(localPath, localContent, 0755); err != nil {
		t.Fatal(err)
	}

	binDir, warning, err := InstallGlobal()
	if err != nil {
		t.Fatalf("InstallGlobal failed: %v", err)
	}

	installed := filepath.Join(binDir, "ff")
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("cannot read installed binary: %v", err)
	}
	if string(data) != string(localContent) {
		t.Errorf("installed binary content doesn't match local ff\ngot:  %q\nwant: %q", string(data), string(localContent))
	}

	_ = warning // warning is expected (PATH update may fail in test env)
}
