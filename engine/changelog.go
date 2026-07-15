package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// changelogSection is a single top-level "## " section in CHANGELOG.md.
type changelogSection struct {
	header  string // e.g. "## [Unreleased] - 2026-07-14"
	bullets []string // "- feat: ..." lines within the section
	raw     string // full section text including the header line
}

// parseChangelog splits CHANGELOG.md content into its top-level "## " sections
// plus any leading preamble (text before the first "## " header). Bullets are
// extracted from each section for easy merging.
func parseChangelog(content string) (preamble string, sections []changelogSection) {
	lines := strings.Split(content, "\n")
	var cur *changelogSection
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if cur != nil {
				sections = append(sections, *cur)
			}
			cur = &changelogSection{header: trimmed, raw: line + "\n"}
			continue
		}
		if cur != nil {
			cur.raw += line + "\n"
			if strings.HasPrefix(trimmed, "- ") {
				cur.bullets = append(cur.bullets, trimmed)
			}
		} else {
			preamble += line + "\n"
		}
	}
	if cur != nil {
		sections = append(sections, *cur)
	}
	return preamble, sections
}

// FinalizeChangelogForRelease promotes every "## [Unreleased] - <date>" section
// in CHANGELOG.md to a single "## [v<version>] - <today>" section. All bullets
// from the Unreleased sections are collected and merged under one versioned header
// placed at the top of the file; already-versioned sections are preserved below it.
// The operation is idempotent: re-running with the same version merges any
// remaining Unreleased bullets into the existing versioned section without
// duplicating them. A missing CHANGELOG.md is a no-op (returns nil).
func FinalizeChangelogForRelease(wd, version string) error {
	changelogPath := filepath.Join(wd, "CHANGELOG.md")
	existing, err := os.ReadFile(changelogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := string(existing)
	versionHeader := fmt.Sprintf("## [v%s]", version)

	preamble, sections := parseChangelog(content)
	var unreleasedBullets []string
	var versioned []string
	var existingVersionBullets []string
	hasVersionSection := false

	for _, sec := range sections {
		switch {
		case strings.HasPrefix(sec.header, "## [Unreleased]"):
			unreleasedBullets = append(unreleasedBullets, sec.bullets...)
		case sec.header == versionHeader:
			hasVersionSection = true
			existingVersionBullets = append(existingVersionBullets, sec.bullets...)
		default:
			versioned = append(versioned, sec.raw)
		}
	}

	if len(unreleasedBullets) == 0 && !hasVersionSection {
		return nil // nothing to finalize
	}

	// Merge: existing version-section bullets first, then any new Unreleased
	// bullets, de-duplicated so re-running is safe.
	merged := append([]string{}, existingVersionBullets...)
	for _, b := range unreleasedBullets {
		if !slices.Contains(merged, b) {
			merged = append(merged, b)
		}
	}

	today := time.Now().Format("2006-01-02")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## [v%s] - %s\n", version, today))
	sb.WriteString("\n")
	sb.WriteString("### 🚀 Release Summary\n")
	for _, b := range merged {
		sb.WriteString(b + "\n")
	}

	var out strings.Builder
	out.WriteString(preamble)
	out.WriteString(sb.String())
	out.WriteString("\n")
	for _, v := range versioned {
		out.WriteString(v)
		if !strings.HasSuffix(v, "\n") {
			out.WriteString("\n")
		}
	}

	return os.WriteFile(changelogPath, []byte(out.String()), 0644)
}
