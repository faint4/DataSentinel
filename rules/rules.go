package rules

import (
	"bufio"
	"regexp"
	"strings"

	"github.com/datasentinel/datasentinel/model"
)

// Rule represents a single sensitive data detection rule.
type Rule struct {
	Name      string
	Category  model.RuleCategory
	Pattern   *regexp.Regexp
	Validator func(string) bool   // optional checksum validation
	MaskValue func(string) string // mask for display
}

// Match runs this rule against text and returns all findings.
func (r *Rule) Match(text string) []model.Match {
	var matches []model.Match
	locs := r.Pattern.FindAllStringIndex(text, -1)
	if locs == nil {
		return nil
	}

	lineStarts := computeLineStarts(text)

	for _, loc := range locs {
		raw := text[loc[0]:loc[1]]
		if r.Validator != nil && !r.Validator(raw) {
			continue
		}
		line, col := lineColFromPos(lineStarts, loc[0])
		context := extractContext(text, loc[0], loc[1], 40)
		matches = append(matches, model.Match{
			Category: r.Category,
			Value:    r.MaskValue(raw),
			Raw:      raw,
			Line:     line,
			Column:   col,
			Context:  context,
		})
	}
	return matches
}

// DefaultRules returns all compiled MVP rules.
func DefaultRules() []Rule {
	return []Rule{
		{
			Name:      "Chinese ID Card",
			Category:  model.CatIDCard,
			Pattern:   regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`),
			Validator: ValidateIDCard,
			MaskValue: maskIDCard,
		},
		{
			Name:     "Chinese Phone",
			Category: model.CatPhone,
			Pattern:  regexp.MustCompile(`\b1[3-9]\d{9}\b`),
			MaskValue: maskPhone,
		},
		{
			Name:      "Bank Card",
			Category:  model.CatBankCard,
			Pattern:   regexp.MustCompile(`\b(\d[\d\s\-]{11,21}\d)\b`),
			Validator: ValidateLuhn,
			MaskValue: maskBankCard,
		},
		{
			Name:     "Email",
			Category: model.CatEmail,
			Pattern:  regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
			MaskValue: maskEmail,
		},
		{
			Name:      "IPv4 Address",
			Category:  model.CatIP,
			Pattern:   regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
			Validator: ValidateIPv4,
			MaskValue: maskIP,
		},
		{
			Name:      "Unified Social Credit Code",
			Category:  model.CatUSCC,
			Pattern:   regexp.MustCompile(`\b[1-9A-HJ-NP-RTUW-Y]{2}\d{6}[A-HJ-NP-RTUW-Y0-9]{10}\b`),
			Validator: ValidateUSCC,
			MaskValue: maskUSCC,
		},
		{
			Name:     "AWS Access Key",
			Category: model.CatAWSKey,
			Pattern:  regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
			MaskValue: maskAll,
		},
		{
			Name:     "GitHub Token",
			Category: model.CatGitHubToken,
			Pattern:  regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`),
			MaskValue: maskAll,
		},
		{
			Name:     "Private Key Block",
			Category: model.CatPrivateKey,
			Pattern:  regexp.MustCompile(`-----BEGIN[A-Z ]*PRIVATE KEY-----`),
			MaskValue: func(s string) string { return "[PRIVATE KEY BLOCK]" },
		},
		{
			Name:      "High Entropy Secret",
			Category:  model.CatHighEntropy,
			Pattern:   regexp.MustCompile(`(?i)(?:password|secret|token|key|api_key|apikey|access_key)\s*[:=]\s*["']?([A-Za-z0-9+/=_\-]{20,})["']?`),
			Validator: ValidateHighEntropy,
			MaskValue: maskAll,
		},
	}
}

// --- Masking helpers ---

func maskIDCard(s string) string {
	if len(s) < 8 {
		return "****"
	}
	return s[:6] + "********" + s[len(s)-4:]
}

func maskPhone(s string) string {
	if len(s) < 7 {
		return "****"
	}
	return s[:3] + "****" + s[len(s)-4:]
}

func maskBankCard(s string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(clean) < 8 {
		return "****"
	}
	return clean[:4] + "****" + clean[len(clean)-4:]
}

func maskEmail(s string) string {
	at := strings.Index(s, "@")
	if at <= 1 {
		return "****"
	}
	return s[:1] + "***" + s[at:]
}

func maskIP(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".*.*"
	}
	return s
}

func maskUSCC(s string) string {
	if len(s) < 8 {
		return "****"
	}
	return s[:4] + "**********" + s[len(s)-4:]
}

func maskAll(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

// --- Position helpers ---

func computeLineStarts(text string) []int {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func lineColFromPos(lineStarts []int, pos int) (line, col int) {
	// Binary search for the line
	lo, hi := 0, len(lineStarts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if lineStarts[mid] <= pos {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1, pos - lineStarts[lo] + 1
}

func extractContext(text string, start, end, margin int) string {
	ctxStart := start - margin
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := end + margin
	if ctxEnd > len(text) {
		ctxEnd = len(text)
	}
	// Trim to line boundaries
	if ctxStart > 0 {
		for i := ctxStart; i < start; i++ {
			if text[i] == '\n' {
				ctxStart = i + 1
				break
			}
		}
	}
	if ctxEnd < len(text) {
		for i := end; i < ctxEnd; i++ {
			if text[i] == '\n' {
				ctxEnd = i
				break
			}
		}
	}
	return strings.TrimSpace(text[ctxStart:ctxEnd])
}

// CountLines returns the number of lines in text.
func CountLines(text string) int {
	scanner := bufio.NewScanner(strings.NewReader(text))
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}
