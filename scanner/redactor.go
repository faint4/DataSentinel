package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/datasentinel/datasentinel/model"
)

// RedactFile reads the input file, replaces matched patterns with asterisks,
// and writes the result to the output directory, maintaining the directory structure.
func RedactFile(fr model.FileResult, scanRoot, outDir string) error {
	if len(fr.Matches) == 0 {
		return nil
	}

	if !isPlainTextExt(fr.Ext) {
		// Only supporting plaintext files for MVP redaction. PDF/DOCX/etc are much harder.
		return fmt.Errorf("redaction not supported for %s", fr.Ext)
	}

	relPath, err := filepath.Rel(scanRoot, fr.Path)
	if err != nil {
		return err
	}

	outPath := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	// Read all content
	data, err := os.ReadFile(fr.Path)
	if err != nil {
		return err
	}
	content := string(data)

	// Since we know the exact raw string matched, we can replace all occurrences.
	// We'll process replacements.
	for _, match := range fr.Matches {
		if match.Raw != "" {
			masked := strings.Repeat("*", len(match.Raw))
			content = strings.ReplaceAll(content, match.Raw, masked)
		}
	}

	// If Raw isn't reliably set or we just have Value, let's do our best.
	// Wait, we populated `Raw` in `Match`! Let's ensure that.

	return os.WriteFile(outPath, []byte(content), 0644)
}
