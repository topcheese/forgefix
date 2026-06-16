package engine

import (
	"errors"
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
	}

	issueTitleRegex = regexp.MustCompile(`^(feat|fix|docs|refactor)/[a-z0-9-]+: [^.!?]+$`)
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
		return ErrInvalidIssueTitle
	}

	if len(title) > maxTitleLength {
		return ErrInvalidIssueTitle
	}

	if strings.HasSuffix(title, ".") || strings.HasSuffix(title, "!") || strings.HasSuffix(title, "?") {
		return ErrInvalidIssueTitle
	}

	if !v.regex.MatchString(title) {
		return ErrInvalidIssueTitle
	}

	parts := strings.SplitN(title, "/", 2)
	if len(parts) != 2 {
		return ErrInvalidIssueTitle
	}

	issueType := parts[0]
	if !allowedTypes[issueType] {
		return ErrInvalidIssueTitle
	}

	return nil
}

func IsValidIssueTitle(title string) bool {
	validator := NewIssueTitleValidator()
	return validator.Validate(title) == nil
}