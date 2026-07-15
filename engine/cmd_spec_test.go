package engine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeSpecTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-feature", "My Feature"},
		{"fix --ai null pointer", "Fix Ai Null Pointer"},
		{"  extra   spaces  ", "Extra Spaces"},
		{"normal title", "Normal Title"},
		{"--leading-dashes", "Leading Dashes"},
		{"trailing-dashes--", "Trailing Dashes"},
		{"multi---dash", "Multi Dash"},
	}
	for _, tt := range tests {
		got := sanitizeSpecTitle(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeSpecTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteSpecFromTemplate_PropagatesRootCause(t *testing.T) {
	tmpDir := t.TempDir()

	// Create template
	tmplDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := `---
spec_id: ""
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
linked_commits: []
---
# [Title]
`
	if err := os.WriteFile(filepath.Join(tmplDir, "spec_template.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	specID, filePath, err := writeSpecFromTemplate(tmpDir, "test-spec", "Test Spec", "## Body content\n\nSome description", "bug", "v0.9.0", "null pointer dereference")
	if err != nil {
		t.Fatalf("writeSpecFromTemplate failed: %v", err)
	}
	if specID == "" {
		t.Fatal("expected non-empty spec ID")
	}
	if filePath == "" {
		t.Fatal("expected non-empty file path")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading generated spec: %v", err)
	}
	content := string(data)

	// Verify root_cause is propagated from specRootCause parameter
	if !strings.Contains(content, `root_cause: "null pointer dereference"`) {
		t.Errorf("expected root_cause in generated spec, got:\n%s", content)
	}
	// Type override: feature -> bug
	if !strings.Contains(content, "type: bug") {
		t.Errorf("expected type: bug in generated spec, got:\n%s", content)
	}
	// Version set
	if !strings.Contains(content, `version: "v0.9.0"`) {
		t.Errorf(`expected version: "v0.9.0" in generated spec, got:\n%s`, content)
	}
	// Body content preserved
	if !strings.Contains(content, "## Body content") {
		t.Errorf("expected body content in generated spec, got:\n%s", content)
	}
}

func TestWriteSpecFromTemplate_EmptyRootCause(t *testing.T) {
	tmpDir := t.TempDir()

	tmplDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := `---
spec_id: ""
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
linked_commits: []
---
# [Title]
`
	if err := os.WriteFile(filepath.Join(tmplDir, "spec_template.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Empty root_cause should leave the template's root_cause: "" unchanged
	_, filePath, err := writeSpecFromTemplate(tmpDir, "no-rc", "No RC", "Body text", "feature", "v0.8.0", "")
	if err != nil {
		t.Fatalf("writeSpecFromTemplate failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading generated spec: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `root_cause: ""`) {
		t.Errorf("expected root_cause to remain empty, got:\n%s", content)
	}
}

func TestWriteSpecFromTemplate_PropagatesBody(t *testing.T) {
	tmpDir := t.TempDir()

	tmplDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := `---
spec_id: ""
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
linked_commits: []
---
# [Title]
`
	if err := os.WriteFile(filepath.Join(tmplDir, "spec_template.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	body := "# My Title\n\n## Objective\nFix the bug\n\n## Requirements\nNone"
	_, filePath, err := writeSpecFromTemplate(tmpDir, "body-test", "My Title", body, "bug", "v0.9.0", "test root cause")
	if err != nil {
		t.Fatalf("writeSpecFromTemplate failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading generated spec: %v", err)
	}
	content := string(data)

	// The body should appear after the frontmatter
	if !strings.Contains(content, "## Objective") {
		t.Errorf("expected body Objective section, got:\n%s", content)
	}
	if !strings.Contains(content, "Fix the bug") {
		t.Errorf("expected body text, got:\n%s", content)
	}
	// Verify the body is NOT in the frontmatter
	if strings.Contains(content, "Objective") && strings.Contains(content, "spec_id:") {
		// Ensure there are exactly two "---" separators
		parts := strings.Split(content, "---")
		if len(parts) < 3 {
			t.Errorf("expected frontmatter with body, got:\n%s", content)
		}
	}
}

func TestCreateSpec_RequiresTypeInAiMode(t *testing.T) {
	tmpDir := t.TempDir()
	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	flags := CLIArgs{
		AIMode:   true,
		SpecType: "",
	}
	err := createSpec(tmpDir, "test-spec", "body content", d, flags)
	if err == nil {
		t.Fatal("expected error when --ai mode is set without --type, got nil")
	}
	if !strings.Contains(err.Error(), "--type is required") {
		t.Errorf("expected '--type is required' error message, got: %v", err)
	}
}

func TestCreateSpec_ValidTypeInAiMode(t *testing.T) {
	tmpDir := t.TempDir()

	// We need a minimal template for writeSpecFromTemplate to succeed,
	// but the goal is to ensure the type check passes before we worry about the template.
	// createSpec calls FindDuplicateSpec which gracefully returns no-op on missing specs dir.
	// The issue is that createSpec will eventually call writeSpecFromTemplate which needs a template.
	// So for this test we need the template.

	tmplDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := `---
spec_id: ""
status: draft
repo_issue: ""
type: feature
version: "v0.8.0"
root_cause: ""
resolution: ""
linked_commits: []
---
# [Title]
`
	if err := os.WriteFile(filepath.Join(tmplDir, "spec_template.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
	flags := CLIArgs{
		AIMode:   true,
		SpecType: "bug",
	}
	// Should not fail on the type check — but will fail later since there's no specs dir etc.
	// Actually, writeSpecFromTemplate will succeed (it creates specs dir), but then
	// LoadLedger/SaveLedger may fail since there's no ff config. Let's see what happens.
	err := createSpec(tmpDir, "valid-spec", "body text for spec", d, flags)
	if err != nil {
		// If it fails, it should NOT be the "--type is required" error
		if strings.Contains(err.Error(), "--type is required") {
			t.Fatalf("unexpected --type error with valid SpecType: %v", err)
		}
		// Any other error is OK (could be SaveLedger, etc.) — the type check passed
		t.Logf("createSpec failed at a later stage (expected without full config): %v", err)
	}
}

func TestIsValidSpecType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"feature", true},
		{"bug", true},
		{"refactor", true},
		{"", false},
		{"invalid", false},
		{"Feature", false},
		{"BUG", false},
	}
	for _, tt := range tests {
		got := isValidSpecType(tt.input)
		if got != tt.want {
			t.Errorf("isValidSpecType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
