package engine

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidIssueTitle = errors.New("invalid issue title format")

	issueTitleRegex = regexp.MustCompile(`^(\[[a-z][a-z0-9-]*\])+ .+$`)
	maxTitleLength  = 120
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

	if strings.HasSuffix(title, ".") || strings.HasSuffix(title, "!") || strings.HasSuffix(title, "?") || strings.HasSuffix(title, "...") {
		return fmt.Errorf("trailing punctuation: %w", ErrInvalidIssueTitle)
	}

	if !v.regex.MatchString(title) {
		return fmt.Errorf("regex mismatch for %q: %w", title, ErrInvalidIssueTitle)
	}

	if len(title) > maxTitleLength {
		return fmt.Errorf("title too long (%d > %d): %w", len(title), maxTitleLength, ErrInvalidIssueTitle)
	}

	return nil
}

func IsValidIssueTitle(title string) bool {
	validator := NewIssueTitleValidator()
	return validator.Validate(title) == nil
}
