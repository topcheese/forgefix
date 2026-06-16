package engine

import (
	"strings"
	"testing"
)

func TestIssueTitleValidator_ValidTypes(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat/engine: add new dashboard renderer",
		"fix/sync: resolve issue closing logic",
		"docs/config: update setup guide",
		"refactor/engine: simplify issue coordinator",
		"feat/config: add multi-backend support",
		"fix/driver: handle 404 errors",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err != nil {
			t.Errorf("Validate(%q) = %v, want nil", tt, err)
		}
	}
}

func TestIssueTitleValidator_InvalidTypes(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feature/engine: add new feature",
		"bug/sync: fix something",
		"chore/engine: cleanup",
		"test/sync: add tests",
		"invalid/engine: test",
		"FEAT/engine: test",
		"Fix/sync: test",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error", tt)
		}
	}
}

func TestIssueTitleValidator_MissingCategory(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat: missing category",
		"fix: no category here",
		"docs: missing",
		"refactor:",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for missing category", tt)
		}
	}
}

func TestIssueTitleValidator_MissingColon(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat/engine missing colon",
		"fix/sync no colon here",
		"docs/config missing",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for missing colon", tt)
		}
	}
}

func TestIssueTitleValidator_TooLong(t *testing.T) {
	validator := NewIssueTitleValidator()

	longTitle := "feat/engine: " + strings.Repeat("a", 50)
	if len(longTitle) <= 60 {
		t.Fatal("test setup error: title not long enough")
	}

	err := validator.Validate(longTitle)
	if err == nil {
		t.Errorf("Validate(long title) = nil, want error for >60 chars")
	}
}

func TestIssueTitleValidator_Exactly60Chars(t *testing.T) {
	validator := NewIssueTitleValidator()

	exactTitle := "feat/engine: " + strings.Repeat("a", 47)
	if len(exactTitle) != 60 {
		t.Fatalf("test setup error: title length is %d, want 60", len(exactTitle))
	}

	err := validator.Validate(exactTitle)
	if err != nil {
		t.Errorf("Validate(60 chars) = %v, want nil", err)
	}
}

func TestIssueTitleValidator_TrailingPunctuation(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat/engine: add new feature.",
		"fix/sync: resolve bug!",
		"docs/config: update guide?",
		"refactor/engine: simplify code...",
		"feat/engine: test...",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for trailing punctuation", tt)
		}
	}
}

func TestIssueTitleValidator_EmptyTitle(t *testing.T) {
	validator := NewIssueTitleValidator()

	err := validator.Validate("")
	if err == nil {
		t.Error("Validate(empty) = nil, want error")
	}
}

func TestIssueTitleValidator_WhitespaceOnly(t *testing.T) {
	validator := NewIssueTitleValidator()

	err := validator.Validate("   ")
	if err == nil {
		t.Error("Validate(whitespace) = nil, want error")
	}
}

func TestIssueTitleValidator_InvalidCategoryChars(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat/engine@v1: invalid char",
		"fix/sync_test: underscore",
		"docs/config path: space",
		"refactor/ENGINE: uppercase",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for invalid category", tt)
		}
	}
}

func TestIsValidIssueTitle_Helper(t *testing.T) {
	tests := []struct {
		title string
		valid bool
	}{
		{title: "feat/engine: add feature", valid: true},
		{title: "fix/sync: fix bug", valid: true},
		{title: "docs/config: update", valid: true},
		{title: "refactor/engine: clean", valid: true},
		{title: "invalid: test", valid: false},
		{title: "feat: missing category", valid: false},
		{title: "feat/engine missing colon", valid: false},
		{title: "feat/engine: too long " + strings.Repeat("a", 50), valid: false},
		{title: "feat/engine: trailing period.", valid: false},
		{title: "", valid: false},
	}

	for _, tt := range tests {
		result := IsValidIssueTitle(tt.title)
		if result != tt.valid {
			t.Errorf("IsValidIssueTitle(%q) = %v, want %v", tt.title, result, tt.valid)
		}
	}
}