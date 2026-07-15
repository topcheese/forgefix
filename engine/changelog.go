package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// changelogEntryRegex parses a conventional-commit message of the form
//
//	feat: [SPEC-123] description
//
// into its type, optional SPEC-ID, and description.
var changelogEntryRegex = regexp.MustCompile(`^([A-Za-z0-9_]+):\s*(?:\[(SPEC-\d+)\]\s*)?(.+)$`)

// changelogItem is a single parsed changelog bullet.
type changelogItem struct {
	ctype  string
	specID string
	desc   string
}

// line renders the changelog bullet for this item.
func (c *changelogItem) line() string {
	if c.specID != "" {
		return fmt.Sprintf("- %s: %s (%s)", c.ctype, c.desc, c.specID)
	}
	return fmt.Sprintf("- %s: %s", c.ctype, c.desc)
}

// parseChangelogEntry extracts a changelogItem from a commit message.
// Returns nil when the message is not a recognizable conventional commit, so
// non-conventional commits are silently skipped.
func parseChangelogEntry(commitMsg string) *changelogItem {
	m := changelogEntryRegex.FindStringSubmatch(strings.TrimSpace(commitMsg))
	if m == nil {
		return nil
	}
	desc := strings.TrimSpace(m[3])
	// A description that is only a SPEC tag (no real text) means the
	// message carried no description, e.g. "feat: [SPEC-1]". Treat as empty.
	if desc == "" || (strings.HasPrefix(desc, "[SPEC-") && !strings.Contains(desc, " ")) {
		return nil
	}
	return &changelogItem{ctype: m[1], specID: m[2], desc: desc}
}

// AppendChangelogEntry records a commit in the project CHANGELOG.md so the
// changelog stays in sync with changes without manual release bookkeeping.
// Entries are grouped under a dated "## [Unreleased] - YYYY-MM-DD" section at
// the top of the file. The section and its "Release Summary" block are created
// on demand, and a missing CHANGELOG.md is created rather than failing.
// Errors are returned so callers can treat the update as best-effort.
func AppendChangelogEntry(wd, commitMsg string) error {
	item := parseChangelogEntry(commitMsg)
	if item == nil {
		return nil
	}

	changelogPath := filepath.Join(wd, "CHANGELOG.md")
	existing, err := os.ReadFile(changelogPath)
	if err != nil {
		if os.IsNotExist(err) {
			content := buildChangelogSection(time.Now(), item.line())
			return os.WriteFile(changelogPath, []byte(content), 0644)
		}
		return err
	}

	content := string(existing)
	today := time.Now().Format("2006-01-02")
	header := fmt.Sprintf("## [Unreleased] - %s", today)

	if strings.Contains(content, header) {
		content = appendBulletToSection(content, header, item.line())
	} else {
		newSection := buildChangelogSection(time.Now(), item.line())
		content = newSection + "\n" + content
	}

	return os.WriteFile(changelogPath, []byte(content), 0644)
}

// buildChangelogSection renders a fresh dated changelog section containing the
// given bullet line(s).
func buildChangelogSection(date time.Time, bullets ...string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## [Unreleased] - %s\n", date.Format("2006-01-02")))
	sb.WriteString("\n")
	sb.WriteString("### 🚀 Release Summary\n")
	for _, b := range bullets {
		sb.WriteString(b + "\n")
	}
	return sb.String()
}

// appendBulletToSection inserts bullet into the dated section identified by
// header (e.g. "## [Unreleased] - 2026-07-12"), placing it as the last
// bullet of that section's "### 🚀 Release Summary" block. If the summary
// block is missing it is created directly under the header.
func appendBulletToSection(content, header, bullet string) string {
	lines := strings.Split(content, "\n")

	headerIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == header {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		// Header vanished between read and write — prepend a fresh section.
		return buildChangelogSection(time.Now(), bullet) + "\n" + content
	}

	// Bound the section: it ends at the next "## " header (or EOF).
	nextHeader := len(lines)
	summaryIdx := -1
	for i := headerIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			nextHeader = i
			break
		}
		if trimmed == "### 🚀 Release Summary" {
			summaryIdx = i
		}
	}

	// Idempotent: skip if the exact bullet already exists in this section.
	for i := headerIdx + 1; i < nextHeader; i++ {
		if strings.TrimSpace(lines[i]) == bullet {
			return content
		}
	}

	insertAt := headerIdx + 1
	if summaryIdx >= 0 {
		lastBullet := -1
		for i := summaryIdx + 1; i < nextHeader; i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "- ") {
				lastBullet = i
			}
		}
		if lastBullet >= 0 {
			insertAt = lastBullet + 1
		} else {
			insertAt = summaryIdx + 1
		}
	}

	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertAt]...)
	result = append(result, bullet)
	result = append(result, lines[insertAt:]...)
	return strings.Join(result, "\n")
}
