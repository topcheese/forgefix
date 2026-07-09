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

	// Create archive directory
	archiveDir := filepath.Join(specDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", 0, fmt.Errorf("creating archive directory: %w", err)
	}

	// First pass: identify spec files to archive and move existing archive files
	var specFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip summary and other non-spec files
		if strings.HasPrefix(name, "27-specs-summary") {
			continue
		}
		// Move existing archive files from specs/ to specs/archive/
		if strings.HasPrefix(name, "archive_") && strings.HasSuffix(name, ".md") {
			oldPath := filepath.Join(specDir, name)
			newPath := filepath.Join(archiveDir, name)
			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to move %s to archive: %v\n", name, err)
			}
			continue // Skip processing archive files as spec files
		}
		// Add regular spec files for processing
		specFiles = append(specFiles, name)
	}

	// Second pass: process the remaining spec files
	for _, name := range specFiles {
		filePath := filepath.Join(specDir, name)

		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", name, err)
			continue
		}

		fm, err := parseSpecFrontmatterOnly(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", name, err)
			continue
		}

		if fm["status"] == "resolved" || fm["status"] == "closed" {
			resolved = append(resolved, archivedSpec{
				content:  string(data),
				specID:   fm["spec_id"],
				filename: name,
			})
		}
	}

	ledger, err := LoadLedger(configDir)
	if err != nil {
		return "", 0, fmt.Errorf("loading ledger: %w", err)
	}

	// Also find resolved/closed specs in the ledger whose files are missing
	seenIDs := make(map[string]bool, len(resolved))
	for _, spec := range resolved {
		seenIDs[spec.specID] = true
	}

	for specID, entry := range ledger.GetAllSpecEntries() {
		if entry.Status != "resolved" && entry.Status != "closed" {
			continue
		}
		if seenIDs[specID] {
			continue
		}
		// File is missing; reconstruct minimal archive entry from ledger data
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("spec_id: \"%s\"\n", specID))
		sb.WriteString(fmt.Sprintf("status: %s\n", entry.Status))
		if entry.RepoIssueID > 0 {
			sb.WriteString(fmt.Sprintf("repo_issue: %d\n", entry.RepoIssueID))
		}
		if entry.Type != "" {
			sb.WriteString(fmt.Sprintf("type: %s\n", entry.Type))
		}
		if len(entry.LinkedCommits) > 0 {
			sb.WriteString(fmt.Sprintf("linked_commits: %s\n", strings.Join(entry.LinkedCommits, ", ")))
		}
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("# %s\n\n", specID))
		sb.WriteString("*Spec file was missing at time of archive. Content reconstructed from ledger.*\n")
		resolved = append(resolved, archivedSpec{
			content:  sb.String(),
			specID:   specID,
			filename: "",
		})
	}

	if len(resolved) == 0 {
		return "", 0, nil
	}

	timestamp := time.Now().Format("20060102")
	archiveName := fmt.Sprintf("archive_%s.md", timestamp)
	archivePath := filepath.Join(archiveDir, archiveName)

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

	for _, spec := range resolved {
		if spec.filename != "" {
			filePath := filepath.Join(specDir, spec.filename)
			if err := os.Remove(filePath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", spec.filename, err)
			}
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
