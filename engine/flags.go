package engine

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Version is the compile-time fallback, used ONLY as the last-resort value by
// CurrentVersion() when no project context (DB/ledger) is available. No display
// or update-check path may reference this const directly — call
// CurrentVersion(configDir) instead so every command reports the same version.
const Version = "0.9.0"

// CurrentVersion returns the canonical ForgeFix version for a project.
// Resolution order: project DB meta.version -> legacy ledger version -> the
// compile-time Version const (last-resort fallback only). Pass an empty configDir to
// skip the DB/ledger lookup and return the const directly (non-project context).
func CurrentVersion(configDir string) string {
	if configDir != "" {
		if vm := NewVersionManager(configDir); vm != nil {
			if pv := vm.CurrentVersion(); pv != "" && pv != "0.0.0" {
				return pv
			}
		}
	}
	return Version
}

type CLIArgs struct {
	AIMode           bool
	Help             bool
	InstallShortcut  bool
	InstallGlobal    bool
	Version          bool
	Message          string
	FailureDecay     int
	RunTest          string
	SpecID           string
	SpecType         string
	SpecVersion      string
	SpecObjective    string
	SpecRequirements string
	SpecAcceptance   string
	Body             string // NEW — --body flag for commit message body
	SpecRootCause    string // NEW — --root-cause flag for spec creation
	All              bool
	Delete           bool
	SearchQuery      string
	SpecStatus       string // NEW — --status flag for spec status
}

func ParseFlags(args []string) CLIArgs {
	var flags CLIArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ai", "-ai":
			flags.AIMode = true
		case "--help", "-h":
			flags.Help = true
		case "--install-shortcut":
			flags.InstallShortcut = true
		case "--install":
			flags.InstallGlobal = true
		case "--version", "-v":
			flags.Version = true
		case "--message", "-m":
			if i+1 < len(args) {
				i++
				flags.Message = SanitizeMessage(args[i])
			}
		case "--failure-decay", "-d":
			if i+1 < len(args) {
				i++
				val, err := strconv.Atoi(args[i])
				if err == nil && val > 0 {
					flags.FailureDecay = val
				}
			}
		case "--run", "-r":
			if i+1 < len(args) {
				i++
				flags.RunTest = args[i]
			}
		case "--spec", "-s":
			if i+1 < len(args) {
				i++
				flags.SpecID = args[i]
			}
		case "--type", "-t":
			if i+1 < len(args) {
				i++
				flags.SpecType = args[i]
			}
		case "--ver":
			if i+1 < len(args) {
				i++
				flags.SpecVersion = args[i]
			}
		case "--objective", "-o":
			if i+1 < len(args) {
				i++
				flags.SpecObjective = args[i]
			}
		case "--requirements", "--req":
			if i+1 < len(args) {
				i++
				flags.SpecRequirements = args[i]
			}
		case "--acceptance", "-a":
			if i+1 < len(args) {
				i++
				flags.SpecAcceptance = args[i]
			}
		case "--body":
			if i+1 < len(args) {
				i++
				flags.Body = args[i]
			}
		case "--root-cause":
			if i+1 < len(args) {
				i++
				flags.SpecRootCause = args[i]
			}
		case "--search":
			if i+1 < len(args) {
				i++
				flags.SearchQuery = args[i]
			}
		case "--all":
			flags.All = true
		case "--delete":
			flags.Delete = true
		case "--status":
			if i+1 < len(args) {
				i++
				flags.SpecStatus = args[i]
			}
		}
	}
	return flags
}

func SanitizeMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	msg = strings.ReplaceAll(msg, "\n", "")
	msg = strings.ReplaceAll(msg, "\r", "")
	return msg
}

func PrintHelp(w io.Writer) {
	fmt.Fprintf(w, `ForgeFix - Automated Test Orchestrator & Bomb Defusal Pipeline

Usage: ff [flags]

Flags:
  --ai                  Machine-readable JSON output mode
  --help, -h            Display this help text and exit
  --install-shortcut    Install 'ff' shell shortcut globally
  --version, -v         Print version information and exit
  --message, -m <msg>   Attach a custom commit message for changelog automation
  --failure-decay, -d <secs>  Override failure_decay_seconds (suppress flaky test issues)
  --run, -r <pattern>         Run only tests matching <pattern>
  --kanban                   Open the kanban board viewer

Examples:
  ff                    Run test suites with interactive TUI
  ff --ai               Run in headless AI mode (JSON output)
  ff --install-shortcut Install the 'ff' shorthand command
  ff -d 300             Run with 5-minute failure decay
  ff -r TestFoo         Run only tests matching "TestFoo"
  ff --kanban           View and manage kanban board
  ff --kanban ls        List board, columns, and cards
  ff --version          Show version
  ff --help             Show this help
`)
}

func PrintVersion(w io.Writer) {
	fmt.Fprintf(w, "ForgeFix %s\n", CurrentVersion(""))
}
