package engine

import (
	"strings"
	"testing"
)

func TestIssueTitleValidator_ValidTypes(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"[feat][draft] add new dashboard renderer",
		"[fix][review] resolve issue closing logic",
		"[docs][done] update setup guide",
		"[refactor][draft] simplify issue coordinator",
		"[feat][in-progress] add multi-backend support",
		"[fix][review] handle 404 errors",
		"[ops][draft] maintenance cleanup",
		"[chore][draft] update dependencies",
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
		"test/sync: add tests",
		"invalid/engine: test",
		"FEAT/engine: test",
		"Fix/sync: test",
		"old format title without brackets",
		"feat[draft] missing opening bracket",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error", tt)
		}
	}
}

func TestIssueTitleValidator_MissingBrackets(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"feat: missing brackets",
		"fix: no brackets here",
		"docs: missing",
		"refactor:",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for missing brackets", tt)
		}
	}
}

func TestIssueTitleValidator_MalformedBrackets(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"[feat missing closing bracket",
		"feat] missing opening bracket",
		"[fix[draft] nested bracket",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for malformed brackets", tt)
		}
	}
}

func TestIssueTitleValidator_TooLong(t *testing.T) {
	validator := NewIssueTitleValidator()

	longTitle := "[feat][draft] " + strings.Repeat("a", 120)
	if len(longTitle) <= 120 {
		t.Fatal("test setup error: title not long enough")
	}

	err := validator.Validate(longTitle)
	if err == nil {
		t.Errorf("Validate(long title) = nil, want error for >120 chars")
	}
}

func TestIssueTitleValidator_Exactly120Chars(t *testing.T) {
	validator := NewIssueTitleValidator()

	prefix := "[feat][draft] "
	exactTitle := prefix + strings.Repeat("a", 120-len(prefix))
	if len(exactTitle) != 120 {
		t.Fatalf("test setup error: title length is %d, want 120", len(exactTitle))
	}

	err := validator.Validate(exactTitle)
	if err != nil {
		t.Errorf("Validate(120 chars) = %v, want nil", err)
	}
}

func TestIssueTitleValidator_TrailingPunctuation(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"[feat][draft] add new feature.",
		"[fix][review] resolve bug!",
		"[docs][done] update guide?",
		"[refactor][draft] simplify code...",
		"[feat][draft] test...",
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

func TestIssueTitleValidator_InvalidBracketContent(t *testing.T) {
	validator := NewIssueTitleValidator()

	tests := []string{
		"[FEAT][draft] uppercase in bracket",
		"[feat@v1][draft] special char in bracket",
		"[feat draft] space in single bracket",
	}

	for _, tt := range tests {
		err := validator.Validate(tt)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want error for invalid bracket content", tt)
		}
	}
}

func TestIsValidIssueTitle_Helper(t *testing.T) {
	tests := []struct {
		title string
		valid bool
	}{
		{title: "[feat][draft] add feature", valid: true},
		{title: "[fix][review] fix bug", valid: true},
		{title: "[docs][done] update", valid: true},
		{title: "[refactor][draft] clean", valid: true},
		{title: "[ops][draft] maintenance", valid: true},
		{title: "[chore][draft] update deps", valid: true},
		{title: "invalid: test", valid: false},
		{title: "feat: missing brackets", valid: false},
		{title: "feat/engine: missing brackets", valid: false},
		{title: "[feat][draft] too long " + strings.Repeat("a", 120), valid: false},
		{title: "[feat][draft] trailing period.", valid: false},
		{title: "", valid: false},
	}

	for _, tt := range tests {
		result := IsValidIssueTitle(tt.title)
		if result != tt.valid {
			t.Errorf("IsValidIssueTitle(%q) = %v, want %v", tt.title, result, tt.valid)
		}
	}
}
