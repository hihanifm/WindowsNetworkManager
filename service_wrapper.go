package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
	"WindowsNetworkManager/version"
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
	appLogger.useServiceLogger = true
	appLogger.Info("Version: %s", version.Version)
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
	appLogger.Info("Service run() started")
	
	// Get executable directory for web files
	exePath, err := os.Executable()
	if err != nil {
		appLogger.Error("Failed to get executable path: %v", err)
		log.Fatalf("Failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	appLogger.Debug("Executable directory: %s", exeDir)

	// Set working directory to executable directory
	if err := os.Chdir(exeDir); err != nil {
		appLogger.Error("Failed to change directory: %v", err)
	} else {
		appLogger.Debug("Working directory set to: %s", exeDir)
	}

	// Initialize packet stats
	packetStats.StartTime = time.Now()
	appLogger.Debug("Packet stats initialized")

	// Initialize upgrade manager
	if err := initUpgradeManager(); err != nil {
		appLogger.Error("Failed to initialize upgrade manager: %v", err)
	} else {
		appLogger.Debug("Upgrade manager initialized")
	}

	// Setup HTTP routes with logging middleware
	http.HandleFunc("/", logHTTPMiddleware(serveIndex))
	http.HandleFunc("/api/config", logHTTPMiddleware(handleConfig))
	http.HandleFunc("/api/stats", logHTTPMiddleware(handleStats))
	http.HandleFunc("/api/start", logHTTPMiddleware(handleStart))
	http.HandleFunc("/api/stop", logHTTPMiddleware(handleStop))
	http.HandleFunc("/api/network", logHTTPMiddleware(handleNetwork))
	http.HandleFunc("/api/discover", logHTTPMiddleware(handleDiscover))
	http.HandleFunc("/api/upgrade/check", logHTTPMiddleware(handleUpgradeCheck))
	http.HandleFunc("/api/upgrade", logHTTPMiddleware(handleUpgrade))
	http.HandleFunc("/api/upgrade/status", logHTTPMiddleware(handleUpgradeStatus))

	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	appLogger.Debug("HTTP routes configured")

	port := "18080"
	bindAddr := "0.0.0.0:" + port
	appLogger.Info("Starting web server on http://localhost:%s", port)

	// Get and display local IP addresses for network access (with retry)
	displayLocalIPs := func() {
		localIPs := getLocalIPs()
		if len(localIPs) > 0 {
			appLogger.Info("Web interface accessible from network at:")
			for _, ip := range localIPs {
				appLogger.Info("  http://%s:%s", ip, port)
			}
		} else {
			appLogger.Info("Note: Could not detect local IP addresses for network access")
			appLogger.Info("The web interface is still accessible at http://localhost:18080")
			appLogger.Info("To find your IP address, run: ipconfig")
			appLogger.Info("Or access http://localhost:18080/api/network from the web interface")
		}
	}
	
	// Try immediately
	displayLocalIPs()
	
	// Retry after a delay in case network interfaces weren't ready
	go func() {
		time.Sleep(3 * time.Second)
		localIPs := getLocalIPs()
		if len(localIPs) > 0 {
			appLogger.Info("Network IP addresses detected (retry):")
			for _, ip := range localIPs {
				appLogger.Info("  http://%s:%s", ip, port)
			}
		}
	}()

	appLogger.Info("Note: This application requires Administrator privileges to intercept packets")
	appLogger.Info("Note: Windows Firewall may need to allow incoming connections on port %s", port)
	appLogger.Info("Server ready and listening on %s", bindAddr)

	httpServer = &http.Server{
		Addr:    bindAddr,
		Handler: nil,
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		serviceLogger.Error("Failed to start server: ", err)
	}
}
