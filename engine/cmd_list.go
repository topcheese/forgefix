package engine

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
)

// handleListSpecs lists active (or all) specs in a formatted table.
func (d *CommandDispatcher) handleListSpecs(args []string) (CommandResult, error) {
	flags := ParseFlags(args)

	ledger, err := LoadLedger(d.ConfigDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: failed to load ledger: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	specs, err := ledger.ListSpecs(flags.All)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: failed to list specs: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	w := tabwriter.NewWriter(d.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "SpecID\tRepo Issue\tStatus\tLinked Commits")
	fmt.Fprintln(w, "-------\t-----------\t------\t------------")

	cfg := ledger.WorkflowConfig
	statusColors := buildColorMap(cfg.Statuses)

	activeCount := 0
	archivedCount := 0

	for _, spec := range specs {
		repoIssue := spec.RepoIssueID
		if repoIssue == 0 {
			repoIssue = -1
		}

		linkedCommits := strings.Join(spec.LinkedCommits, ", ")
		if linkedCommits == "" {
			linkedCommits = "-"
		}

		colorizedStatus := colorizeStatus(spec.Status, statusColors)
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", spec.SpecID, repoIssue, colorizedStatus, linkedCommits)

		if cfg != nil && cfg.IsArchivedStatus(spec.Status) {
			archivedCount++
		} else if cfg == nil && (spec.Status == "resolved" || spec.Status == "deprecated") {
			archivedCount++
		} else {
			activeCount++
		}
	}

	w.Flush()
	fmt.Fprintf(d.Stdout, "\nActive specs: %d\nArchived specs: %d\n", activeCount, archivedCount)

	return CommandResult{ExitCode: 0}, nil
}

// colorFunc is a sprintf-style function that applies a color.
type colorFunc func(format string, a ...interface{}) string

// colorAttr converts a color name string to a color.Attribute.
func colorAttr(name string) color.Attribute {
	switch strings.ToLower(name) {
	case "black":
		return color.FgBlack
	case "red":
		return color.FgRed
	case "green":
		return color.FgGreen
	case "yellow":
		return color.FgYellow
	case "blue":
		return color.FgBlue
	case "magenta":
		return color.FgMagenta
	case "cyan":
		return color.FgCyan
	case "white":
		return color.FgWhite
	case "hiblack", "hi-black":
		return color.FgHiBlack
	case "hired", "hi-red":
		return color.FgHiRed
	case "higreen", "hi-green":
		return color.FgHiGreen
	case "hiyellow", "hi-yellow":
		return color.FgHiYellow
	case "hiblue", "hi-blue":
		return color.FgHiBlue
	case "himagenta", "hi-magenta":
		return color.FgHiMagenta
	case "hicyan", "hi-cyan":
		return color.FgHiCyan
	case "hiwhite", "hi-white":
		return color.FgHiWhite
	default:
		return color.FgWhite
	}
}

// buildColorMap builds a lookup from status name to color.SprintfFunc.
func buildColorMap(defs []StatusDef) map[string]colorFunc {
	m := make(map[string]colorFunc, len(defs))
	for _, d := range defs {
		m[d.Name] = color.New(colorAttr(d.Color)).SprintfFunc()
	}
	return m
}

// colorizeStatus wraps the status string in its configured ANSI color.
func colorizeStatus(status string, m map[string]colorFunc) string {
	if c, ok := m[status]; ok {
		return c(status)
	}
	return status
}
