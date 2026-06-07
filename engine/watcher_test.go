package engine

import (
	"testing"
)

// TestContainsUtility verifies that our path comparison helper correctly
// determines if a string slice contains a specific target directory name token.
func TestContainsUtility(t *testing.T) {
	excludeList := []string{".git", "forgefix", "node_modules"}

	// 1. Assert true matches are correctly isolated
	if !contains(excludeList, "forgefix") {
		t.Error("expected contains utility to return true for 'forgefix', got false")
	}

	// 2. Assert unexpected directories return false cleanly
	if contains(excludeList, "backend") {
		t.Error("expected contains utility to return false for 'backend', got true")
	}
}
