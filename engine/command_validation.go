package engine

import (
	"fmt"
	"strings"
)

// validateTestCommand checks a raw command string for unambiguous shell-injection
// indicators. It intentionally does NOT reject '|', '>', or '&' because the
// built-in default test commands legitimately use '||', output redirection, and
// backgrounding (e.g. "npm test -- --json 2>/dev/null || npm test").
//
// Forbidden patterns (never produced by ForgeFix defaults):
//   - backtick (`)   -> command substitution
//   - "$( "          -> command substitution
//   - ';'            -> command sequencing
//   - newline        -> multi-statement injection
func validateTestCommand(pipelineID, label, command string) error {
	if strings.ContainsRune(command, '`') {
		return fmt.Errorf("pipeline %q: %s contains a forbidden shell metacharacter (`)", pipelineID, label)
	}
	if strings.Contains(command, "$(") {
		return fmt.Errorf("pipeline %q: %s contains a forbidden shell metacharacter ($( )", pipelineID, label)
	}
	if strings.ContainsRune(command, ';') {
		return fmt.Errorf("pipeline %q: %s contains a forbidden shell metacharacter (;)", pipelineID, label)
	}
	if strings.ContainsRune(command, '\n') {
		return fmt.Errorf("pipeline %q: %s contains a forbidden newline", pipelineID, label)
	}
	return nil
}

// validateCommandConfig validates a pipeline's command configuration for
// shell-injection-prone content sourced from untrusted input.
func validateCommandConfig(pipelineID string, cmd CommandConfig) error {
	if err := validateTestCommand(pipelineID, "command.type", cmd.Type); err != nil {
		return err
	}
	for _, a := range cmd.Args {
		if err := validateTestCommand(pipelineID, "command.arg", a); err != nil {
			return err
		}
	}
	for _, p := range cmd.Paths {
		if err := validateTestCommand(pipelineID, "command.path", p); err != nil {
			return err
		}
	}
	return nil
}
