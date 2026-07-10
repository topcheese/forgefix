package engine

import "testing"

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
