package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// AuditEntry represents a single entry in the ForgeFix audit log.
type AuditEntry struct {
	Timestamp   time.Time
	IssueNumber int
	CommitHash  string
	TestName    string
	Message     string
}

// AuditLog provides read/write/delete operations on the ForgeFix audit log file.
// All audit I/O is concentrated here rather than scattered through the coordinator.
type AuditLog struct {
	configDir string
}

// NewAuditLog creates an AuditLog that stores entries under the given project directory.
func NewAuditLog(configDir string) *AuditLog {
	return &AuditLog{configDir: configDir}
}

// resolveDir walks up from dir to find the project root that contains
// a forgefix_ff.yaml config file, falling back to dir.
func resolveAuditDir(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "forgefix_ff.yaml")); err == nil {
			return dir
		}
		forgefixDir := filepath.Join(dir, "forgefix")
		if _, err := os.Stat(filepath.Join(forgefixDir, "forgefix_ff.yaml")); err == nil {
			return forgefixDir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dir
}

// AppendEntry writes a new audit log line with the given issue number, test name,
// and message. It captures the current git commit hash automatically.
func (a *AuditLog) AppendEntry(issueNumber int, testName, message string) {
	configDir := a.configDir
	if configDir == "" {
		var err error
		configDir, err = os.Getwd()
		if err != nil {
			return
		}
	}
	configDir = resolveAuditDir(configDir)

	commitHash := ""
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}
	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("[%s] [#%d] [%s] [%s] [%s]\n", timestamp, issueNumber, commitHash, testName, message)
	auditPath := FFHistoryLogPath(configDir)
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] failed to open audit log %s: %v\n", auditPath, err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(entry); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] failed to write audit log %s: %v\n", auditPath, err)
	}
}

// ReadEntries parses the audit log file and returns all entries as a slice.
func (a *AuditLog) ReadEntries() []AuditEntry {
	configDir := resolveAuditDir(a.configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []AuditEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}
		// [timestamp] [#number] [commit] [testName] [message]
		trimmed := strings.TrimPrefix(line, "[")
		parts := strings.Split(trimmed, "] [")
		if len(parts) < 5 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}
		numberStr := strings.TrimPrefix(parts[1], "#")
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			continue
		}
		message := strings.TrimSuffix(parts[4], "]")
		entries = append(entries, AuditEntry{
			Timestamp:   ts,
			IssueNumber: number,
			CommitHash:  parts[2],
			TestName:    parts[3],
			Message:     message,
		})
	}
	return entries
}

// ReadMap reads the audit log and returns a map of test names to their most
// recent issue numbers. CLOSED entries remove the test from the map.
func (a *AuditLog) ReadMap() map[string]int {
	configDir := resolveAuditDir(a.configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// Format: [timestamp] [#number] [commit] [testName] [message]
	result := make(map[string]int)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "] [")
		if len(parts) < 5 {
			continue
		}
		numberStr := strings.TrimPrefix(parts[1], "#")
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			continue
		}
		testName := parts[3]
		message := strings.TrimSuffix(parts[4], "]")
		if strings.HasPrefix(message, "CLOSED") {
			delete(result, testName)
		} else {
			result[testName] = number
		}
	}
	return result
}

// DeleteEntry removes all audit log lines that reference the given test name.
func (a *AuditLog) DeleteEntry(testName string) {
	configDir := resolveAuditDir(a.configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "["+testName+"]") {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	os.WriteFile(path, []byte(out), 0644)
}

// ── Package-level convenience functions ──────────────────────────────────────
// These delegate to AuditLog so existing callers that pass configDir directly
// continue to work without holding an *AuditLog reference.

// LogAuditEntry writes an entry to the audit log at the given project directory.
func LogAuditEntry(configDir string, issueNumber int, testName, message string) {
	NewAuditLog(configDir).AppendEntry(issueNumber, testName, message)
}

// ReadAuditLogEntries reads all entries from the audit log at configDir.
func ReadAuditLogEntries(configDir string) []AuditEntry {
	return NewAuditLog(configDir).ReadEntries()
}

// ReadAuditLog returns a map of test names to issue numbers from the audit log.
func ReadAuditLog(configDir string) map[string]int {
	return NewAuditLog(configDir).ReadMap()
}

// DeleteAuditEntry removes all lines referencing the given test name from the audit log.
func DeleteAuditEntry(configDir string, testName string) {
	NewAuditLog(configDir).DeleteEntry(testName)
}
