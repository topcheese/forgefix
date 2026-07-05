package engine

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// SpecFile represents a parsed spec file from the specs/ directory.
type SpecFile struct {
	SpecID     string
	Title      string
	Body       string
	RepoIssue  int
	Status     string
	FilePath   string
	Type       string
	Version    string
	RootCause  string
	Resolution string
}

// SpecManager handles reading and writing spec files.
// All YAML frontmatter manipulation is confined to implementations of this interface.
type SpecManager interface {
	// ParseSpecFile reads a spec file from disk and parses its YAML frontmatter.
	ParseSpecFile(filePath string) (*SpecFile, error)

	// UpdateRepoIssue sets the repo_issue field in a spec file's frontmatter.
	UpdateRepoIssue(filePath string, issueNumber int) error

	// UpdateStatus sets the status field in a spec file's frontmatter.
	UpdateStatus(filePath string, status string) error

	// SpecWebURL converts a local spec file path to a web URL on the remote
	// (GitHub or Gitea). Returns empty string if the file doesn't exist locally.
	SpecWebURL(apiBase, owner, repo, filePath string) string
}

type specManager struct{}

// NewSpecManager creates a new SpecManager.
func NewSpecManager() SpecManager {
	return &specManager{}
}

// ParseSpecFile reads a spec file from disk and parses its YAML frontmatter.
func (m *specManager) ParseSpecFile(filePath string) (*SpecFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("invalid spec file: missing frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid spec file: malformed frontmatter")
	}

	frontmatter := parts[1]
	body := strings.TrimSpace(parts[2])

	spec := &SpecFile{
		FilePath: filePath,
		Body:     body,
	}

	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "spec_id:") {
			spec.SpecID = strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
			spec.SpecID = strings.Trim(spec.SpecID, `"`)
		} else if strings.HasPrefix(line, "status:") {
			spec.Status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			spec.Status = strings.Trim(spec.Status, `"`)
			if idx := strings.Index(spec.Status, " "); idx > 0 {
				spec.Status = spec.Status[:idx]
			}
		} else if strings.HasPrefix(line, "type:") {
			spec.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
			spec.Type = strings.Trim(spec.Type, `"`)
		} else if strings.HasPrefix(line, "version:") {
			spec.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
			spec.Version = strings.Trim(spec.Version, `"`)
		} else if strings.HasPrefix(line, "repo_issue:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "repo_issue:"))
			if val != "" && val != `""` {
				fmt.Sscanf(val, "%d", &spec.RepoIssue)
			}
		} else if strings.HasPrefix(line, "root_cause:") {
			spec.RootCause = strings.TrimSpace(strings.TrimPrefix(line, "root_cause:"))
			spec.RootCause = strings.Trim(spec.RootCause, `"`)
		} else if strings.HasPrefix(line, "resolution:") {
			spec.Resolution = strings.TrimSpace(strings.TrimPrefix(line, "resolution:"))
			spec.Resolution = strings.Trim(spec.Resolution, `"`)
		}
	}

	if strings.HasPrefix(body, "# ") {
		titleLine := strings.SplitN(body, "\n", 2)[0]
		spec.Title = strings.TrimPrefix(titleLine, "# ")
	}

	return spec, nil
}

// UpdateRepoIssue sets the repo_issue field in a spec file's frontmatter.
func (m *specManager) UpdateRepoIssue(filePath string, issueNumber int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "repo_issue:") {
			lines[i] = fmt.Sprintf("repo_issue: %d", issueNumber)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// UpdateStatus sets the status field in a spec file's frontmatter.
func (m *specManager) UpdateStatus(filePath string, status string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "status:") {
			lines[i] = fmt.Sprintf("status: %s", status)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// Package-level wrappers for backward compatibility with existing callers.
// These delegate to a SpecManager instance and maintain the same function signatures.

// parseSpecFile wraps SpecManager.ParseSpecFile for backward compatibility.
func parseSpecFile(filePath string) (*SpecFile, error) {
	return NewSpecManager().ParseSpecFile(filePath)
}

// updateSpecFileRepoIssue wraps SpecManager.UpdateRepoIssue for backward compatibility.
func updateSpecFileRepoIssue(filePath string, issueNumber int) error {
	return NewSpecManager().UpdateRepoIssue(filePath, issueNumber)
}

// specFileWebURL wraps SpecManager.SpecWebURL for backward compatibility.
func specFileWebURL(apiBase, owner, repo, filePath string) string {
	return NewSpecManager().SpecWebURL(apiBase, owner, repo, filePath)
}

// SpecWebURL converts an API base URL and local spec file path into a
// web URL for the file on the remote (GitHub/Gitea). It handles both
// GitHub (api.github.com) and Gitea (/api/v1) URL patterns.
// Returns empty string if the file doesn't exist locally (e.g., archived).
func (m *specManager) SpecWebURL(apiBase, owner, repo, filePath string) string {
	if apiBase == "" || filePath == "" {
		return ""
	}

	// If the spec file no longer exists locally (e.g., it was archived),
	// return empty so callers can fall back to just the spec ID text
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ""
	}

	apiBase = strings.TrimRight(apiBase, "/")
	webRoot := apiBase

	// Extract the filename from the local path and URL-encode it
	filename := filePath
	if idx := strings.LastIndexByte(filename, '/'); idx >= 0 {
		filename = filename[idx+1:]
	}
	filename = url.PathEscape(filename)

	if strings.Contains(webRoot, "api.github.com") {
		return fmt.Sprintf("https://github.com/%s/%s/blob/main/specs/%s", owner, repo, filename)
	}

	// Gitea: derive web root by stripping /api/* suffix
	if idx := strings.LastIndex(webRoot, "/api/"); idx >= 0 {
		webRoot = webRoot[:idx]
	}

	return fmt.Sprintf("%s/%s/%s/src/branch/main/specs/%s", webRoot, owner, repo, filename)
}
