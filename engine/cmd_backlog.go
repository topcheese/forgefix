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

// validateSpecFrontmatter ensures the spec's frontmatter parses as valid YAML with no duplicate keys.
func validateSpecFrontmatter(content string) error {
	lines := strings.Split(content, "\n")
	keys := make(map[string]bool)
	inFrontmatter := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if !inFrontmatter {
			continue
		}

		// Check for key: value format (not array values)
		if strings.Contains(trimmed, ": ") {
			parts := strings.SplitN(trimmed, ": ", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				if keys[key] {
					return fmt.Errorf("duplicate frontmatter key: %s", key)
				}
				keys[key] = true
			}
		}
	}
	return nil
}

// consolidateLinkedCommits removes duplicate linked_commits keys and merges them into one.
func consolidateLinkedCommits(content string) (string, error) {
	lines := strings.Split(content, "\n")
	var newLines []string
	inFrontmatter := false
	collectedValues := make([]string, 0)
	foundLinkedCommits := false

	// First pass: collect all values from all linked_commits keys
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if inFrontmatter && strings.HasPrefix(trimmed, "linked_commits:") {
			foundLinkedCommits = true
			// Parse this occurrence
			start := strings.Index(trimmed, "[")
			end := strings.LastIndex(trimmed, "]")
			if start >= 0 && end > start {
				existing := trimmed[start+1 : end]
				if existing != "" {
					parts := strings.Split(existing, ",")
					for _, p := range parts {
						h := strings.TrimSpace(p)
						h = strings.Trim(h, `"'`)
						if h != "" {
							collectedValues = append(collectedValues, h)
						}
					}
				}
			}
		}
	}

	// Build the final linked_commits line — preserve empty [] if key existed
	linkedCommitsLine := ""
	if foundLinkedCommits {
		var sb strings.Builder
		sb.WriteString("linked_commits: [")
		for i, val := range collectedValues {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf(`"%s"`, val))
		}
		sb.WriteString("]")
		linkedCommitsLine = sb.String()
	}

	// Second pass: rebuild the frontmatter, removing all linked_commits lines
	// and inserting the consolidated version before the closing ---
	inFrontmatter = false
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			newLines = append(newLines, line)
			continue
		}
		if trimmed == "---" && inFrontmatter {
			// Insert consolidated linked_commits before closing ---
			if linkedCommitsLine != "" {
				newLines = append(newLines, linkedCommitsLine)
			}
			// Preserve closing --- and everything after it
			newLines = append(newLines, lines[idx:]...)
			return strings.Join(newLines, "\n"), nil
		}
		if inFrontmatter && strings.HasPrefix(trimmed, "linked_commits:") {
			// Skip all original linked_commits lines (consolidated version replaces them)
			continue
		}
		newLines = append(newLines, line)
	}

	return strings.Join(newLines, "\n"), nil
}

// UpdateSpecFileLinkedCommits appends a commit hash to the spec file's
// linked_commits frontmatter field. If the field doesn't exist it is created.
// If duplicate linked_commits keys exist, they are merged into one.
func UpdateSpecFileLinkedCommits(filePath string, newHash string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)

	// First, consolidate any duplicate linked_commits keys
	content, err = consolidateLinkedCommits(content)
	if err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	targetLineIndex := -1
	currentValues := make([]string, 0)

	// Parse the consolidated content
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
			// Parse existing YAML list
			start := strings.Index(trimmed, "[")
			end := strings.LastIndex(trimmed, "]")
			if start >= 0 && end > start {
				existing := trimmed[start+1 : end]
				if existing != "" {
					parts := strings.Split(existing, ",")
					for _, p := range parts {
						h := strings.TrimSpace(p)
						h = strings.Trim(h, `"'`)
						if h != "" {
							currentValues = append(currentValues, h)
						}
					}
				}
			}
			targetLineIndex = i
		}
	}

	// Check if hash already exists (deduplication)
	for _, h := range currentValues {
		if h == newHash {
			return nil // already present
		}
	}

	// Add new hash and write back
	currentValues = append(currentValues, newHash)

	// Reconstruct the line
	var sb strings.Builder
	sb.WriteString("linked_commits: [")
	for i, val := range currentValues {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf(`"%s"`, val))
	}
	sb.WriteString("]")

	if targetLineIndex >= 0 {
		lines[targetLineIndex] = sb.String()
	} else {
		// No existing linked_commits line — insert before the closing ---
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" && i > 0 {
				// Find the last frontmatter key to insert after it
				insertIdx := i
				for j := i - 1; j >= 0; j-- {
					if strings.TrimSpace(lines[j]) != "" {
						insertIdx = j + 1
						break
					}
				}
				lines = append(lines[:insertIdx], append([]string{sb.String()}, lines[insertIdx:]...)...)
				break
			}
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// ReplaceSpecFileLastLinkedCommit replaces the last entry in the spec file's
// linked_commits frontmatter with newHash. This is used after a commit amend
// to correct the pre-amend SHA to the final SHA.
func ReplaceSpecFileLastLinkedCommit(filePath string, newHash string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)

	content, err = consolidateLinkedCommits(content)
	if err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	targetLineIndex := -1
	currentValues := make([]string, 0)

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
			start := strings.Index(trimmed, "[")
			end := strings.LastIndex(trimmed, "]")
			if start >= 0 && end > start {
				existing := trimmed[start+1 : end]
				if existing != "" {
					parts := strings.Split(existing, ",")
					for _, p := range parts {
						h := strings.TrimSpace(p)
						h = strings.Trim(h, `"'`)
						if h != "" {
							currentValues = append(currentValues, h)
						}
					}
				}
			}
			targetLineIndex = i
		}
	}

	if targetLineIndex == -1 {
		return UpdateSpecFileLinkedCommits(filePath, newHash)
	}

	if len(currentValues) > 0 {
		currentValues[len(currentValues)-1] = newHash
	} else {
		currentValues = append(currentValues, newHash)
	}

	var sb2 strings.Builder
	sb2.WriteString("linked_commits: [")
	for i, val := range currentValues {
		if i > 0 {
			sb2.WriteString(", ")
		}
		sb2.WriteString(fmt.Sprintf(`"%s"`, val))
	}
	sb2.WriteString("]")
	lines[targetLineIndex] = sb2.String()

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}