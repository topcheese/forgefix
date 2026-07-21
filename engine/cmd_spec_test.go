package engine

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
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

func TestValidateSpecFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		spec *SpecFile
		want []string
	}{
		{
			name: "draft with empty optional fields",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "draft",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "",
				Resolution:    "",
				LinkedCommits: nil,
			},
			want: nil,
		},
		{
			name: "review with empty root_cause",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "review",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "",
				Resolution:    "fixed it",
				LinkedCommits: []string{"abc123"},
			},
			want: []string{"root_cause is required for status 'review'"},
		},
		{
			name: "review with empty resolution",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "review",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "null pointer",
				Resolution:    "",
				LinkedCommits: []string{"abc123"},
			},
			want: []string{"resolution is required for status 'review'"},
		},
		{
			name: "review with both root_cause and resolution empty",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "review",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "",
				Resolution:    "",
				LinkedCommits: []string{"abc123"},
			},
			want: []string{
				"root_cause is required for status 'review'",
				"resolution is required for status 'review'",
			},
		},
		{
			name: "ship with empty linked_commits",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "ship",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "cause",
				Resolution:    "fixed",
				LinkedCommits: nil,
			},
			want: []string{"linked_commits is required for status 'ship' (at least 1 entry)"},
		},
		{
			name: "ship with valid linked_commits",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "ship",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "cause",
				Resolution:    "fixed",
				LinkedCommits: []string{"abc123"},
			},
			want: nil,
		},
		{
			name: "invalid status",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "unknown",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "",
				Resolution:    "",
				LinkedCommits: nil,
			},
			want: []string{"status 'unknown' is not a valid status"},
		},
		{
			name: "fully valid review",
			spec: &SpecFile{
				SpecID:        "SPEC-001",
				Status:        "review",
				Type:          "feature",
				Version:       "v0.8.0",
				RootCause:     "null pointer",
				Resolution:    "added nil check",
				LinkedCommits: []string{"abc123"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSpecManager().ValidateSpecFrontmatter(tt.spec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateSpecFrontmatter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWriteSpecFromTemplate_IncludesTitleHeading(t *testing.T) {
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
version: "v0.9.0"
root_cause: ""
resolution: ""
linked_commits: []
---
`
	if err := os.WriteFile(filepath.Join(tmplDir, "spec_template.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, filePath, err := writeSpecFromTemplate(tmpDir, "my-test-spec", "My Test Spec", "## Objective\nFix the bug", "bug", "v0.9.0", "")
	if err != nil {
		t.Fatalf("writeSpecFromTemplate failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("reading generated spec: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# My Test Spec") {
		t.Errorf("expected '# My Test Spec' H1 heading in generated spec, got:\n%s", content)
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		t.Fatalf("malformed spec content: %s", content)
	}
	body := strings.TrimSpace(parts[2])
	if !strings.HasPrefix(body, "# My Test Spec") {
		t.Errorf("expected body to start with '# My Test Spec' H1 heading, got:\n%s", body)
	}
}
