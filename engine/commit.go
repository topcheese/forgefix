package engine

import (
	"fmt"
	"os/exec"
	"strings"
)

func AutoStageAndCommit(gitRoot, commitMsg string) (string, error) {
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = gitRoot
	out, err := addCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("auto-stage: git add failed: %w\n%s", err, out)
	}

	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitCmd.Dir = gitRoot
	out, err = commitCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit") {
			return "", fmt.Errorf("no changes to commit")
		}
		return "", fmt.Errorf("auto-stage: git commit failed: %w\n%s", err, out)
	}

	fmt.Print(string(out))

	hashOut, err := exec.Command("git", "-C", gitRoot, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("auto-stage: getting commit hash: %w", err)
	}
	return strings.TrimSpace(string(hashOut)), nil
}
// auto-stage test
