package engine

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const DuplicateThreshold = 0.7

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	m, n := len(r1), len(r2)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func SimilarityRatio(s1, s2 string) float64 {
	distance := levenshteinDistance(s1, s2)
	maxLen := math.Max(float64(len([]rune(s1))), float64(len([]rune(s2))))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(distance)/maxLen
}

func NormalizeTitle(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	words := strings.Fields(normalized)
	return strings.Join(words, " ")
}

type SpecInfo struct {
	SpecID   string
	Title    string
	FilePath string
}

func listSpecs(configDir string) ([]SpecInfo, error) {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var specs []SpecInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil {
			continue
		}
		if spec.SpecID == "" || spec.Title == "" {
			continue
		}
		specs = append(specs, SpecInfo{SpecID: spec.SpecID, Title: spec.Title, FilePath: filePath})
	}
	return specs, nil
}

func FindDuplicateSpec(configDir, newTitle string) (string, string, bool) {
	existing, err := listSpecs(configDir)
	if err != nil || len(existing) == 0 {
		return "", "", false
	}

	normalizedNew := NormalizeTitle(newTitle)
	if normalizedNew == "" {
		return "", "", false
	}

	var bestMatch SpecInfo
	bestSimilarity := DuplicateThreshold

	for _, s := range existing {
		normalizedExisting := NormalizeTitle(s.Title)
		sim := SimilarityRatio(normalizedNew, normalizedExisting)
		if sim > bestSimilarity {
			bestSimilarity = sim
			bestMatch = s
		}
	}

	if bestMatch.SpecID != "" {
		return bestMatch.SpecID, bestMatch.Title, true
	}
	return "", "", false
}

// FindSpecByID scans all spec files for an exact spec_id match.
// Returns the matching spec's SpecID, Title, file path, and true if found.
func FindSpecByID(configDir, specID string) (string, string, string, bool) {
	existing, err := listSpecs(configDir)
	if err != nil || len(existing) == 0 {
		return "", "", "", false
	}

	for _, s := range existing {
		if s.SpecID == specID {
			return s.SpecID, s.Title, s.FilePath, true
		}
	}
	return "", "", "", false
}

func MarkSpecBodyAsDuplicate(body, originalSpecID string) string {
	ref := fmt.Sprintf("\n\n> This spec has been identified as a duplicate of `%s`.", originalSpecID)
	return body + ref
}
