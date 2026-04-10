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
	serviceLogger.Infof("Windows Network Manager v%s service starting...", version.Version)
	p.exit = make(chan struct{})

	// Start the application in a goroutine
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	serviceLogger.Info("Windows Network Manager service stopping...")

	// Stop packet interception if running
	// Don't clear state file on shutdown - preserve it for auto-restart after reboot
	runningMutex.Lock()
	wasRunning := isRunning
	if isRunning {
		isRunning = false
		if packetEngine != nil {
			packetEngine.Stop()
			packetEngine = nil
		}
	}
	runningMutex.Unlock()

	// Save state before shutdown if it was running (to preserve for next boot)
	if wasRunning {
		if err := saveState(); err != nil {
			serviceLogger.Error("Failed to save state before shutdown: ", err)
		} else {
			serviceLogger.Info("State saved for auto-restart after reboot")
		}
	}

	// Stop scheduler
	if scheduler != nil {
		scheduler.Stop()
	}

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

	// Initialize upgrade manager
	if err := initUpgradeManager(); err != nil {
		serviceLogger.Error("Failed to initialize upgrade manager: ", err)
	}

	// Initialize scheduler
	scheduler = NewScheduler()
	if err := scheduler.LoadConfig(); err != nil {
		serviceLogger.Error("Failed to load schedule config: ", err)
	}
	scheduler.Start()

	// Setup HTTP routes
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/stats/stream", handleStatsStream)
	http.HandleFunc("/api/start", handleStart)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/network", handleNetwork)
	http.HandleFunc("/api/discover", handleDiscover)
	http.HandleFunc("/api/upgrade/check", handleUpgradeCheck)
	http.HandleFunc("/api/upgrade", handleUpgrade)
	http.HandleFunc("/api/upgrade/status", handleUpgradeStatus)
	http.HandleFunc("/api/ping/start", handlePingStart)
	http.HandleFunc("/api/ping/stop", handlePingStop)
	http.HandleFunc("/api/ping/status", handlePingStatus)
	http.HandleFunc("/api/ping/stream", handlePingStream)
	http.HandleFunc("/api/schedule", handleSchedule)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/logs/local", handleLocalLogs)

	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	port := "18080"
	bindAddr := "0.0.0.0:" + port
	serviceLogger.Infof("Windows Network Manager v%s", version.Version)
	serviceLogger.Infof("Starting web server on http://localhost:%s", port)

	// Get and display local IP addresses for network access (with retry)
	displayLocalIPs := func() {
		localIPs := getLocalIPs()
		if len(localIPs) > 0 {
			serviceLogger.Info("Web interface accessible from network at:")
			for _, ip := range localIPs {
				serviceLogger.Infof("  http://%s:%s", ip, port)
			}
		} else {
			serviceLogger.Info("Note: Could not detect local IP addresses for network access")
			serviceLogger.Info("The web interface is still accessible at http://localhost:18080")
			serviceLogger.Info("To find your IP address, run: ipconfig")
			serviceLogger.Info("Or access http://localhost:18080/api/network from the web interface")
		}
	}
	
	// Try immediately
	displayLocalIPs()
	
	// Retry after a delay in case network interfaces weren't ready
	go func() {
		time.Sleep(3 * time.Second)
		localIPs := getLocalIPs()
		if len(localIPs) > 0 {
			serviceLogger.Info("Network IP addresses detected (retry):")
			for _, ip := range localIPs {
				serviceLogger.Infof("  http://%s:%s", ip, port)
			}
		}
	}()

	// Services typically run with admin privileges, so we don't need to check/warn
	serviceLogger.Info("Service mode - running with service account privileges")
	serviceLogger.Infof("Note: Windows Firewall may need to allow incoming connections on port %s", port)

	httpServer = &http.Server{
		Addr:    bindAddr,
		Handler: nil,
	}

	// Check for auto-start after reboot
	go func() {
		// Wait a bit for HTTP server to be ready
		time.Sleep(2 * time.Second)
		
		state, err := loadState()
		if err != nil {
			serviceLogger.Error("Failed to load state: ", err)
			return
		}
		
		if state != nil && state.WasRunning {
			// Get auto-restart delay (default to 5 minutes if not set)
			autoRestartDelay := state.AutoRestartDelayMinutes
			if autoRestartDelay <= 0 {
				autoRestartDelay = 5
			}
			
			serviceLogger.Infof("Packet interception was running before reboot. Auto-starting in %d minutes...", autoRestartDelay)
			
			// Wait for the configured delay
			time.Sleep(time.Duration(autoRestartDelay) * time.Minute)
			
			// Check if still not running (user might have started it manually)
			runningMutex.RLock()
			stillNotRunning := !isRunning
			runningMutex.RUnlock()
			
			if stillNotRunning {
				serviceLogger.Info("Auto-starting packet interception with saved settings...")
				
				// Restore saved delay and random delay settings
				delayMutex.Lock()
				configuredDelay = time.Duration(state.DelayMs) * time.Millisecond
				delayMutex.Unlock()
				
				randomDelayMutex.Lock()
				useRandomDelay = state.RandomDelay
				randomDelayMutex.Unlock()

				loss := state.PacketLossPercent
				if loss < 0 {
					loss = 0
				}
				if loss > 100 {
					loss = 100
				}
				dropPercentMutex.Lock()
				configuredDropPercent = loss
				dropPercentMutex.Unlock()
				
				// Start packet interception
				delay := time.Duration(state.DelayMs) * time.Millisecond
				var err error
				packetEngine, err = NewPacketEngine(delay)
				if err != nil {
					serviceLogger.Error("Failed to auto-start packet interception: ", err)
					return
				}
				
				packetEngine.SetRandomDelay(state.RandomDelay)
				packetEngine.SetDropPercent(loss)
				
				runningMutex.Lock()
				isRunning = true
				runningMutex.Unlock()
				
				go packetEngine.Start()
				
				// Save state again (in case it was cleared)
				if err := saveState(); err != nil {
					serviceLogger.Error("Failed to save state after auto-start: ", err)
				}
				
				serviceLogger.Infof("Packet interception auto-started successfully with delay=%dms, random_delay=%v, packet_loss_percent=%d", state.DelayMs, state.RandomDelay, loss)
			} else {
				serviceLogger.Info("Packet interception already started manually, skipping auto-start")
			}
		}
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		serviceLogger.Error("Failed to start server: ", err)
	}
}
