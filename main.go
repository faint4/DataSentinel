package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/datasentinel/datasentinel/server"
	"github.com/datasentinel/datasentinel/ui"
	"github.com/pkg/browser"
)

func main() {
	// 1. Initialize logging to both console and file
	logFile := initLogger()
	if logFile != nil {
		defer logFile.Close()
	}
	log.Println("[MAIN] DataSentinel 数据哨兵 v1.0 启动")

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
