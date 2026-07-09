package engine

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (d *CommandDispatcher) handleBacklog(args []string) (CommandResult, error) {
	specID := extractSpecIDFromArgs(args)
	if specID == "" {
		fmt.Fprintln(d.Stderr, "error: backlog requires a spec ID")
		fmt.Fprintln(d.Stderr, "usage: ff backlog <spec_id>")
		return CommandResult{ExitCode: 1}, nil
	}

	specDir := filepath.Join(d.ConfigDir, "specs")
	specFile, err := findSpecFileByID(specDir, specID)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: spec %s not found\n", specID)
		return CommandResult{ExitCode: 1}, nil
	}

	diskStatus, err := readSpecFileStatus(specFile)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: reading spec status: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	ledger, err := LoadLedger(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: loading ledger: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	entry := ledger.GetSpecEntry(specID)

	currentStatus := diskStatus
	if entry != nil && entry.Status != "" {
		currentStatus = entry.Status
	}

	terminal := map[string]bool{"ship": true, "closed": true, "resolved": true, "deprecated": true}
	if terminal[currentStatus] {
		fmt.Fprintf(d.Stderr, "error: spec %s is %s — cannot backlog\n", specID, currentStatus)
		return CommandResult{ExitCode: 1}, nil
	}

	if currentStatus == "backlog" {
		return d.promptAdvancement(specID, specFile, entry, ledger)
	}

	if err := UpdateSpecFileStatus(specFile, "backlog"); err != nil {
		fmt.Fprintf(d.Stderr, "error: updating spec file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	if entry != nil {
		entry.Status = "backlog"
		ledger.SetSpecEntry(specID, entry)
		_ = SaveLedger(ledger, d.ConfigDir)
	}

	fmt.Fprintf(d.Stdout, "✓ Spec %s moved to backlog\n", specID)
	queueSpecBackgroundSync(d.ConfigDir, specID, d.Stdout, d.Stderr)
	return CommandResult{ExitCode: 0}, nil
}

func (d *CommandDispatcher) promptAdvancement(specID, specFile string, entry *SpecEntry, ledger *LedgerEngine) (CommandResult, error) {
	fmt.Fprintf(d.Stderr, "Spec %s is already backlog. Select advancement:\n", specID)
	fmt.Fprintln(d.Stderr, "  1. in-progress")
	fmt.Fprintln(d.Stderr, "  2. review")
	fmt.Fprintln(d.Stderr, "  3. ship")
	fmt.Fprintln(d.Stderr, "  4. close")
	fmt.Fprintln(d.Stderr, "  5. delete")
	fmt.Fprintln(d.Stderr, "  q. quit")
	fmt.Fprint(d.Stderr, "Choice: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var newStatus string
	switch input {
	case "1":
		newStatus = "in-progress"
	case "2":
		newStatus = "review"
	case "3":
		newStatus = "ship"
	case "4":
		newStatus = "closed"
	case "5":
		if err := deleteSpec(d.ConfigDir, specID); err != nil {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		fmt.Fprintf(d.Stdout, "✓ Deleted spec %s\n", specID)
		return CommandResult{ExitCode: 0}, nil
	case "q", "":
		return CommandResult{ExitCode: 0}, nil
	default:
		fmt.Fprintln(d.Stderr, "Invalid choice")
		return CommandResult{ExitCode: 1}, nil
	}

	if err := UpdateSpecFileStatus(specFile, newStatus); err != nil {
		fmt.Fprintf(d.Stderr, "error: updating spec file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	if entry != nil {
		entry.Status = newStatus
		ledger.SetSpecEntry(specID, entry)
		_ = SaveLedger(ledger, d.ConfigDir)
	}

	fmt.Fprintf(d.Stdout, "✓ Spec %s advanced to %s\n", specID, newStatus)
	queueSpecBackgroundSync(d.ConfigDir, specID, d.Stdout, d.Stderr)
	return CommandResult{ExitCode: 0}, nil
}

func extractSpecIDFromArgs(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func readSpecFileStatus(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return "", fmt.Errorf("missing frontmatter")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("malformed frontmatter")
	}
	lines := strings.Split(parts[1], "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "status:") {
			status := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			status = strings.Split(status, " ")[0]
			return status, nil
		}
	}
	return "", fmt.Errorf("status field not found in frontmatter")
}

func queueSpecBackgroundSync(configDir, specID string, stdout, stderr io.Writer) {
	loaded, loadErr := LoadPipelineConfig(configDir)
	if loadErr == nil && loaded.Config != nil {
		if err := QueueSyncSpec(loaded.ConfigDir, specID); err != nil {
			fmt.Fprintf(stderr, "warning: failed to queue spec sync: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "Queued sync for spec %s\n", specID)
		}

		if loaded.Config.AutoIssueManagement {
			if err := SpawnBackgroundSync(loaded.ConfigDir, specID); err != nil {
				fmt.Fprintf(stderr, "warning: failed to spawn background sync: %v\n", err)
			} else {
				fmt.Fprintf(stdout, "Triggered background sync for remote reconciliation\n")
			}
		}
	}
}

func UpdateSpecFileStatus(filePath, status string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if inFrontmatter && strings.HasPrefix(trimmed, "status:") {
			lines[i] = fmt.Sprintf("status: %s", status)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("status field not found in frontmatter")
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}
