package model

// Level represents the data classification level.
type Level int

const (
	L1Public       Level = 1 // No sensitive data found
	L2Internal     Level = 2 // Email, IP address
	L3Confidential Level = 3 // ID card, phone, bank card
	L4Secret       Level = 4 // Multiple L3 matches
	L5Restricted   Level = 5 // Private keys, cloud API keys
)

func (l Level) String() string {
	switch l {
	case L1Public:
		return "L1-Public"
	case L2Internal:
		return "L2-Internal"
	case L3Confidential:
		return "L3-Confidential"
	case L4Secret:
		return "L4-Secret"
	case L5Restricted:
		return "L5-Restricted"
	default:
		return "Unknown"
	}
}

// RuleCategory identifies the kind of sensitive data.
type RuleCategory string

const (
	CatIDCard      RuleCategory = "id_card"
	CatPhone       RuleCategory = "phone"
	CatBankCard    RuleCategory = "bank_card"
	CatEmail       RuleCategory = "email"
	CatIP          RuleCategory = "ip"
	CatUSCC        RuleCategory = "uscc"
	CatAWSKey      RuleCategory = "aws_key"
	CatGitHubToken RuleCategory = "github_token"
	CatPrivateKey  RuleCategory = "private_key"
	CatHighEntropy RuleCategory = "high_entropy"
)

func (rc RuleCategory) String() string { return string(rc) }

// Match represents a single sensitive data finding within a file.
type Match struct {
	Category RuleCategory `json:"category"`
	Value    string       `json:"value"`    // Masked for display
	Raw      string       `json:"-"`        // Unmasked raw value (report only)
	Line     int          `json:"line"`
	Column   int          `json:"column"`
	Context  string       `json:"context"`  // ~40 chars around the match
}

// FileResult contains the scan result for a single file.
type FileResult struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	Ext       string  `json:"ext"`
	Size      int64   `json:"size"`
	Level     Level   `json:"level"`
	LevelName string  `json:"level_name"`
	Matches   []Match `json:"matches"`
	Error     string  `json:"error,omitempty"`
	ScannedAt int64   `json:"scanned_at"`

	// Correction fields (user-adjusted)
	CorrectedLevel     Level   `json:"corrected_level,omitempty"`
	CorrectedLevelName string  `json:"corrected_level_name,omitempty"`
	CorrectionNote     string  `json:"correction_note,omitempty"`
	DismissedMatches   []int   `json:"dismissed_matches,omitempty"` // indices of dismissed matches
}

// ScanRequest is what the frontend sends to start a scan.
type ScanRequest struct {
	Path       string   `json:"path"`
	Extensions []string `json:"extensions"`
	Categories []string `json:"categories"` // selected rule categories, empty = all
}

// BrowseEntry represents a directory or drive in browse results.
type BrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "drive", "directory"
}

// ProgressEventType identifies the kind of progress event.
type ProgressEventType string

const (
	ProgressFileDone ProgressEventType = "file_done"
	ProgressScanDone ProgressEventType = "scan_done"
	ProgressError    ProgressEventType = "error"
)

// ProgressEvent is pushed via SSE to the browser.
type ProgressEvent struct {
	Type       ProgressEventType `json:"type"`
	Total      int               `json:"total,omitempty"`
	Completed  int               `json:"completed,omitempty"`
	Current    string            `json:"current,omitempty"`
	FileResult *FileResult       `json:"file_result,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// Summary holds aggregated scan statistics.
type Summary struct {
	TotalFiles   int                        `json:"total_files"`
	ByLevel      map[string]int             `json:"by_level"`
	ByCategory   map[string]int             `json:"by_category"`
	TotalMatches int                        `json:"total_matches"`
}

// ScanReport is the full exportable report.
type ScanReport struct {
	Summary   Summary      `json:"summary"`
	Results   []FileResult `json:"results"`
	ScanPath  string       `json:"scan_path"`
	StartedAt int64        `json:"started_at"`
	EndedAt   int64        `json:"ended_at"`
}

// FileHistory represents stored metadata for a file to support incremental scanning.
type FileHistory struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
	Level   Level  `json:"level"`
}

// ScanHistory maps file paths to their last known state.
type ScanHistory struct {
	Files map[string]FileHistory `json:"files"`
}

// SupportedExtensions returns all file extensions the MVP can scan.
func SupportedExtensions() []string {
	return []string{
		".txt", ".csv", ".log", ".json", ".xml", ".md",
		".env", ".yaml", ".yml", ".ini", ".conf", ".toml",
		".bat", ".ps1", ".sh", ".sql",
		".docx", ".xlsx", ".pptx", ".zip", ".pdf",
	}
}
