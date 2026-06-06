package engine

import (
	"os"
)

// ============================================================================
// LEDGER FILE MANAGER
// ============================================================================

const (
	LedgerFilePath = ".forgefix_ledger.json"
)

func LoadLedger() (*LedgerEngine, error) {
	ledger := NewLedgerEngine()
	_, err := os.Stat(LedgerFilePath)
	if err == nil {
		_ = ledger.LoadFromFile(LedgerFilePath)
	}
	return ledger, nil
}

func SaveLedger(ledger *LedgerEngine) error {
	return ledger.SaveToFile(LedgerFilePath)
}
