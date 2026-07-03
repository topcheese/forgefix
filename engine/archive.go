package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ArchiveResolvedSpecs(configDir string) (string, int, error) {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return "", 0, fmt.Errorf("reading specs directory: %w", err)
	}

	type archivedSpec struct {
		content  string
		specID   string
		filename string
	}
	var resolved []archivedSpec

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}

		fm, err := parseSpecFrontmatterOnly(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}

		if fm["status"] == "resolved" || fm["status"] == "closed" {
			resolved = append(resolved, archivedSpec{
				content:  string(data),
				specID:   fm["spec_id"],
				filename: entry.Name(),
			})
		}
	}

	if len(resolved) == 0 {
		return "", 0, nil
	}

	timestamp := time.Now().Format("20060102")
	archiveName := fmt.Sprintf("archive_%s.md", timestamp)
	archivePath := filepath.Join(specDir, archiveName)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Spec Archive - %s\n\n", timestamp))
	sb.WriteString(fmt.Sprintf("Total: %d resolved specs\n\n", len(resolved)))

	for i, spec := range resolved {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(spec.content)
	}
	sb.WriteString("\n")

	if err := os.WriteFile(archivePath, []byte(sb.String()), 0644); err != nil {
		return "", 0, fmt.Errorf("writing archive file: %w", err)
	}

	ledger, err := LoadLedger(configDir)
	if err != nil {
		return "", 0, fmt.Errorf("loading ledger: %w", err)
	}

	for _, spec := range resolved {
		filePath := filepath.Join(specDir, spec.filename)
		if err := os.Remove(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", spec.filename, err)
		}
		if spec.specID != "" {
			ledger.DeleteSpecEntry(spec.specID)
		}
	}

	if err := SaveLedger(ledger, configDir); err != nil {
		return "", 0, fmt.Errorf("saving ledger: %w", err)
	}

	return archiveName, len(resolved), nil
}

func parseSpecFrontmatterOnly(content string) (map[string]string, error) {
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing frontmatter")
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed frontmatter")
	}

	fm := make(map[string]string)
	lines := strings.Split(parts[1], "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])
		val = strings.Trim(val, `"`)
		if idx := strings.Index(val, " "); idx > 0 {
			val = val[:idx]
		}
		fm[key] = val
	}
	return fm, nil
}
