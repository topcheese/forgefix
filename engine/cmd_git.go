package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// GitPassthroughCommands is the set of known git commands that can be proxied.
// Exported so main.go can check for passthrough candidates before dispatching.
var GitPassthroughCommands = map[string]bool{
	"add": true, "am": true, "annotate": true, "archive": true,
	"bisect": true, "blame": true, "branch": true,
	"bundle": true, "cat-file": true, "checkout": true,
	"cherry": true, "cherry-pick": true, "clean": true, "clone": true,
	"commit": true, "config": true, "describe": true, "diff": true,
	"fetch": true, "format-patch": true, "fsck": true,
	"gc": true, "grep": true, "gui": true,
	"init": true, "log": true, "maintenance": true, "merge": true,
	"mergetool": true, "mv": true, "notes": true, "pull": true,
	"push": true, "rebase": true, "reflog": true, "remote": true,
	"repack": true, "replace": true, "request-pull": true,
	"reset": true, "restore": true, "revert": true, "rm": true,
	"shortlog": true, "show": true, "sparse-checkout": true,
	"stash": true, "status": true, "submodule": true, "switch": true,
	"tag": true, "whatchanged": true, "worktree": true,
}

// isGitPassthroughEnabled checks the project config for the git_passthrough flag.
// Defaults to true if not configured.
func (d *CommandDispatcher) isGitPassthroughEnabled() bool {
	loaded, err := LoadPipelineConfig(d.ConfigDir)
	if err != nil || loaded.Config == nil || loaded.Config.GitPassthrough == nil {
		return true // default: enabled
	}
	return *loaded.Config.GitPassthrough
}

// handleGitPassthrough proxies an unknown command to `git <cmd> <args...>`.
// It pipes stdin/stdout/stderr transparently and preserves the exit code.
func (d *CommandDispatcher) handleGitPassthrough(cmd string, args []string) (CommandResult, error) {
	if !d.isGitPassthroughEnabled() {
		// Passthrough disabled — fall through to test runner
		return d.handleRun(cmd, args)
	}

	gitArgs := append([]string{cmd}, args...)
	c := exec.Command("git", gitArgs...)
	c.Dir = d.WorkDir
	c.Stdout = d.Stdout
	c.Stderr = d.Stderr
	c.Stdin = os.Stdin

	err := c.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(d.Stderr, "git passthrough error: %v\n", err)
			exitCode = 1
		}
	}
	return CommandResult{ExitCode: exitCode}, nil
}
