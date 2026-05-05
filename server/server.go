package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/datasentinel/datasentinel/model"
	"github.com/datasentinel/datasentinel/scanner"
)

// Server holds the application state.
type Server struct {
	mu       sync.RWMutex
	report   *model.ScanReport
	scanning bool
	cancelFn context.CancelFunc

	progressSubs   map[chan model.ProgressEvent]struct{}
	progressSubsMu sync.RWMutex
}

// New creates a new Server instance.
func New() *Server {
	return &Server{
		progressSubs: make(map[chan model.ProgressEvent]struct{}),
	}
}

// RegisterRoutes registers all API routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/scan/start", s.handleScanStart)
	mux.HandleFunc("/api/scan/cancel", s.handleScanCancel)
	mux.HandleFunc("/api/scan/status", s.handleScanStatus)
	mux.HandleFunc("/api/results", s.handleResults)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/export/json", s.handleExportJSON)
	mux.HandleFunc("/api/export/csv", s.handleExportCSV)
	mux.HandleFunc("/api/filetypes", s.handleFileTypes)
	mux.HandleFunc("/api/browse", s.handleBrowse)
	mux.HandleFunc("/api/rules", s.handleRules)
	mux.HandleFunc("/api/correct", s.handleCorrect)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleScanStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	if s.scanning {
		s.mu.RUnlock()
		http.Error(w, `{"error":"scan already in progress"}`, http.StatusConflict)
		return
	}
	s.mu.RUnlock()

	var req model.ScanRequest
	if err := parseJSON(r, &req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, `{"error":"path is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	s.mu.Lock()
	s.cancelFn = cancel
	s.scanning = true
	s.report = nil
	s.mu.Unlock()

	progressCh := make(chan model.ProgressEvent, 256)
	s.addProgressSub(progressCh)
	defer s.removeProgressSub(progressCh)

	scan := scanner.NewScanner(progressCh, req.Categories) // UI doesn't supply custom rules yet
	log.Printf("[SCAN] 开始扫描: path=%s exts=%v categories=%v", req.Path, req.Extensions, req.Categories)

	go func() {
		defer func() {
			close(progressCh)
			s.mu.Lock()
			s.scanning = false
			s.mu.Unlock()
			cancel()
		}()
		report, err := scan.Scan(ctx, req)
		if err != nil && ctx.Err() == nil {
			log.Printf("[SCAN] 扫描出错: %v", err)
			progressCh <- model.ProgressEvent{
				Type:    model.ProgressError,
				Message: err.Error(),
			}
			return
		}
		s.mu.Lock()
		s.report = report
		s.mu.Unlock()
		log.Printf("[SCAN] 扫描完成: %d 个文件, %d 个匹配, 耗时 %d 秒",
			report.Summary.TotalFiles, report.Summary.TotalMatches,
			report.EndedAt-report.StartedAt)
	}()

	// Fan-out progress to SSE subscribers in background
	go s.fanOutProgress(progressCh)

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleScanCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
		log.Println("[SCAN] 扫描已取消")
	}
	s.scanning = false
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

func (s *Server) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := make(chan model.ProgressEvent, 64)
	s.addProgressSub(sub)
	defer s.removeProgressSub(sub)

	for {
		select {
		case evt, ok := <-sub:
			if !ok {
				return
			}
			data := toJSON(evt)
			w.Write([]byte("data: " + data + "\n\n"))
			flusher.Flush()
			if evt.Type == model.ProgressScanDone || evt.Type == model.ProgressError {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	report := s.report
	s.mu.RUnlock()

	if report == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"results": []interface{}{},
			"summary": nil,
		})
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	report := s.report
	s.mu.RUnlock()

	if report == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"summary": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"summary": report.Summary})
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	report := s.report
	s.mu.RUnlock()

	if report == nil {
		http.Error(w, `{"error":"no scan results"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=datasentinel_report.json")
	w.Write(toJSONBytes(report))
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	report := s.report
	s.mu.RUnlock()

	if report == nil {
		http.Error(w, `{"error":"no scan results"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=datasentinel_report.csv")
	// BOM for Excel compatibility
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	writeCSV(w, report)
}

func (s *Server) handleFileTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"extensions": scanner.AllSupportedExts(),
	})
}

// --- Progress fan-out ---

func (s *Server) addProgressSub(ch chan model.ProgressEvent) {
	s.progressSubsMu.Lock()
	s.progressSubs[ch] = struct{}{}
	s.progressSubsMu.Unlock()
}

func (s *Server) removeProgressSub(ch chan model.ProgressEvent) {
	s.progressSubsMu.Lock()
	delete(s.progressSubs, ch)
	s.progressSubsMu.Unlock()
}

func (s *Server) fanOutProgress(src <-chan model.ProgressEvent) {
	for evt := range src {
		s.progressSubsMu.RLock()
		for ch := range s.progressSubs {
			select {
			case ch <- evt:
			default:
				// drop if subscriber is slow
			}
		}
		s.progressSubsMu.RUnlock()
	}
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")

	if path == "" {
		// Return available drives
		var entries []model.BrowseEntry
		drives := []string{"A:", "B:", "C:", "D:", "E:", "F:", "G:", "H:",
			"I:", "J:", "K:", "L:", "M:", "N:", "O:", "P:", "Q:", "R:",
			"S:", "T:", "U:", "V:", "W:", "X:", "Y:", "Z:"}
		for _, d := range drives {
			d += `\`
			fi, err := os.Stat(d)
			if err == nil && fi.IsDir() {
				entries = append(entries, model.BrowseEntry{
					Name: d,
					Path: d,
					Type: "drive",
				})
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries, "current": ""})
		return
	}

	// Normalize path
	path = strings.ReplaceAll(path, "/", `\`)
	if len(path) == 2 && path[1] == ':' {
		path += `\`
	}

	var entries []model.BrowseEntry
	// Add parent directory entry
	parent := filepath.Dir(path)
	if parent != path {
		entries = append(entries, model.BrowseEntry{
			Name: "..",
			Path: parent,
			Type: "directory",
		})
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"entries": entries,
			"current": path,
			"error":   err.Error(),
		})
		return
	}

	// Collect directories, sort alphabetically
	var dirs []string
	for _, de := range dirEntries {
		if de.IsDir() {
			dirs = append(dirs, de.Name())
		}
	}
	sort.Strings(dirs)

	for _, d := range dirs {
		full := filepath.Join(path, d)
		entries = append(entries, model.BrowseEntry{
			Name: d,
			Path: full,
			Type: "directory",
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"current": path,
	})
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	type ruleInfo struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	allRules := []ruleInfo{
		{ID: "id_card", Name: "身份证号"},
		{ID: "phone", Name: "手机号"},
		{ID: "bank_card", Name: "银行卡号"},
		{ID: "email", Name: "邮箱地址"},
		{ID: "ip", Name: "IP地址"},
		{ID: "uscc", Name: "统一社会信用代码"},
		{ID: "aws_key", Name: "AWS密钥"},
		{ID: "github_token", Name: "GitHub Token"},
		{ID: "private_key", Name: "私钥文件"},
		{ID: "high_entropy", Name: "高熵密钥"},
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"rules": allRules})
}

func (s *Server) handleCorrect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.report == nil {
		http.Error(w, `{"error":"no scan results"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Paths []string `json:"paths"`
		Level int      `json:"level"`
		Note  string   `json:"note"`
	}
	if err := parseJSON(r, &req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if len(req.Paths) == 0 {
		http.Error(w, `{"error":"paths is required"}`, http.StatusBadRequest)
		return
	}

	pathSet := make(map[string]bool, len(req.Paths))
	for _, p := range req.Paths {
		pathSet[p] = true
	}

	corrected := model.Level(req.Level)
	correctedName := corrected.String()
	count := 0
	for i := range s.report.Results {
		fr := &s.report.Results[i]
		if pathSet[fr.Path] {
			fr.CorrectedLevel = corrected
			fr.CorrectedLevelName = correctedName
			fr.CorrectionNote = req.Note
			count++
		}
	}

	s.report.Summary = computeSummaryFromReport(s.report.Results)

	log.Printf("[CORRECT] 已修正 %d 个文件为 %s", count, correctedName)
	writeJSON(w, http.StatusOK, map[string]interface{}{"corrected": count})
}

func computeSummaryFromReport(results []model.FileResult) model.Summary {
	summary := model.Summary{
		ByLevel:    make(map[string]int),
		ByCategory: make(map[string]int),
	}
	for _, fr := range results {
		summary.TotalFiles++
		lvlName := fr.LevelName
		if fr.CorrectedLevelName != "" {
			lvlName = fr.CorrectedLevelName
		}
		summary.ByLevel[lvlName]++
		if len(fr.Matches) > 0 {
			summary.TotalMatches += len(fr.Matches)
			for _, m := range fr.Matches {
				summary.ByCategory[string(m.Category)]++
			}
		}
	}
	return summary
}
