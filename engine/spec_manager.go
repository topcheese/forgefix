package engine

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// SpecFile represents a parsed spec file from the specs/ directory.
type SpecFile struct {
	SpecID        string
	Title         string
	Body          string
	RepoIssue     int
	Status        string
	FilePath      string
	Type          string
	Version       string
	RootCause     string
	Resolution    string
	LinkedCommits []string
}

// htmlCommentRE matches HTML comments spanning multiple lines.
var htmlCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)

// headingRE matches a markdown heading line (e.g. "## Objective").
var headingRE = regexp.MustCompile(`(?m)^\s*#{1,6}\s.*$`)

// isTemplateBody reports whether the spec body contains only template scaffolding:
// markdown headings and HTML comments with no real prose. Such bodies should not
// be posted verbatim to a remote issue because they carry no useful content.
func isTemplateBody(body string) bool {
	stripped := htmlCommentRE.ReplaceAllString(body, "")
	stripped = headingRE.ReplaceAllString(stripped, "")
	stripped = strings.TrimSpace(stripped)
	return stripped == ""
}

// generateSpecBody builds a meaningful remote-issue body for a spec whose own
// body is only template scaffolding. It derives content from the spec's
// frontmatter so the resulting issue is not empty.
func generateSpecBody(spec *SpecFile) string {
	var b strings.Builder
	if spec.Title != "" {
		b.WriteString("# ")
		b.WriteString(spec.Title)
		b.WriteString("\n\n")
	}
	var meta []string
	if spec.SpecID != "" {
		meta = append(meta, fmt.Sprintf("**Spec ID:** %s", spec.SpecID))
	}
	if spec.Type != "" {
		meta = append(meta, fmt.Sprintf("**Type:** %s", spec.Type))
	}
	if spec.Version != "" {
		meta = append(meta, fmt.Sprintf("**Version:** %s", spec.Version))
	}
	if spec.Status != "" {
		meta = append(meta, fmt.Sprintf("**Status:** %s", spec.Status))
	}
	if len(meta) > 0 {
		b.WriteString(strings.Join(meta, "  \n"))
		b.WriteString("\n\n")
	}
	if spec.RootCause != "" {
		b.WriteString("**Root Cause:** ")
		b.WriteString(spec.RootCause)
		b.WriteString("\n\n")
	}
	b.WriteString("_This spec is tracked in ForgeFix. Fill in the spec file body for full details._\n")
	return b.String()
}

// effectiveSpecBody returns the body to post for a spec. If the spec's body is
// only template scaffolding, a meaningful body is generated from its frontmatter.
func effectiveSpecBody(spec *SpecFile) string {
	if isTemplateBody(spec.Body) {
		return generateSpecBody(spec)
	}
	return spec.Body
}

// collapseWS collapses all runs of whitespace (including newlines) to a single
// space and trims the ends, so comparisons ignore formatting-only diffs.
func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
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

	// ValidateSpecFrontmatter checks that required frontmatter fields are non-empty
	// for the given status level. Returns a list of missing/empty field names.
	ValidateSpecFrontmatter(spec *SpecFile) []string
}

type specManager struct{}

// NewSpecManager creates a new SpecManager.
func NewSpecManager() SpecManager {
	return &specManager{}
}

// validSpecStatuses contains all recognised spec status values.
var validSpecStatuses = map[string]bool{
	"draft":       true,
	"review":      true,
	"in-progress": true,
	"ship":        true,
	"closed":      true,
	"fixed":       true,
	"resolved":    true,
	"backlog":     true,
}

// isValidSpecStatus checks whether the given status is a known spec status.
func isValidSpecStatus(status string) bool {
	return validSpecStatuses[status]
}

// specStatusLevel returns the validation level for a status:
//   - 0: draft (minimum requirements)
//   - 1: review, in-progress (requires root_cause, resolution)
//   - 2: ship, closed, fixed, resolved (requires root_cause, resolution, linked_commits)
//   - -1: invalid status
func specStatusLevel(status string) int {
	switch status {
	case "draft":
		return 0
	case "review", "in-progress":
		return 1
	case "ship", "closed", "fixed", "resolved":
		return 2
	default:
		return -1
	}
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
		} else if strings.HasPrefix(line, "linked_commits:") {
			// Parse YAML list: [hash1, hash2] or []
			val := strings.TrimSpace(strings.TrimPrefix(line, "linked_commits:"))
			val = strings.Trim(val, `"`)
			if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				inner := val[1 : len(val)-1]
				if inner != "" {
					for _, part := range strings.Split(inner, ",") {
						h := strings.TrimSpace(part)
						h = strings.Trim(h, `"' `)
						if h != "" {
							spec.LinkedCommits = append(spec.LinkedCommits, h)
						}
					}
				}
			}
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

// ValidateSpecFrontmatter checks that required frontmatter fields are non-empty
// for the given status level. Returns a list of human-readable messages describing
// missing or empty fields, or nil if all required fields pass.
func (m *specManager) ValidateSpecFrontmatter(spec *SpecFile) []string {
	var missing []string

	// Always required
	if spec.SpecID == "" {
		missing = append(missing, "spec_id is required")
	}
	if spec.Type == "" {
		missing = append(missing, "type is required")
	}
	if spec.Version == "" {
		missing = append(missing, "version is required")
	}

	// Status must be non-empty and valid
	if spec.Status == "" {
		missing = append(missing, "status is required")
	} else if !isValidSpecStatus(spec.Status) {
		missing = append(missing, fmt.Sprintf("status '%s' is not a valid status", spec.Status))
	}

	// Fields required at levels 1 and above (review, ship, etc.)
	level := specStatusLevel(spec.Status)
	if level >= 1 {
		if spec.RootCause == "" {
			missing = append(missing, fmt.Sprintf("root_cause is required for status '%s'", spec.Status))
		}
		if spec.Resolution == "" {
			missing = append(missing, fmt.Sprintf("resolution is required for status '%s'", spec.Status))
		}
	}

	// Fields required at level 2 and above (ship, closed, etc.)
	if level >= 2 {
		if len(spec.LinkedCommits) == 0 {
			missing = append(missing, fmt.Sprintf("linked_commits is required for status '%s' (at least 1 entry)", spec.Status))
		}
	}

	if len(missing) == 0 {
		return nil
	}
	return missing
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
