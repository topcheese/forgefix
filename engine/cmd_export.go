package engine

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (d *CommandDispatcher) handleExport(args []string) (CommandResult, error) {
	specFilter := ""
	outputPath := "specs-export.tar.gz"

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--spec" || args[i] == "-s":
			if i+1 < len(args) {
				specFilter = args[i+1]
				i++
			}
		case args[i] == "--output" || args[i] == "-o":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			}
		case args[i] == "--help" || args[i] == "-h":
			fmt.Fprintf(d.Stdout, "usage: ff export [--spec SPEC-ID] [--output file.tar.gz]\n")
			return CommandResult{ExitCode: 0}, nil
		}
	}

	specDir := filepath.Join(d.WorkDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: reading specs directory: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	exported := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		if specFilter != "" {
			content := string(data)
			if !strings.Contains(content, fmt.Sprintf(`spec_id: "%s"`, specFilter)) &&
				!strings.Contains(content, fmt.Sprintf("spec_id: %s", specFilter)) {
				continue
			}
		}

		hdr := &tar.Header{
			Name: entry.Name(),
			Size: int64(len(data)),
			Mode: 0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			fmt.Fprintf(d.Stderr, "error: writing tar header: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		if _, err := tw.Write(data); err != nil {
			fmt.Fprintf(d.Stderr, "error: writing tar data: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}
		exported++
	}

	if err := tw.Close(); err != nil {
		fmt.Fprintf(d.Stderr, "error: closing tar: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	if err := gw.Close(); err != nil {
		fmt.Fprintf(d.Stderr, "error: closing gzip: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		fmt.Fprintf(d.Stderr, "error: writing output file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	fmt.Fprintf(d.Stdout, "exported %d spec(s) to %s\n", exported, outputPath)
	return CommandResult{ExitCode: 0}, nil
}

func (d *CommandDispatcher) handleImport(args []string) (CommandResult, error) {
	if len(args) == 0 || args[0] == "" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintf(d.Stdout, "usage: ff import <file.tar.gz> [--force]\n")
		return CommandResult{ExitCode: 0}, nil
	}

	inputPath := args[0]
	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
			break
		}
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: reading import file: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(d.Stderr, "error: reading gzip data: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}
	defer gr.Close()

	specDir := filepath.Join(d.WorkDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		fmt.Fprintf(d.Stderr, "error: creating specs directory: %v\n", err)
		return CommandResult{ExitCode: 1}, nil
	}

	tr := tar.NewReader(gr)
	imported := 0
	skipped := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(d.Stderr, "error: reading tar: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}

		if !strings.HasSuffix(hdr.Name, ".md") {
			continue
		}

		var content bytes.Buffer
		if _, err := io.Copy(&content, tr); err != nil {
			fmt.Fprintf(d.Stderr, "error: reading file content: %v\n", err)
			return CommandResult{ExitCode: 1}, nil
		}

		filePath := filepath.Join(specDir, hdr.Name)

		if _, err := os.Stat(filePath); err == nil && !force {
			fmt.Fprintf(d.Stderr, "skipping %s (already exists, use --force to overwrite)\n", hdr.Name)
			skipped++
			continue
		}

		if err := os.WriteFile(filePath, content.Bytes(), 0644); err != nil {
			fmt.Fprintf(d.Stderr, "error: writing %s: %v\n", filePath, err)
			return CommandResult{ExitCode: 1}, nil
		}
		imported++
	}

	ledgerDir := SpecConfigDir(d.WorkDir)
	ledger, loadErr := LoadLedger(ledgerDir)
	if loadErr == nil {
		for _, spec := range loadSpecFiles(specDir) {
			if entry := ledger.GetSpecEntry(spec.SpecID); entry == nil {
				ledger.SetSpecEntry(spec.SpecID, &SpecEntry{
					SpecID:        spec.SpecID,
					Status:        spec.Status,
					RepoIssueID:   0,
					LinkedCommits: []string{},
				})
			}
		}
		_ = SaveLedger(ledger, ledgerDir)
	}

	fmt.Fprintf(d.Stdout, "imported %d spec(s)", imported)
	if skipped > 0 {
		fmt.Fprintf(d.Stdout, ", skipped %d (use --force to overwrite)", skipped)
	}
	fmt.Fprintln(d.Stdout)
	return CommandResult{ExitCode: 0}, nil
}

type specFileMeta struct {
	SpecID string
	Status string
}

func loadSpecFiles(specDir string) []specFileMeta {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil
	}
	var specs []specFileMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(specDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := string(data)
		specID := extractSpecField(content, "spec_id:")
		status := extractSpecField(content, "status:")
		if specID != "" {
			specs = append(specs, specFileMeta{SpecID: specID, Status: status})
		}
	}
	return specs
}

func extractSpecField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			return strings.Trim(val, `" `)
		}
	}
	return ""
}
