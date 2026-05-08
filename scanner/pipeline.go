package scanner

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/datasentinel/datasentinel/model"
	"github.com/datasentinel/datasentinel/rules"
)

// Scanner coordinates the full scanning pipeline.
type Scanner struct {
	rules    []rules.Rule
	progress chan<- model.ProgressEvent
	history  model.ScanHistory
}

// NewScanner creates a Scanner with compiled default rules, optionally filtered by categories.
func NewScanner(progress chan<- model.ProgressEvent, categories []string, customRules ...rules.Rule) *Scanner {
	allRules := rules.DefaultRules()
	allRules = append(allRules, customRules...)

	var filtered []rules.Rule
	if len(categories) == 0 {
		filtered = allRules
	} else {
		catSet := make(map[string]bool, len(categories))
		for _, c := range categories {
			catSet[c] = true
		}
		for _, r := range allRules {
			if catSet[string(r.Category)] {
				filtered = append(filtered, r)
			}
		}
	}
	log.Printf("[SCANNER] 已加载 %d/%d 条检测规则", len(filtered), len(allRules))
	return &Scanner{
		rules:    filtered,
		progress: progress,
		history:  model.ScanHistory{Files: make(map[string]model.FileHistory)},
	}
}

func (s *Scanner) loadHistory() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	histPath := filepath.Join(filepath.Dir(exe), ".datasentinel_history.json")
	data, err := os.ReadFile(histPath)
	if err == nil {
		var h model.ScanHistory
		if err := json.Unmarshal(data, &h); err == nil && h.Files != nil {
			s.history = h
		}
	}
	if s.history.Files == nil {
		s.history.Files = make(map[string]model.FileHistory)
	}
}

func (s *Scanner) saveHistory(report *model.ScanReport) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	histPath := filepath.Join(filepath.Dir(exe), ".datasentinel_history.json")
	for _, fr := range report.Results {
		s.history.Files[fr.Path] = model.FileHistory{
			Path:    fr.Path,
			Size:    fr.Size,
			ModTime: fr.ScannedAt, // MVP: using scan time as a proxy, or ideally actual file mod time. Wait, FileInfo has ModTime. We should pass it along.
			Level:   fr.Level,
		}
	}
	data, err := json.Marshal(s.history)
	if err == nil {
		os.WriteFile(histPath, data, 0644)
	}
}

// Scan runs the full pipeline: walk -> extract -> match -> classify.
func (s *Scanner) Scan(ctx context.Context, req model.ScanRequest) (*model.ScanReport, error) {
	s.loadHistory()

	extensions := req.Extensions
	if len(extensions) == 0 {
		extensions = model.SupportedExtensions()
	}

	fileCh, err := WalkFiles(ctx, req.Path, extensions)
	if err != nil {
		return nil, err
	}

	resultCh := make(chan model.FileResult, 64)
	var wg sync.WaitGroup

	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	if numWorkers < 2 {
		numWorkers = 2
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fp := range fileCh {
				select {
				case <-ctx.Done():
					return
				default:
				}
				result := s.processFile(ctx, fp)
				resultCh <- result
				s.sendProgress(model.ProgressEvent{
					Type:       model.ProgressFileDone,
					Current:    fp.Path,
					FileResult: &result,
				})
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	report := &model.ScanReport{
		ScanPath:  req.Path,
		StartedAt: time.Now().Unix(),
		Results:   make([]model.FileResult, 0),
	}
	for fr := range resultCh {
		report.Results = append(report.Results, fr)
	}
	report.EndedAt = time.Now().Unix()
	report.Summary = computeSummary(report.Results)

	s.saveHistory(report)
	s.sendProgress(model.ProgressEvent{Type: model.ProgressScanDone})
	return report, nil
}

func (s *Scanner) processFile(ctx context.Context, fp FileInfo) model.FileResult {
	if hist, ok := s.history.Files[fp.Path]; ok {
		if hist.Size == fp.Size && hist.ModTime == fp.ModTime && hist.Level == model.L1Public {
			// Skip actual scan for L1 files to save time.
			// If it's > L1, we must rescan to populate Matches for redaction and reporting.
			return model.FileResult{
				Path:      fp.Path,
				Name:      fp.Name,
				Ext:       fp.Ext,
				Size:      fp.Size,
				Level:     hist.Level,
				LevelName: hist.Level.String(),
				ScannedAt: fp.ModTime,
			}
		}
	}

	text, err := ExtractText(fp.Path, fp.Ext)
	if err != nil {
		return model.FileResult{
			Path:      fp.Path,
			Name:      fp.Name,
			Ext:       fp.Ext,
			Size:      fp.Size,
			Error:     err.Error(),
			ScannedAt: time.Now().Unix(),
			Level:     model.L1Public,
			LevelName: model.L1Public.String(),
		}
	}

	var allMatches []model.Match
	for i := range s.rules {
		select {
		case <-ctx.Done():
			break
		default:
		}
		allMatches = append(allMatches, s.rules[i].Match(text)...)
	}

	level := rules.Classify(allMatches)
	return model.FileResult{
		Path:      fp.Path,
		Name:      fp.Name,
		Ext:       fp.Ext,
		Size:      fp.Size,
		Level:     level,
		LevelName: level.String(),
		Matches:   allMatches,
		ScannedAt: fp.ModTime, // Save actual mod time here so it gets into history
	}
}

func (s *Scanner) sendProgress(evt model.ProgressEvent) {
	if s.progress != nil {
		select {
		case s.progress <- evt:
		default:
			// Drop event if channel is full (non-blocking)
		}
	}
}

func computeSummary(results []model.FileResult) model.Summary {
	summary := model.Summary{
		ByLevel:    make(map[string]int),
		ByCategory: make(map[string]int),
	}
	for _, fr := range results {
		summary.TotalFiles++
		summary.ByLevel[fr.LevelName]++
		if len(fr.Matches) > 0 {
			summary.TotalMatches += len(fr.Matches)
			for _, m := range fr.Matches {
				summary.ByCategory[string(m.Category)]++
			}
		}
	}
	return summary
}
