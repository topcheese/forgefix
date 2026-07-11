//go:build windows

package engine

import "os/exec"

func setProcessGroup(cmd *exec.Cmd) {
	// Windows doesn't support Setpgid — child processes inherit the group.
}

func killProcessGroup(pid int) {
	// Windows uses TerminateProcess, not SIGKILL. This is a no-op for now.
}
