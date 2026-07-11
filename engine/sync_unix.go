//go:build !windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

func SpawnBackgroundSync(configDir, specID string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	args := []string{"sync"}
	if specID != "" {
		args = append(args, "--spec", specID)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = configDir
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting background sync: %w", err)
	}
	return cmd.Process.Release()
}
