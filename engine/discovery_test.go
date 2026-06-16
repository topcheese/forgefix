package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjectRootFindsAnchor(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock project structure with excluded dirs
	excludedDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(excludedDir, 0755); err != nil {
		t.Fatalf("failed to create excluded dir: %v", err)
	}

	// Create a go.mod in a subdirectory (the real anchor)
	projectDir := filepath.Join(tmpDir, "myproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	anchorPath := filepath.Join(projectDir, "go.mod")
	if err := os.WriteFile(anchorPath, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Test discovery from temp root
	found := findAnchorDir(tmpDir, "go.mod", []string{"vendor"})
	if found != projectDir {
		t.Errorf("expected to find project at %s, got %s", projectDir, found)
	}
}

func TestDiscoverProjectRootSkipsExcludedDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create excluded dir with a fake anchor (should be skipped)
	excludedDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(excludedDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	fakeAnchor := filepath.Join(excludedDir, "go.mod")
	if err := os.WriteFile(fakeAnchor, []byte("module fake\n"), 0644); err != nil {
		t.Fatalf("failed to write fake go.mod: %v", err)
	}

	// Create real project elsewhere
	projectDir := filepath.Join(tmpDir, "realproject")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create real project dir: %v", err)
	}
	realAnchor := filepath.Join(projectDir, "go.mod")
	if err := os.WriteFile(realAnchor, []byte("module real\n"), 0644); err != nil {
		t.Fatalf("failed to write real go.mod: %v", err)
	}

	// Should find the real project, not the one in excluded dir
	found := findAnchorDir(tmpDir, "go.mod", []string{".git"})
	if found != projectDir {
		t.Errorf("expected to find real project at %s, got %s (excluded dir not skipped)", projectDir, found)
	}
}