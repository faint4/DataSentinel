package scanner

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const maxTextSize = 2 * 1024 * 1024 // 2MB text extraction limit

// ExtractText reads a file and extracts plain text based on its extension.
func ExtractText(path string, ext string) (string, error) {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt", ".csv", ".log", ".json", ".xml", ".md",
		".env", ".yaml", ".yml", ".ini", ".conf", ".toml",
		".bat", ".ps1", ".sh", ".sql":
		return extractPlainText(path)
	case ".docx":
		return extractDOCX(path)
	case ".xlsx":
		return extractXLSX(path)
	case ".pptx":
		return extractPPTX(path)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// extractPlainText reads a text file with auto-detection of UTF-8/GBK encoding.
func extractPlainText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxTextSize {
		data = data[:maxTextSize]
	}

	// Skip UTF-8 BOM
	trimmed := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// Try UTF-8 first
	if utf8.Valid(trimmed) {
		return string(trimmed), nil
	}

	// Fallback: try GBK -> UTF-8
	reader := transform.NewReader(bytes.NewReader(trimmed), simplifiedchinese.GBK.NewDecoder())
	converted, err := io.ReadAll(reader)
	if err != nil {
		// Last resort: return as-is with lossy replacement
		return strings.ToValidUTF8(string(trimmed), "?"), nil
	}
	return string(converted), nil
}

// --- DOCX extraction ---

type docxDocument struct {
	XMLName xml.Name       `xml:"document"`
	Body    docxBody       `xml:"body"`
}

type docxBody struct {
	Paragraphs []docxParagraph `xml:"p"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

func extractDOCX(path string) (string, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer rc.Close()

	var buf strings.Builder
	for _, f := range rc.File {
		if f.Name == "word/document.xml" {
			data, err := readZipFile(f)
			if err != nil {
				return "", err
			}
			var doc docxDocument
			if err := xml.Unmarshal(data, &doc); err != nil {
				return "", fmt.Errorf("parse document.xml: %w", err)
			}
			for i, p := range doc.Body.Paragraphs {
				if i > 0 {
					buf.WriteByte('\n')
				}
				for _, r := range p.Runs {
					buf.WriteString(r.Text)
				}
			}
			break
		}
	}

	result := buf.String()
	if len(result) > maxTextSize {
		result = result[:maxTextSize]
	}
	return result, nil
}

// --- XLSX extraction ---

func extractXLSX(path string) (string, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}
	defer rc.Close()

	// First pass: read shared strings
	sharedStrings := make([]string, 0)
	for _, f := range rc.File {
		if f.Name == "xl/sharedStrings.xml" {
			data, err := readZipFile(f)
			if err != nil {
				return "", err
			}
			sharedStrings = parseSharedStrings(data)
			break
		}
	}

	// Second pass: read worksheets
	var buf strings.Builder
	worksheetIdx := 0
	for _, f := range rc.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			if worksheetIdx > 0 {
				buf.WriteString("\n\n")
			}
			data, err := readZipFile(f)
			if err != nil {
				continue
			}
			cells := parseSheetCells(data, sharedStrings)
			for i, cell := range cells {
				if i > 0 {
					buf.WriteByte(' ')
				}
				buf.WriteString(cell)
			}
			worksheetIdx++
		}
	}

	result := buf.String()
	if len(result) > maxTextSize {
		result = result[:maxTextSize]
	}
	return result, nil
}

type xlsxSST struct {
	XMLName xml.Name    `xml:"sst"`
	Items   []xlsxSSTItem `xml:"si"`
}

type xlsxSSTItem struct {
	T string `xml:"t"`
}

func parseSharedStrings(data []byte) []string {
	var sst xlsxSST
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil
	}
	result := make([]string, len(sst.Items))
	for i, item := range sst.Items {
		result[i] = item.T
	}
	return result
}

type xlsxSheet struct {
	XMLName xml.Name     `xml:"worksheet"`
	SheetData xlsxSheetData `xml:"sheetData"`
}

type xlsxSheetData struct {
	Rows []xlsxRow `xml:"row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Type string `xml:"t,attr"` // "s" = shared string, "n" or "" = inline
	T    string `xml:"v"`      // value
}

func parseSheetCells(data []byte, sharedStrings []string) []string {
	var sheet xlsxSheet
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return nil
	}
	var cells []string
	for _, row := range sheet.SheetData.Rows {
		for _, cell := range row.Cells {
			val := ""
			if cell.Type == "s" && sharedStrings != nil {
				// Reference into shared strings table
				idx := 0
				fmt.Sscanf(cell.T, "%d", &idx)
				if idx >= 0 && idx < len(sharedStrings) {
					val = sharedStrings[idx]
				}
			} else {
				val = cell.T
			}
			if val != "" {
				cells = append(cells, val)
			}
		}
	}
	return cells
}

// --- PPTX extraction ---

type pptxSlide struct {
	XMLName xml.Name `xml:"sld"`
	SpTree  pptxSpTree `xml:"cSld>spTree"`
}

type pptxSpTree struct {
	Shapes []pptxShape `xml:"sp"`
}

type pptxShape struct {
	Elements []pptxTextElement `xml:"txBody>p"`
}

type pptxTextElement struct {
	Runs []pptxRun `xml:"r"`
}

type pptxRun struct {
	Text string `xml:"t"`
}

func extractPPTX(path string) (string, error) {
	rc, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open pptx: %w", err)
	}
	defer rc.Close()

	var buf strings.Builder
	slideIdx := 0
	for _, f := range rc.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			if slideIdx > 0 {
				buf.WriteString("\n\n")
			}
			data, err := readZipFile(f)
			if err != nil {
				continue
			}
			var slide pptxSlide
			if err := xml.Unmarshal(data, &slide); err != nil {
				continue
			}
			for _, shape := range slide.SpTree.Shapes {
				for i, elem := range shape.Elements {
					if i > 0 {
						buf.WriteByte('\n')
					}
					for _, run := range elem.Runs {
						buf.WriteString(run.Text)
					}
				}
			}
			slideIdx++
		}
	}

	result := buf.String()
	if len(result) > maxTextSize {
		result = result[:maxTextSize]
	}
	return result, nil
}

// --- Utility ---

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", f.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.Name, err)
	}
	return data, nil
}

// IsScannable checks if a file should be scanned based on extension.
func IsScannable(ext string) bool {
	ext = strings.ToLower(ext)
	scannable := map[string]bool{
		".txt": true, ".csv": true, ".log": true, ".json": true, ".xml": true,
		".md": true, ".env": true, ".yaml": true, ".yml": true, ".ini": true,
		".conf": true, ".toml": true, ".bat": true, ".ps1": true, ".sh": true,
		".sql": true, ".docx": true, ".xlsx": true, ".pptx": true,
	}
	return scannable[ext]
}

// FormatFileSize returns a human-readable file size string.
func FormatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FilePathExt returns the lowercase file extension.
func FilePathExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
