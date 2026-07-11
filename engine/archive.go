package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Clean up orphaned spec files — files whose spec_id is not in the ledger
	// (ledger entry was already removed by a previous archive run, but the
	// file was left behind because ArchiveSpec used the wrong filename).
	orphaned := cleanupOrphanFiles(configDir, ledger)
	if orphaned > 0 {
		fmt.Fprintf(os.Stderr, "Removed %d orphaned spec file(s) left by previous archive runs.\n", orphaned)
	}

	if archived == 0 && orphaned == 0 {
		return "", 0, nil
	}

	if err := SaveLedger(ledger, configDir); err != nil {
		return "", 0, fmt.Errorf("saving ledger: %w", err)
	}

	msg := fmt.Sprintf("archived %d specs to database", archived)
	if orphaned > 0 {
		msg += fmt.Sprintf("; cleaned %d orphaned files", orphaned)
	}
	return msg, archived, nil
}

// cleanupOrphanFiles removes spec files whose spec_id has no matching entry
// in the ledger. These are files left behind when a previous archive run
// deleted the ledger entry but failed to remove the file.
func cleanupOrphanFiles(configDir string, ledger *LedgerEngine) int {
	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return 0
	}
	var cleaned int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFileForCommit(path)
		if err != nil || spec.SpecID == "" {
			continue
		}
		// If the spec_id is not in the ledger, the entry was already removed.
		if ledger.GetSpecEntry(spec.SpecID) == nil {
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove orphaned file %s: %v\n", entry.Name(), err)
			} else {
				cleaned++
			}
		}
	}
	return cleaned
}

// DeprecatedArchiveExists checks whether the old specs/archive/ directory still
// exists (before ImportArchiveFiles clears it). Used for informational messages.
func DeprecatedArchiveExists(configDir string) bool {
	archiveDir := filepath.Join(configDir, "specs", "archive")
	_, err := os.Stat(archiveDir)
	return err == nil
}
