package engine

import (
	"os"
	"path/filepath"
)

// ============================================================================
// LEDGER FILE MANAGER
// ============================================================================

func ledgerPath(configDir string) string {
	return filepath.Join(configDir, ".forgefix_ledger.json")
}

func LoadLedger(configDir string) (*LedgerEngine, error) {
	ledger := NewLedgerEngine()
	path := ledgerPath(configDir)
	_, err := os.Stat(path)
	if err == nil {
		_ = ledger.LoadFromFile(path)
	}
	return ledger, nil
}

func SaveLedger(ledger *LedgerEngine, configDir string) error {
	return ledger.SaveToFile(ledgerPath(configDir))
}
