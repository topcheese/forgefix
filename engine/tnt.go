package engine

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// ============================================================================
// [TNT] SCANNER ENGINE
// ============================================================================
// Tokenizing and fingerprinting local files with a thread-safe registry
// that associates code chunk hashes with their native file paths and row coordinates.
// ============================================================================

// CodeChunk represents a normalized code segment ready for fingerprinting
type CodeChunk struct {
	Content string
	Row     int
	Col     int
}

// FingerprintRegistry provides thread-safe hash-to-location mapping
type FingerprintRegistry struct {
	mu       sync.RWMutex
	registry map[string]string // hash -> "filepath:row:col"
}

// NewFingerprintRegistry initializes an empty registry
func NewFingerprintRegistry() *FingerprintRegistry {
	return &FingerprintRegistry{
		registry: make(map[string]string),
	}
}

// Register adds a code chunk hash to the registry
func (r *FingerprintRegistry) Register(hash string, filepath string, row, col int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := fmt.Sprintf("%s:%d:%d", filepath, row, col)
	r.registry[hash] = entry
}

// Get retrieves the file path and coordinates for a given hash
func (r *FingerprintRegistry) Get(hash string) (string, int, int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, exists := r.registry[hash]
	if !exists {
		return "", 0, 0, false
	}
	// Parse "filepath:row:col" format
	parts := strings.Split(entry, ":")
	if len(parts) < 3 {
		return "", 0, 0, false
	}

	var row, col int
	fmt.Sscanf(parts[1], "%d", &row)
	fmt.Sscanf(parts[2], "%d", &col)

	return parts[0], row, col, true
}

// GetAll returns all registered hashes
func (r *FingerprintRegistry) GetAll() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := make(map[string]string, len(r.registry))
	for k, v := range r.registry {
		clone[k] = v
	}
	return clone
}

// Clear removes all entries from the registry
func (r *FingerprintRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registry = make(map[string]string)
}

// WindowSize defines how many consecutive lines form a single structural code fingerprint
const WindowSize = 3

// ScanAndRegister reads a file, normalizes it, splits it into line windows, and registers each chunk.
func (r *FingerprintRegistry) ScanAndRegister(filePath string) error {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return err
	}

	normalized := NormalizeCode(string(data))
	lines := strings.Split(normalized, "\n")
	if len(lines) < WindowSize {
		// File is smaller than our window; fallback to registering the whole content block
		hash := CalculateHash(normalized)
		r.Register(hash, filePath, 1, 0)
		return nil
	}

	// Sliding window segmentation pass
	for i := 0; i <= len(lines)-WindowSize; i++ {
		windowContent := strings.Join(lines[i:i+WindowSize], "\n")
		if strings.TrimSpace(windowContent) == "" {
			continue
		}

		chunk := CodeChunk{
			Content: windowContent,
			Row:     i + 1,
			Col:     0,
		}
		hash := CalculateChunkHash(chunk)
		r.Register(hash, filePath, chunk.Row, chunk.Col)
	}
	return nil
}

// ScanAndCheck reads a file, extracts its line windows, and checks if any sub-chunk
// matches a previously registered fingerprint. Returns true and the original file path if a collision occurs.
func (r *FingerprintRegistry) ScanAndCheck(filePath string) (bool, string, error) {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return false, "", err
	}

	normalized := NormalizeCode(string(data))
	lines := strings.Split(normalized, "\n")
	if len(lines) < WindowSize {
		hash := CalculateHash(normalized)
		originalPath, _, _, found := r.Get(hash)
		return found, originalPath, nil
	}

	// Slide across target file to inspect internal fragments
	for i := 0; i <= len(lines)-WindowSize; i++ {
		windowContent := strings.Join(lines[i:i+WindowSize], "\n")
		if strings.TrimSpace(windowContent) == "" {
			continue
		}

		chunk := CodeChunk{Content: windowContent}
		hash := CalculateChunkHash(chunk)

		if originalPath, _, _, found := r.Get(hash); found {
			// Guard against self-collision matching if scanning the same file or updates
			if originalPath != filePath {
				return true, originalPath, nil
			}
		}
	}
	return false, "", nil
}

