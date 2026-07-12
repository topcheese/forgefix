package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChangelogEntry(t *testing.T) {
	cases := []struct {
		name     string
		msg      string
		wantNil  bool
		wantType string
		wantSpec string
		wantDesc string
	}{
		{"feat with spec", "feat: [SPEC-123] add thing", false, "feat", "SPEC-123", "add thing"},
		{"fix no spec", "fix: resolve crash", false, "fix", "", "resolve crash"},
		{"chore", "chore: tidy imports", false, "chore", "", "tidy imports"},
		{"no colon", "just a plain message", true, "", "", ""},
		{"empty desc", "feat: [SPEC-1]   ", true, "", "", ""},
		{"upcased type", "Feat: do work", false, "Feat", "", "do work"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			item := parseChangelogEntry(c.msg)
			if c.wantNil {
				if item != nil {
					t.Fatalf("expected nil for %q, got %+v", c.msg, item)
				}
				return
			}
			if item == nil {
				t.Fatalf("expected item for %q, got nil", c.msg)
			}
			if item.ctype != c.wantType || item.specID != c.wantSpec || item.desc != c.wantDesc {
				t.Errorf("got (%q,%q,%q) want (%q,%q,%q)",
					item.ctype, item.specID, item.desc, c.wantType, c.wantSpec, c.wantDesc)
			}
		})
	}
}

func TestChangelogItemLine(t *testing.T) {
	withSpec := &changelogItem{ctype: "feat", specID: "SPEC-1", desc: "add thing"}
	if got := withSpec.line(); got != "- feat: add thing (SPEC-1)" {
		t.Errorf("unexpected line: %q", got)
	}
	noSpec := &changelogItem{ctype: "fix", desc: "resolve crash"}
	if got := noSpec.line(); got != "- fix: resolve crash" {
		t.Errorf("unexpected line: %q", got)
	}
}

func TestAppendChangelogEntry_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := AppendChangelogEntry(tmpDir, "feat: [SPEC-123] add thing"); err != nil {
		t.Fatalf("AppendChangelogEntry: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "## [Unreleased] - ") {
		t.Errorf("expected dated Unreleased header, got:\n%s", content)
	}
	if !strings.Contains(content, "### 🚀 Release Summary") {
		t.Errorf("expected Release Summary block, got:\n%s", content)
	}
	if !strings.Contains(content, "- feat: add thing (SPEC-123)") {
		t.Errorf("expected bullet, got:\n%s", content)
	}
}

func TestAppendChangelogEntry_SameDayAppends(t *testing.T) {
	tmpDir := t.TempDir()
	if err := AppendChangelogEntry(tmpDir, "feat: [SPEC-1] first change"); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelogEntry(tmpDir, "fix: [SPEC-2] second change"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(tmpDir, "CHANGELOG.md"))
	content := string(data)

	// Only ONE dated Unreleased section should exist.
	if n := strings.Count(content, "## [Unreleased] - "); n != 1 {
		t.Errorf("expected exactly 1 Unreleased section, got %d:\n%s", n, content)
	}
	if !strings.Contains(content, "- feat: first change (SPEC-1)") {
		t.Errorf("missing first bullet:\n%s", content)
	}
	if !strings.Contains(content, "- fix: second change (SPEC-2)") {
		t.Errorf("missing second bullet:\n%s", content)
	}
}

func TestAppendChangelogEntry_NoDuplicateBullet(t *testing.T) {
	tmpDir := t.TempDir()
	msg := "feat: [SPEC-9] idempotent change"
	if err := AppendChangelogEntry(tmpDir, msg); err != nil {
		t.Fatal(err)
	}
	// Re-running the same commit must not duplicate the bullet.
	if err := AppendChangelogEntry(tmpDir, msg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(tmpDir, "CHANGELOG.md"))
	if n := strings.Count(string(data), "- feat: idempotent change (SPEC-9)"); n != 1 {
		t.Errorf("expected exactly 1 bullet, got %d:\n%s", n, string(data))
	}
}

func TestAppendChangelogEntry_NonConventionalIsNoOp(t *testing.T) {
	tmpDir := t.TempDir()
	if err := AppendChangelogEntry(tmpDir, "wrote some stuff"); err != nil {
		t.Fatalf("expected no error for non-conventional msg, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Errorf("expected no CHANGELOG.md for non-conventional commit")
	}
}

func TestAppendChangelogEntry_PrependsToExisting(t *testing.T) {
	tmpDir := t.TempDir()
	existing := `## [v0.9.0] - 2026-07-05

### 🚀 Release Summary
- feat: old release entry (SPEC-1)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "CHANGELOG.md"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AppendChangelogEntry(tmpDir, "fix: [SPEC-2] new fix"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(tmpDir, "CHANGELOG.md"))
	content := string(data)
	if !strings.Contains(content, "## [Unreleased] - ") {
		t.Errorf("expected new Unreleased section at top:\n%s", content)
	}
	// New section must appear before the existing v0.9.0 section.
	unrelIdx := strings.Index(content, "## [Unreleased]")
	v090Idx := strings.Index(content, "## [v0.9.0]")
	if unrelIdx < 0 || v090Idx < 0 || unrelIdx > v090Idx {
		t.Errorf("Unreleased section should precede v0.9.0:\n%s", content)
	}
}
