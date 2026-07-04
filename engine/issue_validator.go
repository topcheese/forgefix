package engine

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidIssueTitle = errors.New("invalid issue title format")

	allowedTypes = map[string]bool{
		"feat":     true,
		"fix":      true,
		"docs":     true,
		"refactor": true,
		"ops":      true,
		"chore":    true,
	}

	issueTitleRegex = regexp.MustCompile(`^(feat|fix|docs|refactor|ops|chore)/[a-z0-9-]+: .+$`)
	maxTitleLength  = 60
)

type IssueTitleValidator struct {
	regex *regexp.Regexp
}

func NewIssueTitleValidator() *IssueTitleValidator {
	return &IssueTitleValidator{
		regex: issueTitleRegex,
	}
}

func (v *IssueTitleValidator) Validate(title string) error {
	if title == "" {
		return fmt.Errorf("empty title: %w", ErrInvalidIssueTitle)
	}

	if len(title) > maxTitleLength {
		return fmt.Errorf("title too long (%d > %d): %w", len(title), maxTitleLength, ErrInvalidIssueTitle)
	}

	if strings.HasSuffix(title, ".") || strings.HasSuffix(title, "!") || strings.HasSuffix(title, "?") {
		return fmt.Errorf("trailing punctuation: %w", ErrInvalidIssueTitle)
	}

	if !v.regex.MatchString(title) {
		return fmt.Errorf("regex mismatch for %q: %w", title, ErrInvalidIssueTitle)
	}

	parts := strings.SplitN(title, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("no slash separator: %w", ErrInvalidIssueTitle)
	}

	issueType := parts[0]
	if !allowedTypes[issueType] {
		return fmt.Errorf("invalid type %q: %w", issueType, ErrInvalidIssueTitle)
	}

	return nil
}

func IsValidIssueTitle(title string) bool {
	validator := NewIssueTitleValidator()
	return validator.Validate(title) == nil
}