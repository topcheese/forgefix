package engine

import "testing"

func TestParseFlags_Body(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no body flag", []string{}, ""},
		{"body with value", []string{"--body", "commit body text"}, "commit body text"},
		{"body after message flag", []string{"-m", "feat: msg", "--body", "body text"}, "body text"},
		{"body preserves newlines", []string{"--body", "line1\nline2"}, "line1\nline2"},
		{"body with multiline text", []string{"--body", "Summary: fix race condition\n\nDetails: The mutex was not held during read"}, "Summary: fix race condition\n\nDetails: The mutex was not held during read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := ParseFlags(tt.args)
			if flags.Body != tt.want {
				t.Errorf("ParseFlags(%q).Body = %q, want %q", tt.args, flags.Body, tt.want)
			}
		})
	}
}

func TestParseFlags_BodyNotSanitized(t *testing.T) {
	// --body does NOT go through SanitizeMessage so newlines, \r are preserved.
	// Contrast: --message goes through SanitizeMessage which strips them.
	bodyArg := "line1\nline2\r\nline3"
	msgArg := "line1\nline2\r\nline3"

	flags := ParseFlags([]string{"--body", bodyArg, "--message", msgArg})

	// Body should preserve newlines
	if flags.Body != bodyArg {
		t.Errorf("Body = %q, want %q (newlines should be preserved)", flags.Body, bodyArg)
	}
	// Message should have newlines stripped by SanitizeMessage
	if flags.Message == msgArg {
		t.Errorf("Message should have been sanitized (newlines stripped), but was %q", flags.Message)
	}
	if flags.Message != "line1line2line3" {
		t.Errorf("Message = %q, want %q", flags.Message, "line1line2line3")
	}
}

func TestParseFlags_RootCause(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no root cause flag", []string{}, ""},
		{"root cause with value", []string{"--root-cause", "null pointer dereference"}, "null pointer dereference"},
		{"root cause with complex value", []string{"--root-cause", "race condition in mutex lock"}, "race condition in mutex lock"},
		{"root cause coexists with type", []string{"--type", "bug", "--root-cause", "missing nil check"}, "missing nil check"},
		{"root cause before type", []string{"--root-cause", "buffer overflow", "--type", "bug"}, "buffer overflow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := ParseFlags(tt.args)
			if flags.SpecRootCause != tt.want {
				t.Errorf("ParseFlags(%q).SpecRootCause = %q, want %q", tt.args, flags.SpecRootCause, tt.want)
			}
		})
	}
}

func TestParseFlags_RootCauseNotConfusedWithBody(t *testing.T) {
	flags := ParseFlags([]string{"--root-cause", "memory leak", "--body", "fix memory allocation"})
	if flags.SpecRootCause != "memory leak" {
		t.Errorf("SpecRootCause = %q, want %q", flags.SpecRootCause, "memory leak")
	}
	if flags.Body != "fix memory allocation" {
		t.Errorf("Body = %q, want %q", flags.Body, "fix memory allocation")
	}
}

func TestParseFlags_MessageStillSanitized(t *testing.T) {
	// Regression: --message should continue to strip newlines.
	// SanitizeMessage strips \n characters without adding any separator.
	flags := ParseFlags([]string{"--message", "fix: resolve\ntimeout issue"})
	want := "fix: resolvetimeout issue"
	if flags.Message != want {
		t.Errorf("Message = %q, want %q", flags.Message, want)
	}
}
