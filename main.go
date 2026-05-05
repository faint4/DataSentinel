package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/datasentinel/datasentinel/model"
	"github.com/datasentinel/datasentinel/rules"
	"github.com/datasentinel/datasentinel/scanner"
	"github.com/datasentinel/datasentinel/server"
	"github.com/datasentinel/datasentinel/ui"
	"github.com/pkg/browser"
)

func main() {
	headless := flag.Bool("headless", false, "Run in headless CLI mode")
	scanPath := flag.String("scan", "", "Directory to scan (headless mode)")
	outputPath := flag.String("output", "", "Path to save JSON report (headless mode)")
	rulesPath := flag.String("rules", "", "Path to custom rules JSON file (headless mode)")
	redactDir := flag.String("redact", "", "Directory to save redacted copies of files (headless mode)")
	flag.Parse()

	// 1. Initialize logging to both console and file
	logFile := initLogger()
	if logFile != nil {
		defer logFile.Close()
	}
	log.Println("[MAIN] DataSentinel 数据哨兵 v1.0 启动")

	if *headless {
		if *scanPath == "" {
			log.Fatal("[MAIN] --scan 参数在 headless 模式下是必需的")
		}
		runHeadlessMode(*scanPath, *outputPath, *rulesPath, *redactDir)
		return
	}

	// 2. Listen on random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://127.0.0.1:%d", addr.Port)
	log.Printf("[MAIN] 监听地址: %s", url)

	// 3. Set up HTTP server with embedded UI
	mux := http.NewServeMux()

	// Serve embedded static files
	fs := http.FileServer(http.FS(ui.StaticFS))
	mux.Handle("/", fs)

	// Register API routes
	srv := server.New()
	srv.RegisterRoutes(mux)

	// 4. Start HTTP server
	httpServer := &http.Server{Handler: mux}
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// 5. Open browser
	fmt.Printf("DataSentinel 数据哨兵 v1.0\n地址: %s\n日志文件: %s\n", url, logPath())
	_ = browser.OpenURL(url)

	// 6. Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 7. Graceful shutdown
	log.Println("[MAIN] 正在关闭...")
	_ = httpServer.Shutdown(context.Background())
}

func runHeadlessMode(scanPath, outputPath string, rulesPath string, redactDir string) {
	log.Printf("[MAIN] 无头模式启动，扫描目录: %s", scanPath)

	req := model.ScanRequest{
		Path:       scanPath,
		Extensions: model.SupportedExtensions(),
		Categories: []string{}, // all categories
	}

	var customRules []rules.Rule
	if rulesPath != "" {
		cr, err := rules.LoadCustomRules(rulesPath)
		if err != nil {
			log.Printf("[MAIN] 无法加载自定义规则: %v", err)
		} else {
			customRules = cr
			log.Printf("[MAIN] 成功加载 %d 条自定义规则", len(customRules))
		}
	}

	progressCh := make(chan model.ProgressEvent, 256)
	scan := scanner.NewScanner(progressCh, req.Categories, customRules...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for evt := range progressCh {
			if evt.Type == model.ProgressFileDone {
				// Optional: output progress if needed, but keeping console clean is often preferred.
			}
		}
	}()

	report, err := scan.Scan(ctx, req)
	if err != nil {
		log.Fatalf("[MAIN] 扫描失败: %v", err)
	}

	log.Printf("[MAIN] 扫描完成: 发现 %d 个文件，%d 个匹配项", report.Summary.TotalFiles, report.Summary.TotalMatches)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("[MAIN] 序列化报告失败: %v", err)
	}

	if outputPath != "" {
		err = os.WriteFile(outputPath, data, 0644)
		if err != nil {
			log.Fatalf("[MAIN] 写入报告失败: %v", err)
		}
		log.Printf("[MAIN] 报告已保存至: %s", outputPath)
	} else {
		fmt.Println(string(data))
	}

	if redactDir != "" {
		log.Printf("[MAIN] 开始脱敏处理，输出目录: %s", redactDir)
		redactedCount := 0
		for _, fr := range report.Results {
			if len(fr.Matches) > 0 {
				err := scanner.RedactFile(fr, scanPath, redactDir)
				if err == nil {
					redactedCount++
				}
			}
		}
		log.Printf("[MAIN] 脱敏完成，共处理 %d 个文件", redactedCount)
	}
}

func logPath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "datasentinel.log")
}

func initLogger() *os.File {
	p := logPath()
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback: console only
		log.SetOutput(os.Stdout)
		log.Println("[MAIN] 无法创建日志文件，仅控制台输出")
		return nil
	}
	// Write to both file and console
	multi := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multi)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	return f
}
