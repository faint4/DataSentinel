package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/datasentinel/datasentinel/model"
	"github.com/datasentinel/datasentinel/scanner"
)

func parseJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.Unmarshal(body, v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(toJSONBytes(v))
}

func toJSON(v interface{}) string {
	return string(toJSONBytes(v))
}

func toJSONBytes(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return data
}

func writeCSV(w io.Writer, report *model.ScanReport) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	// Header
	cw.Write([]string{
		"文件路径", "文件名", "扩展名", "大小(字节)",
		"自动分类级别", "匹配类别", "匹配值(脱敏)", "行号", "列号", "上下文",
		"修正后级别", "修正说明",
	})

	for _, fr := range report.Results {
		base := []string{
			fr.Path, fr.Name, fr.Ext,
			fmt.Sprintf("%d", fr.Size),
			fr.LevelName,
		}
		correctedLevel := fr.CorrectedLevelName
		correctedNote := fr.CorrectionNote

		if len(fr.Matches) == 0 {
			row := append(base, "", "", "", "", "")
			row = append(row, correctedLevel, correctedNote)
			cw.Write(row)
			continue
		}
		for _, m := range fr.Matches {
			row := append(base,
				string(m.Category),
				m.Value,
				fmt.Sprintf("%d", m.Line),
				fmt.Sprintf("%d", m.Column),
				m.Context,
			)
			row = append(row, correctedLevel, correctedNote)
			cw.Write(row)
		}
	}
}

// formatFileSize is a convenience wrapper for server-side formatting.
func formatFileSize(bytes int64) string {
	return scanner.FormatFileSize(bytes)
}

// WriteSARIF converts the ScanReport to SARIF format and writes it.
func WriteSARIF(w io.Writer, report *model.ScanReport) error {
	type message struct {
		Text string `json:"text"`
	}
	type artifactLocation struct {
		Uri string `json:"uri"`
	}
	type region struct {
		StartLine   int `json:"startLine"`
		StartColumn int `json:"startColumn"`
	}
	type physicalLocation struct {
		ArtifactLocation artifactLocation `json:"artifactLocation"`
		Region           region           `json:"region"`
	}
	type location struct {
		PhysicalLocation physicalLocation `json:"physicalLocation"`
	}
	type result struct {
		RuleId    string     `json:"ruleId"`
		Message   message    `json:"message"`
		Locations []location `json:"locations"`
		Level     string     `json:"level"`
	}
	type toolComponent struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	type tool struct {
		Driver toolComponent `json:"driver"`
	}
	type run struct {
		Tool    tool     `json:"tool"`
		Results []result `json:"results"`
	}
	type sarifDocument struct {
		Version string `json:"version"`
		Schema  string `json:"$schema"`
		Runs    []run  `json:"runs"`
	}

	var results []result
	for _, fr := range report.Results {
		if len(fr.Matches) == 0 {
			continue
		}
		for _, m := range fr.Matches {
			level := "warning"
			if fr.Level >= model.L4Secret {
				level = "error"
			}
			res := result{
				RuleId: string(m.Category),
				Message: message{
					Text: fmt.Sprintf("Sensitive data found: %s", m.Value),
				},
				Level: level,
				Locations: []location{
					{
						PhysicalLocation: physicalLocation{
							ArtifactLocation: artifactLocation{
								Uri: fr.Path,
							},
							Region: region{
								StartLine:   m.Line,
								StartColumn: m.Column,
							},
						},
					},
				},
			}
			results = append(results, res)
		}
	}

	doc := sarifDocument{
		Version: "2.1.0",
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Runs: []run{
			{
				Tool: tool{
					Driver: toolComponent{
						Name:    "DataSentinel",
						Version: "1.0",
					},
				},
				Results: results,
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// Ensure all needed types are referenced
var _ model.Level = model.L1Public
var _ model.ProgressEvent = model.ProgressEvent{}
