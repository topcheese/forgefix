package engine

import (
	"fmt"
	"sync"
	"testing"
)

// ============================================================================
// [TNT] SCANNER ENGINE UNIT TESTS
// ============================================================================

func TestNormalizeCode_StripsLineComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips_go_style_comment",
			input:    "package main\n// this is a comment\nfunc main() {}",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "strips_python_style_comment",
			input:    "def hello():\n# this is a comment\n    print('hi')",
			expected: "def hello():\n    print('hi')",
		},
		{
			name:     "strips_multiple_comment_types",
			input:    "// go comment\n# python comment\npackage main",
			expected: "package main",
		},
		{
			name:     "strips_comment_with_leading_whitespace",
			input:    "  // indented comment\n\t# tabbed comment",
			expected: "",
		},
		{
			name:     "strips_comment_at_end_of_code",
			input:    "func add(a, b int) int { return a + b } // inline comment",
			expected: "func add(a, b int) int { return a + b }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeCode(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeCode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeCode_CollapsesWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "collapses_multiple_spaces",
			input:    "func  add(a,  b  int)  int  {  return  a  +  b  }",
			expected: "func add(a, b int) int { return a + b }",
		},
		{
			name:     "collapses_tabs_and_spaces",
			input:    "func\tadd(a,\tb\tint)\tint\t{\treturn\ta\t+\tb\t}",
			expected: "func add(a, b int) int { return a + b }",
		},
		{
			name:     "collapses_mixed_whitespace",
			input:    "func  add(a,\t b\tint) int { return a + b }",
			expected: "func add(a, b int) int { return a + b }",
		},
		{
			name:     "preserves_single_space",
			input:    "func add(a, b int) int { return a + b }",
			expected: "func add(a, b int) int { return a + b }",
		},
		{
			name:     "collapses_whitespace_in_jagged_lines",
			input:    "func  add(a,\t b\tint)  int  {\n\t\treturn  a  +  b\n}",
			expected: "func add(a, b int) int {\n\t\treturn a + b\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeCode(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeCode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeCode_Comprehensive(t *testing.T) {
	input := `package main

// This is a go comment
# This is a python comment
func  add(a,\t b\tint)  int  {
	// inline comment
	return  a  +  b
}

# Another comment
var x = 10 // trailing comment`

	expected := `package main

func add(a, b int) int {
	return a + b
}

var x = 10`

	result := NormalizeCode(input)
	if result != expected {
		t.Errorf("NormalizeCode() = %q, want %q", result, expected)
	}
}

func TestCalculateHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: "00000000",
		},
		{
			name:     "simple_string",
			input:    "hello",
			expected: "12345678", // placeholder - actual hash will vary
		},
		{
			name:     "code_snippet",
			input:    "func main() {}",
			expected: "87654321", // placeholder - actual hash will vary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := CalculateHash(tt.input)
			if len(hash) != 8 {
				t.Errorf("CalculateHash() returned %q, expected 8 hex chars", hash)
			}
		})
	}
}

func TestCalculateChunkHash(t *testing.T) {
	chunk := CodeChunk{
		Content: "func add(a, b int) int { return a + b }",
		Row:     10,
		Col:     5,
	}

	hash := CalculateChunkHash(chunk)
	if len(hash) != 8 {
		t.Errorf("CalculateChunkHash() returned %q, expected 8 hex chars", hash)
	}
}

func TestFingerprintRegistry_RegisterAndGet(t *testing.T) {
	registry := NewFingerprintRegistry()

	// Register some entries
	registry.Register("hash1", "/path/to/file.go", 10, 5)
	registry.Register("hash2", "/path/to/other.go", 20, 15)

	// Test retrieval
	filepath, row, col, exists := registry.Get("hash1")
	if !exists {
		t.Error("Expected hash1 to exist")
	}
	if filepath != "/path/to/file.go" {
		t.Errorf("Expected filepath %q, got %q", "/path/to/file.go", filepath)
	}
	if row != 10 {
		t.Errorf("Expected row 10, got %d", row)
	}
	if col != 5 {
		t.Errorf("Expected col 5, got %d", col)
	}

	// Test non-existent hash
	_, _, _, exists = registry.Get("nonexistent")
	if exists {
		t.Error("Expected nonexistent hash to not exist")
	}
}

func TestFingerprintRegistry_GetAll(t *testing.T) {
	registry := NewFingerprintRegistry()

	registry.Register("hash1", "/path/to/file.go", 10, 5)
	registry.Register("hash2", "/path/to/other.go", 20, 15)

	all := registry.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(all))
	}

	if _, exists := all["hash1"]; !exists {
		t.Error("Expected hash1 in GetAll()")
	}
	if _, exists := all["hash2"]; !exists {
		t.Error("Expected hash2 in GetAll()")
	}
}

func TestFingerprintRegistry_Clear(t *testing.T) {
	registry := NewFingerprintRegistry()

	registry.Register("hash1", "/path/to/file.go", 10, 5)
	registry.Register("hash2", "/path/to/other.go", 20, 15)

	registry.Clear()

	if len(registry.GetAll()) != 0 {
		t.Error("Expected registry to be empty after Clear()")
	}
}

func TestFingerprintRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewFingerprintRegistry()
	var wg sync.WaitGroup
	const numWorkers = 10
	const numOpsPerWorker = 100

	// Register entries concurrently
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerWorker; j++ {
				hash := fmt.Sprintf("hash-%d-%d", workerID, j)
				row := j % 100
				col := j % 50
				registry.Register(hash, fmt.Sprintf("/path/to/file_%d.go", workerID), row, col)
			}
		}(i)
	}

	// Read entries concurrently
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerWorker; j++ {
				hash := fmt.Sprintf("hash-%d-%d", workerID, j)
				_, _, _, _ = registry.Get(hash)
			}
		}(i)
	}

	wg.Wait()

	// Verify all entries are present
	all := registry.GetAll()
	expectedCount := numWorkers * numOpsPerWorker
	if len(all) != expectedCount {
		t.Errorf("Expected %d entries after concurrent operations, got %d", expectedCount, len(all))
	}
}

func TestFingerprintRegistry_NoRaceCondition(t *testing.T) {
	registry := NewFingerprintRegistry()
	var wg sync.WaitGroup
	const numWorkers = 20
	const numOpsPerWorker = 50

	// Concurrent register and get operations
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerWorker; j++ {
				hash := fmt.Sprintf("hash-%d-%d", workerID, j)
				row := j % 100
				col := j % 50

				// Register
				registry.Register(hash, fmt.Sprintf("/path/to/file_%d.go", workerID), row, col)

				// Get
				_, _, _, _ = registry.Get(hash)

				// Get all
				_ = registry.GetAll()
			}
		}(i)
	}

	wg.Wait()
}

func TestCodeChunk(t *testing.T) {
	chunk := CodeChunk{
		Content: "func add(a, b int) int { return a + b }",
		Row:     42,
		Col:     10,
	}

	if chunk.Content != "func add(a, b int) int { return a + b }" {
		t.Error("CodeChunk content mismatch")
	}
	if chunk.Row != 42 {
		t.Errorf("Expected row 42, got %d", chunk.Row)
	}
	if chunk.Col != 10 {
		t.Errorf("Expected col 10, got %d", chunk.Col)
	}
}

func TestNormalizeCode_EmptyInput(t *testing.T) {
	result := NormalizeCode("")
	if result != "" {
		t.Errorf("NormalizeCode() on empty string returned %q, expected empty", result)
	}
}

func TestNormalizeCode_OnlyComments(t *testing.T) {
	input := "// comment 1\n# comment 2\n// comment 3"
	result := NormalizeCode(input)
	if result != "" {
		t.Errorf("NormalizeCode() on comments only returned %q, expected empty", result)
	}
}

func TestNormalizeCode_MultipleConsecutiveComments(t *testing.T) {
	input := "// comment 1\n// comment 2\n# comment 3\n// comment 4"
	result := NormalizeCode(input)
	if result != "" {
		t.Errorf("NormalizeCode() on consecutive comments returned %q, expected empty", result)
	}
}

func TestCalculateHash_Deterministic(t *testing.T) {
	input := "test content for hashing"
	hash1 := CalculateHash(input)
	hash2 := CalculateHash(input)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: got %q and %q", hash1, hash2)
	}
}

func TestCalculateHash_NormalizedInput(t *testing.T) {
	input1 := "func  add(a,  b  int)  int  {  return  a  +  b  }"
	input2 := "func add(a, b int) int { return a + b }"

	hash1 := CalculateHash(input1)
	hash2 := CalculateHash(input2)

	// After normalization, these should produce the same hash
	normalized1 := NormalizeCode(input1)
	normalized2 := NormalizeCode(input2)

	if normalized1 != normalized2 {
		t.Errorf("Normalized inputs should match: %q vs %q", normalized1, normalized2)
	}

	// Verify hashes are different for unnormalized input
	if hash1 == hash2 {
		t.Error("Unnormalized inputs should produce different hashes")
	}
}

func TestCalculateChunkHash_Deterministic(t *testing.T) {
	chunk1 := CodeChunk{
		Content: "test content",
		Row:     10,
		Col:     5,
	}

	chunk2 := CodeChunk{
		Content: "test content",
		Row:     10,
		Col:     5,
	}

	hash1 := CalculateChunkHash(chunk1)
	hash2 := CalculateChunkHash(chunk2)

	if hash1 != hash2 {
		t.Errorf("Chunk hash should be deterministic: got %q and %q", hash1, hash2)
	}
}

func TestCalculateChunkHash_DifferentContent(t *testing.T) {
	chunk1 := CodeChunk{
		Content: "func add(a, b int) int { return a + b }",
		Row:     10,
		Col:     5,
	}

	chunk2 := CodeChunk{
		Content: "func sub(a, b int) int { return a - b }",
		Row:     10,
		Col:     5,
	}

	hash1 := CalculateChunkHash(chunk1)
	hash2 := CalculateChunkHash(chunk2)

	if hash1 == hash2 {
		t.Error("Different content should produce different hashes")
	}
}

func TestCalculateChunkHash_DifferentCoordinates(t *testing.T) {
	chunk1 := CodeChunk{
		Content: "test content",
		Row:     10,
		Col:     5,
	}

	chunk2 := CodeChunk{
		Content: "test content",
		Row:     20,
		Col:     10,
	}

	hash1 := CalculateChunkHash(chunk1)
	hash2 := CalculateChunkHash(chunk2)

	// Hash should be the same since content is identical
	if hash1 != hash2 {
		t.Error("Same content should produce same hash regardless of coordinates")
	}
}
