package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// VersionManager handles reading, writing, and prompting for project versions.
// All version-related logic is concentrated here rather than scattered through
// ShipReconciliation.
type VersionManager struct {
	configDir string
}

// NewVersionManager creates a VersionManager for the given project directory.
func NewVersionManager(configDir string) *VersionManager {
	return &VersionManager{configDir: configDir}
}

// CurrentVersion reads the project version from the ledger JSON file.
// Returns "0.0.0" if the file cannot be read or parsed.
func (vm *VersionManager) CurrentVersion() string {
	path := ledgerPath(vm.configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return "0.0.0"
	}
	var wrapper struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return "0.0.0"
	}
	if wrapper.Version == "" {
		return "0.0.0"
	}
	return wrapper.Version
}

// WriteVersion persists the given version string to the project ledger and
// updates the spec template so newly created specs use the current version.
func (vm *VersionManager) WriteVersion(version string) error {
	ledger, err := LoadLedger(vm.configDir)
	if err != nil {
		return fmt.Errorf("loading ledger: %w", err)
	}
	ledger.Version = version
	if err := SaveLedger(ledger, vm.configDir); err != nil {
		return err
	}
	// Update the template so future specs get the correct version.
	templatePath := filepath.Join(vm.configDir, "templates", "spec_template.md")
	data, err := os.ReadFile(templatePath)
	if err == nil {
		content := strings.ReplaceAll(string(data), `version: "v0.8.0"`, fmt.Sprintf(`version: "%s"`, version))
		os.WriteFile(templatePath, []byte(content), 0644)
	}
	return nil
}

// PromptForVersion asks the user for a release version, defaulting to the
// current version with an incremented patch number. In AI mode the default
// is returned immediately without prompting.
func (vm *VersionManager) PromptForVersion(current string, aiMode bool) string {
	defaultVersion := incrementPatchVersion(current)
	if aiMode {
		return defaultVersion
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Current project version: %s\n", current)
	fmt.Printf("Release version for this ship [%s]: ", defaultVersion)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		input = defaultVersion
	}
	if !isValidSemver(input) {
		fmt.Fprintf(os.Stderr, "Warning: version %q is not a valid semver (expected X.Y.Z), using %s\n", input, defaultVersion)
		return defaultVersion
	}
	return input
}

// HandleShipVersion reads the current version, prompts for a new one, writes
// it if changed, and returns the chosen version. This encapsulates the three-step
// version flow used during ship reconciliation.
func (vm *VersionManager) HandleShipVersion(aiMode bool) string {
	current := vm.CurrentVersion()
	shipVersion := vm.PromptForVersion(current, aiMode)
	if shipVersion != current {
		if err := vm.WriteVersion(shipVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write project version: %v\n", err)
		}
	}
	return shipVersion
}

// UpdateSpecFileVersion updates the version: field in a spec file's YAML frontmatter.
// If no version: field exists, it inserts one before the closing ---.
func (vm *VersionManager) UpdateSpecFileVersion(filePath, version string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading spec file %s: %w", filePath, err)
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !inFrontmatter {
			inFrontmatter = true
			continue
		}
		if trimmed == "---" && inFrontmatter {
			break
		}
		if strings.HasPrefix(trimmed, "version:") {
			lines[i] = fmt.Sprintf("version: \"%s\"", version)
			found = true
		}
	}
	if !found {
		// Insert version before the closing ---
		for i, line := range lines {
			if strings.TrimSpace(line) == "---" && i > 0 {
				lines = append(lines[:i], append([]string{fmt.Sprintf("version: \"%s\"", version)}, lines[i:]...)...)
				break
			}
		}
	}
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

// incrementPatchVersion bumps the patch segment of a semver string.
// "1.2.3" → "1.2.4". Returns "0.0.1" on parse failure.
func incrementPatchVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "0.0.1"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "0.0.1"
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, ".")
}

// isValidSemver checks whether a string matches the basic X.Y.Z format.
func isValidSemver(version string) bool {
	re := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	return re.MatchString(version)
}
