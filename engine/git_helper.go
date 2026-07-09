package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitHelper encapsulates git operations for a single repository.
// All raw exec.Command("git", ...) calls should go through this struct
// so staging flags, error handling, and .gitignore semantics are consistent.
type GitHelper struct {
	gitRoot string
}

// NewGitHelper creates a helper rooted at the given repository directory.
func NewGitHelper(gitRoot string) *GitHelper {
	return &GitHelper{gitRoot: gitRoot}
}

// AddAll stages all tracked and untracked changes in the working tree
// (equivalent of git add --all). Uses --all (not .) so that .gitignore
// un-exclusion patterns like .ff/* → !.ff/forgefix_ledger.json are
// correctly handled for tracked files whose content has changed.
func (g *GitHelper) AddAll() error {
	return g.run("add", "--all")
}

// Add stages one or more specific file paths (git add -- <files...>).
func (g *GitHelper) Add(files ...string) error {
	args := append([]string{"add", "--"}, files...)
	return g.run(args...)
}

// Commit creates a commit with the given message and returns the short hash.
// Returns an error wrapping "no changes to commit" if nothing is staged.
func (g *GitHelper) Commit(msg string) (string, error) {
	out, err := exec.Command("git", "-C", g.gitRoot, "commit", "-m", msg).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit") {
			return "", fmt.Errorf("no changes to commit")
		}
		return "", fmt.Errorf("git commit: %w\n%s", err, out)
	}
	fmt.Print(string(out))

	hash, err := exec.Command("git", "-C", g.gitRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(hash)), nil
}

// Amend folds all staged changes into the previous commit with --no-edit.
func (g *GitHelper) Amend() error {
	return g.run("commit", "--amend", "--no-edit")
}

func (g *GitHelper) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.gitRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		// "nothing to commit" / "no changes" are not errors for stash-like operations
		msg := string(out)
		if strings.Contains(msg, "nothing to commit") ||
			strings.Contains(msg, "no changes") {
			return nil
		}
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
