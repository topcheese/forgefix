package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
)

// handleStatus displays a project health dashboard including spec counts by
// status, last sync time, pending sync failures, and ship-gate blocking specs.
func (d *CommandDispatcher) handleStatus(args []string) (CommandResult, error) {
	ledger, err := LoadLedger(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: failed to load ledger: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	cfg := ledger.WorkflowConfig
	statusColors := buildColorMap(cfg.Statuses)

	// ── Spec counts by status ──────────────────────────────────────────
	specs := ledger.GetAllSpecEntries()
	countByStatus := make(map[string]int)
	for _, s := range specs {
		countByStatus[s.Status]++
	}

	// Build ordered status list from config (preserves config order)
	type statusLine struct {
		Name  string
		Count int
	}
	var lines []statusLine
	activeTotal := 0
	for _, sd := range cfg.Statuses {
		c := countByStatus[sd.Name]
		lines = append(lines, statusLine{Name: sd.Name, Count: c})
		if sd.Active {
			activeTotal += c
		}
	}

	// ── Pipeline entry ─────────────────────────────────────────────────
	entry := ledger.GetEntry("forgefix")

	// ── Sync state ─────────────────────────────────────────────────────
	var lastSync string
	syncState, err := LoadSyncScheduleState(d.ConfigDir)
	if err == nil && syncState != nil && !syncState.LastFullSync.IsZero() {
		lastSync = syncState.LastFullSync.Format(time.RFC3339)
	} else {
		lastSync = "never"
	}

	hasFailures, _ := HasPendingSyncFailures(d.ConfigDir)
	var failures []SyncFailure
	if hasFailures {
		failures, _ = LoadSyncFailures(d.ConfigDir)
	}

	// ── Ship-gate blockers ─────────────────────────────────────────────
	blockingSpecs := checkBlockingSpecs(d.ConfigDir)

	// ── Render dashboard ───────────────────────────────────────────────
	printStatusHeader(d.Stdout, entry)

	fmt.Fprintln(d.Stdout, "")
	fmt.Fprintln(d.Stdout, "Spec Status:")
	for _, l := range lines {
		if l.Count == 0 {
			continue
		}
		c, ok := statusColors[l.Name]
		if !ok {
			c = color.New(color.FgWhite).SprintfFunc()
		}
		barLen := l.Count
		if barLen > 20 {
			barLen = 20
		}
		bar := strings.Repeat("█", barLen)
		if l.Count > 20 {
			bar += ">"
		}
		fmt.Fprintf(d.Stdout, "  %s  %s %5s  %d specs\n", c("%-12s", l.Name), bar, "", l.Count)
	}
	fmt.Fprintf(d.Stdout, "  %-12s  %s %5s  %d specs\n", "total", strings.Repeat("░", activeTotal), "", len(specs))

	// ── Sync status ────────────────────────────────────────────────────
	fmt.Fprintln(d.Stdout, "")
	fmt.Fprintln(d.Stdout, "Sync Status:")
	if lastSync != "never" {
		fmt.Fprintf(d.Stdout, "  Last full sync: %s\n", lastSync)
	} else {
		fmt.Fprintf(d.Stdout, "  Last full sync: %s\n", color.YellowString("never"))
	}

	if hasFailures {
		fmt.Fprintf(d.Stdout, "  Pending failures: %s (%d)\n",
			color.YellowString("⚠"),
			len(failures))
		for _, f := range failures {
			fmt.Fprintf(d.Stdout, "    - %s: %s\n", f.SpecID, shorten(f.Error, 80))
		}
	} else {
		fmt.Fprintf(d.Stdout, "  Pending failures: %s\n", color.GreenString("none"))
	}

	// ── Ship gate ──────────────────────────────────────────────────────
	fmt.Fprintln(d.Stdout, "")
	if len(blockingSpecs) > 0 {
		fmt.Fprintf(d.Stdout, "  Ship Gate: %s\n", color.RedString("⛔ Blocked"))
		fmt.Fprintf(d.Stdout, "  Blocked by:\n")
		for _, b := range blockingSpecs {
			fmt.Fprintf(d.Stdout, "    - %s\n", b)
		}
	} else {
		fmt.Fprintf(d.Stdout, "  Ship Gate: %s\n", color.GreenString("✓ Clear"))
	}

	return CommandResult{ExitCode: 0}, nil
}

// printStatusHeader prints the top section of the status dashboard.
func printStatusHeader(w io.Writer, entry *LedgerEntry) {
	fmt.Fprintln(w, "╔══════════════════════════════════════╗")
	fmt.Fprintln(w, "║        ForgeFix Status              ║")
	fmt.Fprintln(w, "╠══════════════════════════════════════╣")
	if entry != nil {
		fmt.Fprintf(w, "║ Pipeline: forgefix                  ║\n")
		fmt.Fprintf(w, "║ Tests:    %d ran, %d passed, %d failed ║\n",
			entry.TotalRan, entry.TotalPassed, entry.TotalFailed)
	}
	fmt.Fprintln(w, "╚══════════════════════════════════════╝")
}

// checkBlockingSpecs scans the specs directory and returns a list of spec
// identifiers that are not in "ship" or "closed" status (i.e. would block
// the shipping gate).
func checkBlockingSpecs(configDir string) []string {
	specDir := filepath.Join(configDir, "specs")
	if _, statErr := os.Stat(specDir); os.IsNotExist(statErr) {
		specDir = filepath.Join(filepath.Dir(configDir), "specs")
	}
	entries, readErr := os.ReadDir(specDir)
	if readErr != nil {
		return nil
	}

	var blocking []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		spec, parseErr := parseSpecFile(filepath.Join(specDir, e.Name()))
		if parseErr != nil {
			continue
		}
		if spec.Status != "ship" && spec.Status != "closed" {
			blocking = append(blocking, fmt.Sprintf("%s (%s)", spec.SpecID, spec.Status))
		}
	}
	return blocking
}

// shorten truncates s to at most n runes, appending "..." if truncated.
func shorten(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
