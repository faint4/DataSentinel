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

// Ensure all needed types are referenced
var _ model.Level = model.L1Public
var _ model.ProgressEvent = model.ProgressEvent{}
