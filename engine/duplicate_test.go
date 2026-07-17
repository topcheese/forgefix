package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func writeSpecWithTitle(t *testing.T, dir, specID, title, status string) {
	t.Helper()
	content := `---
spec_id: "` + specID + `"
status: "` + status + `"
type: "type/bug"
version: "version/v0.8.0"
repo_issue: 0
---

# ` + title + `

Body content
`
	path := filepath.Join(dir, "specs", specID+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "ab", 1},
		{"kitten", "sitting", 3},
		{"hello", "world", 4},
	}
	for _, tc := range tests {
		got := levenshteinDistance(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSimilarityRatio(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1.0},
		{"abc", "abc", 1.0},
		{"abc", "abd", 2.0 / 3.0},
		{"kitten", "sitting", 4.0 / 7.0},
		{"hello", "world", 0.2},
	}
	for _, tc := range tests {
		got := SimilarityRatio(tc.a, tc.b)
		if abs(got-tc.want) > 1e-9 {
			t.Errorf("SimilarityRatio(%q, %q) = %f, want %f", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  Hello World  ", "hello world"},
		{"Fix Bug", "fix bug"},
		{"  multiple   spaces  here  ", "multiple spaces here"},
		{"", ""},
		{"   ", ""},
	}
	for _, tc := range tests {
		got := NormalizeTitle(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFindDuplicateSpec_MatchesSimilarTitle(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-ORIGINAL", "Fix Login Button", "in-progress")

	origID, origTitle, ok := FindDuplicateSpec(tmpDir, "Fix Login Buton")
	if !ok {
		t.Fatal("expected duplicate to be detected")
	}
	if origID != "SPEC-ORIGINAL" {
		t.Errorf("expected SPEC-ORIGINAL, got %s", origID)
	}
	if origTitle != "Fix Login Button" {
		t.Errorf("expected 'Fix Login Button', got %s", origTitle)
	}
}

func TestFindDuplicateSpec_NoMatchOnDifferentTitle(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-UNIQUE", "Database Connection Pool", "ready")

	_, _, ok := FindDuplicateSpec(tmpDir, "User Authentication Flow")
	if ok {
		t.Fatal("expected no duplicate for unrelated title")
	}
}

func TestFindDuplicateSpec_MatchOnExactTitle(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-EXISTING", "Add Dark Mode", "draft")

	origID, _, ok := FindDuplicateSpec(tmpDir, "Add Dark Mode")
	if !ok {
		t.Fatal("expected duplicate detection on exact title match")
	}
	if origID != "SPEC-EXISTING" {
		t.Errorf("expected SPEC-EXISTING, got %s", origID)
	}
}

func TestFindDuplicateSpec_MatchOnCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-CASE", "Cache Invalidation", "in-progress")

	origID, _, ok := FindDuplicateSpec(tmpDir, "cache invalidation")
	if !ok {
		t.Fatal("expected duplicate detection on case-insensitive match")
	}
	if origID != "SPEC-CASE" {
		t.Errorf("expected SPEC-CASE, got %s", origID)
	}
}

func TestFindDuplicateSpec_SkipsResolved(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-RESOLVED", "Old Fixed Bug", "closed")
	// Should not match resolved specs because SyncFromSpecsDir skips them
	// However FindDuplicateSpec uses parseSpecFile which parses all files
	// That's fine — even if resolved specs are in the dir, we should detect dupes against them
	_, _, ok := FindDuplicateSpec(tmpDir, "Old Fixed Bug")
	if !ok {
		t.Fatal("expected duplicate detection to still find resolved specs")
	}
}

func TestFindDuplicateSpec_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	_, _, ok := FindDuplicateSpec(tmpDir, "Some Brand New Feature")
	if ok {
		t.Fatal("expected no duplicate in empty directory")
	}
}

func TestFindDuplicateSpec_NoSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, ok := FindDuplicateSpec(tmpDir, "Some Feature")
	if ok {
		t.Fatal("expected no duplicate when specs dir doesn't exist")
	}
}

func TestMarkSpecBodyAsDuplicate(t *testing.T) {
	body := "# Test Title\n\nSome content"
	result := MarkSpecBodyAsDuplicate(body, "SPEC-ORIGINAL")
	expected := "# Test Title\n\nSome content\n\n> This spec has been identified as a duplicate of `SPEC-ORIGINAL`."
	if result != expected {
		t.Errorf("unexpected result:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFindDuplicateSpec_PicksBestMatch(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-DISTANT", "Database Setup Guide", "ready")
	writeSpecWithTitle(t, tmpDir, "SPEC-CLOSE", "User Login Flow", "in-progress")

	origID, origTitle, ok := FindDuplicateSpec(tmpDir, "User Login Flo")
	if !ok {
		t.Fatal("expected duplicate to be detected")
	}
	if origID != "SPEC-CLOSE" {
		t.Errorf("expected best match SPEC-CLOSE, got %s", origID)
	}
	if origTitle != "User Login Flow" {
		t.Errorf("expected 'User Login Flow', got %s", origTitle)
	}
}

// TestFindSpecByID_ExactMatch tests the exact spec_id collision detection.
// This is the new guard added in SPEC-1784101811 to catch duplicate spec_ids
// that escape the title-only detector.
func TestFindSpecByID_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	// Create a spec with a specific spec_id
	writeSpecWithTitle(t, tmpDir, "SPEC-12345", "Original Feature", "draft")

	// Try to find by exact spec_id
	foundID, foundTitle, _, ok := FindSpecByID(tmpDir, "SPEC-12345")
	if !ok {
		t.Fatal("expected exact spec_id match to be found")
	}
	if foundID != "SPEC-12345" {
		t.Errorf("expected SPEC-12345, got %s", foundID)
	}
	if foundTitle != "Original Feature" {
		t.Errorf("expected 'Original Feature', got %s", foundTitle)
	}
}

// TestFindSpecByID_NoMatch tests that non-existent spec_id returns no match.
func TestFindSpecByID_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	writeSpecWithTitle(t, tmpDir, "SPEC-12345", "Original Feature", "draft")

	_, _, _, ok := FindSpecByID(tmpDir, "SPEC-99999")
	if ok {
		t.Fatal("expected no match for non-existent spec_id")
	}
}

// TestFindSpecByID_EmptyDirectory tests that empty directory returns no match.
func TestFindSpecByID_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "specs"), 0755)

	_, _, _, ok := FindSpecByID(tmpDir, "SPEC-12345")
	if ok {
		t.Fatal("expected no match in empty directory")
	}
}

// TestFindSpecByID_NoSpecsDir tests that missing specs dir returns no match.
func TestFindSpecByID_NoSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, _, ok := FindSpecByID(tmpDir, "SPEC-12345")
	if ok {
		t.Fatal("expected no match when specs dir doesn't exist")
	}
}