// ============================================================================
// NORMALIZATION FILTER
// ============================================================================
// Strips line comments and flushes irregular white spaces before hashing
// ============================================================================

var (
	// Line comment patterns for various languages
	lineCommentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\s*//.*$`), // C, C++, Java, JavaScript, TypeScript, Go
		regexp.MustCompile(`^\s*#.*$`),  // Python, Ruby, Shell, Bash
		regexp.MustCompile(`^\s*//.*$`), // PHP
	}

	// Whitespace normalization pattern - collapses multiple spaces/tabs
	whitespacePattern = regexp.MustCompile(`\s+`)
)

// NormalizeCode removes line comments and normalizes whitespace
func NormalizeCode(code string) string {
	lines := strings.Split(code, "\n")
	var normalized []string
	var prevBlank bool

	for _, line := range lines {
		// Check if original line is blank (only whitespace)
		isBlank := strings.TrimSpace(line) == ""

		// Strip line comments
		normalizedLine := stripLineComment(line)

		// Check if line is blank after stripping comments
		isBlankAfterStrip := strings.TrimSpace(normalizedLine) == ""

		// Only preserve blank lines that were originally blank
		if isBlank {
			if !prevBlank {
				normalized = append(normalized, "")
			}
			prevBlank = true
			continue
		}
		prevBlank = false

		// If line became blank after stripping comments, skip it entirely
		if isBlankAfterStrip {
			continue
		}

		// Normalize whitespace: collapse multiple spaces to single space
		// but preserve tabs in indentation
		normalizedLine = normalizeWhitespace(normalizedLine)

		// Trim trailing whitespace
		normalizedLine = strings.TrimRight(normalizedLine, " ")

		if normalizedLine != "" {
			normalized = append(normalized, normalizedLine)
		}
	}

	return strings.Join(normalized, "\n")
}

// normalizeWhitespace collapses multiple spaces to single space and converts tabs to single spaces
// Tabs at the beginning of a line (indentation) are preserved as tabs
// Also handles escaped tab sequences (\t) by converting them to spaces
func normalizeWhitespace(line string) string {
	// First, replace escaped tab sequences with actual tabs
	line = strings.ReplaceAll(line, "\\t", "\t")

	var result strings.Builder
	inIndentation := true
	lastWasSpace := false

	for _, r := range line {
		if r == '\t' {
			if inIndentation {
				// Preserve tab-based indentation
				result.WriteRune(r)
			} else {
				// Convert tabs to single space in the middle of lines
				result.WriteRune(' ')
				lastWasSpace = true
			}
		} else if r == ' ' {
			if inIndentation {
				// Preserve leading whitespace (indentation)
				result.WriteRune(r)
				lastWasSpace = true
			} else {
				// Collapse multiple spaces to single space
				if !lastWasSpace {
					result.WriteRune(r)
					lastWasSpace = true
				}
			}
		} else {
			// Non-whitespace character
			inIndentation = false
			lastWasSpace = false
			result.WriteRune(r)
		}
	}
	return result.String()
}

// stripLineComment removes line comments from a single line, preserving leading whitespace
func stripLineComment(line string) string {
	trimmed := strings.TrimSpace(line)
	// Handle Python-style comments (#)
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		return line[:idx]
	}
	// Handle Go/C/Java/JS-style comments (//)
	if idx := strings.Index(trimmed, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

// ============================================================================
// HASHING
// ============================================================================

// CalculateHash computes a fingerprint for normalized code content
func CalculateHash(content string) string {
	hash := fnv.New32a()
	hash.Write([]byte(content))
	return fmt.Sprintf("%08x", hash.Sum32())
}

// CalculateChunkHash computes a fingerprint for a specific code chunk
func CalculateChunkHash(chunk CodeChunk) string {
	normalized := NormalizeCode(chunk.Content)
	hash := fnv.New32a()
	hash.Write([]byte(normalized))
	return fmt.Sprintf("%08x", hash.Sum32())
}
