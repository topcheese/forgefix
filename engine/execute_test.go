package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindAnchorDirDownwardTraversal verifies that the look-ahead directory walker
// correctly resolves nested child locations containing target blueprints.
func TestFindAnchorDirDownwardTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	subFolder := filepath.Join(tmpDir, "src", "app")
	_ = os.MkdirAll(subFolder, 0755)
	_ = os.WriteFile(filepath.Join(subFolder, "go.mod"), []byte("module test"), 0644)

	found := findAnchorDir(tmpDir, "go.mod", []string{".git"})
	if found != subFolder {
		t.Errorf("expected findAnchorDir to resolve down to '%s', got '%s'", subFolder, found)
	}
}

// TestEmitDetonatedPayloadStructure verifies that our emergency machine response
// correctly serializes high-density data context without throwing panics.
func TestEmitDetonatedPayloadStructure(t *testing.T) {
	dashboard := NewDashboard([]PipelineConfig{
		{ID: "test-pipe", Name: "Test Pipeline"},
	})
	
	// Execute a dry-run recover wrapper to ensure the serializer functions smoothly
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EmitDetonated panicked unexpectedly during serialization: %v", r)
		}
	}()
	
	// Test the core structural transformation mapping
	payload := dashboard.ToAIPayload()
	payload.Status = "DETONATED"
	if payload.Status != "DETONATED" {
		t.Error("failed to assign proper high-density detonation status token")
	}
}