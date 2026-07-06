package engine

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const Version = "0.9.0"

type CLIArgs struct {
	AIMode          bool
	Help            bool
	InstallShortcut bool
	InstallGlobal   bool
	Version         bool
	Message         string
	FailureDecay    int
	RunTest         string
	SpecID          string
	SpecType        string
	SpecVersion     string
	All             bool
	Delete          bool
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
		case "--all":
			flags.All = true
		case "--delete":
			flags.Delete = true
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

Examples:
  ff                    Run test suites with interactive TUI
  ff --ai               Run in headless AI mode (JSON output)
  ff --install-shortcut Install the 'ff' shorthand command
  ff -d 300             Run with 5-minute failure decay
  ff -r TestFoo         Run only tests matching "TestFoo"
  ff --version          Show version
  ff --help             Show this help
`)
}

func PrintVersion(w io.Writer) {
	fmt.Fprintf(w, "ForgeFix %s\n", Version)
}
