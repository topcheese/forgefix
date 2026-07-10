package engine

import (
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveResolvedSpecs archives all closed/resolved specs to the DB and
// removes their spec files. Returns the count of archived specs.
func ArchiveResolvedSpecs(configDir string) (string, int, error) {
	ledger, err := LoadLedger(configDir)
	if err != nil {
		return "", 0, fmt.Errorf("loading ledger: %w", err)
	}

	db, err := OpenDB(configDir)
	if err != nil {
		return "", 0, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Import any existing file-based archives into the DB first.
	if err := ImportArchiveFiles(configDir, db); err != nil {
		fmt.Fprintf(os.Stderr, "warning: importing archive files: %v\n", err)
	}

	var archived int
	for specID, entry := range ledger.GetAllSpecEntries() {
		if entry.Status != "closed" && entry.Status != "resolved" {
			continue
		}
		specType := entry.Type
		if specType == "" {
			specType = "feature"
		}
		if err := db.ArchiveSpec(specID, entry.SpecID, specType, entry.RepoIssueID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to archive spec %s: %v\n", specID, err)
			continue
		}
		ledger.DeleteSpecEntry(specID)
		archived++
	}

	if archived == 0 {
		return "", 0, nil
	}

	if err := SaveLedger(ledger, configDir); err != nil {
		return "", 0, fmt.Errorf("saving ledger: %w", err)
	}

	return fmt.Sprintf("archived %d specs to database", archived), archived, nil
}

// DeprecatedArchiveExists checks whether the old specs/archive/ directory still
// exists (before ImportArchiveFiles clears it). Used for informational messages.
func DeprecatedArchiveExists(configDir string) bool {
	archiveDir := filepath.Join(configDir, "specs", "archive")
	_, err := os.Stat(archiveDir)
	return err == nil
}
