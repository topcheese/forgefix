package engine

import (
	"fmt"
	"io"
	"os"
	"strings"
)

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

// UpdateSpecFileLinkedCommits appends a commit hash to the spec file's
// linked_commits frontmatter field. If the field doesn't exist it is created.
func UpdateSpecFileLinkedCommits(filePath string, newHash string) error {
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
		if inFrontmatter && strings.HasPrefix(trimmed, "linked_commits:") {
			// Parse existing YAML list: [hash1, hash2] or []
			start := strings.Index(trimmed, "[")
			end := strings.LastIndex(trimmed, "]")
			if start >= 0 && end > start {
				existing := trimmed[start+1 : end]
				if existing != "" {
					parts := strings.Split(existing, ",")
					for _, p := range parts {
						h := strings.TrimSpace(p)
						h = strings.Trim(h, `"'`)
						if h == newHash {
							return nil // already present
						}
					}
				}
				var sb strings.Builder
				sb.WriteString("linked_commits: [")
				if existing != "" {
					sb.WriteString(existing)
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf(`"%s"`, newHash))
				sb.WriteString("]")
				lines[i] = sb.String()
				found = true
			}
			break
		}
	}
	if !found {
		// Field doesn't exist — add it before the closing ---
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" && i > 0 {
				lines[i] = fmt.Sprintf(`linked_commits: ["%s"]`, newHash)
				lines = append(lines[:i+1], append([]string{lines[i]}, lines[i+1:]...)...)
				found = true
				break
			}
		}
	}
	if !found {
		return fmt.Errorf("could not find frontmatter to add linked_commits")
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}
