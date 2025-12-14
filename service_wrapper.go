package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
)

var (
	serviceLogger service.Logger
	httpServer    *http.Server
)

// program struct implements service.Interface
type program struct {
	exit chan struct{}
}

func (p *program) Start(s service.Service) error {
	serviceLogger.Info("Windows Network Manager service starting...")
	p.exit = make(chan struct{})
	
	// Start the application in a goroutine
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	serviceLogger.Info("Windows Network Manager service stopping...")
	
	// Stop packet interception if running
	runningMutex.Lock()
	if isRunning {
		isRunning = false
		if packetEngine != nil {
			packetEngine.Stop()
			packetEngine = nil
		}
	}
	runningMutex.Unlock()
	
	// Stop HTTP server
	if httpServer != nil {
		if err := httpServer.Shutdown(context.Background()); err != nil {
			serviceLogger.Error("Error shutting down HTTP server: ", err)
		}
	}
	
	close(p.exit)
	return nil
}

func (p *program) run() {
	// Get executable directory for web files
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	
	// Set working directory to executable directory
	if err := os.Chdir(exeDir); err != nil {
		log.Printf("Warning: Failed to change directory: %v", err)
	}
	
	// Initialize packet stats
	packetStats.StartTime = time.Now()
	
	// Setup HTTP routes
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/start", handleStart)
	http.HandleFunc("/api/stop", handleStop)
	
	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	
	port := "8080"
	serviceLogger.Infof("Starting web server on http://localhost:%s", port)
	serviceLogger.Info("Note: This application requires Administrator privileges to intercept packets")
	
	httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}
	
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		serviceLogger.Error("Failed to start server: ", err)
	}
}

