package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kardianos/service"
	"golang.org/x/sys/windows"
	"WindowsNetworkManager/version"
)

// isRunningAsAdmin checks if the current process is running with administrator privileges
func isRunningAsAdmin() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	
	// Use Windows API to check if process token is elevated
	// This is the same method used by WinDivert library
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

var (
	configuredDelay   time.Duration
	delayMutex        sync.RWMutex
	useRandomDelay    bool
	randomDelayMutex  sync.RWMutex
	packetStats       = &PacketStats{}
	statsMutex        sync.RWMutex
	isRunning         bool
	runningMutex      sync.RWMutex
	packetEngine      *PacketEngine
	sessionEndTime    time.Time
	sessionEndTimeMutex sync.RWMutex
	autoRestartDelayMinutes int64 = 5 // Default 5 minutes
	autoRestartDelayMutex sync.RWMutex
	// Scheduler
	scheduler *Scheduler
	// Ping state
	pingProcess    *exec.Cmd
	pingRunning    bool
	pingDomain     string
	pingMutex      sync.RWMutex
	pingOutputChan chan string
	// Domain filter state
	filteredDomains    []string
	domainFilterMutex  sync.RWMutex
	domainFilterEnabled bool
)

type PacketStats struct {
	TotalPackets   uint64    `json:"total_packets"`
	DelayedPackets uint64    `json:"delayed_packets"`
	BytesProcessed uint64    `json:"bytes_processed"`
	StartTime      time.Time `json:"start_time"`
}

type ConfigResponse struct {
	DelayMs                 int64    `json:"delay_ms"`
	ActiveDelayMs           int64    `json:"active_delay_ms"`
	RandomDelay             bool     `json:"random_delay"`
	IsRunning               bool     `json:"is_running"`
	DurationMinutes         int64    `json:"duration_minutes,omitempty"`
	RemainingMinutes        float64  `json:"remaining_minutes,omitempty"`
	AutoRestartDelayMinutes int64    `json:"auto_restart_delay_minutes,omitempty"`
	FilteredDomains         []string `json:"filtered_domains,omitempty"`
	DomainFilterEnabled     bool     `json:"domain_filter_enabled,omitempty"`
	Error                   string   `json:"error,omitempty"`
}

// StateFile represents the persisted state for auto-restart after reboot
type StateFile struct {
	WasRunning              bool     `json:"was_running"`
	DelayMs                 int64    `json:"delay_ms"`
	RandomDelay             bool     `json:"random_delay"`
	AutoRestartDelayMinutes int64    `json:"auto_restart_delay_minutes"`
	FilteredDomains         []string `json:"filtered_domains,omitempty"`
	DomainFilterEnabled     bool     `json:"domain_filter_enabled,omitempty"`
}

type StatsResponse struct {
	TotalPackets    uint64  `json:"total_packets"`
	DelayedPackets  uint64  `json:"delayed_packets"`
	BytesProcessed  uint64  `json:"bytes_processed"`
	UptimeSeconds   float64 `json:"uptime_seconds"`
	ActiveDelayMs   int64   `json:"active_delay_ms"`
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	EventID   int    `json:"event_id"`
	Message   string `json:"message"`
}

type LogsResponse struct {
	Entries []LogEntry `json:"entries"`
	Count   int        `json:"count"`
	Error   string     `json:"error,omitempty"`
}

func main() {
	// Command line flags
	port := flag.String("port", "18080", "Web server port (ignored in service mode)")
	svcFlag := flag.String("service", "", "Service command: install, uninstall, start, stop, restart")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	
	// Handle version flag
	if *versionFlag {
		fmt.Printf("Windows Network Manager version %s\n", version.Version)
		fmt.Printf("Compiled version (baked into binary): %s\n", version.Version)
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("Go version: %s\n", buildInfo.GoVersion)
		}
		os.Exit(0)
	}

	// Get executable path for service configuration
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)

	// Service configuration
	svcConfig := &service.Config{
		Name:        "WindowsNetworkManager",
		DisplayName: "Windows Network Manager",
		Description: "Monitors network traffic and adds configurable latency to network packets",
		Executable:  exePath,
	}

	// Create service
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Handle service commands
	if len(*svcFlag) != 0 {
		serviceLogger, err = s.Logger(nil)
		if err != nil {
			log.Fatalf("Failed to create service logger: %v", err)
		}

		switch *svcFlag {
		case "install":
			if err := s.Install(); err != nil {
				log.Fatalf("Failed to install service: %v", err)
			}
			log.Println("Service installed successfully!")
			log.Println("To start the service, run: WindowsNetworkManager.exe -service start")
			log.Println("Or use: net start WindowsNetworkManager")
			return
		case "uninstall":
			if err := s.Uninstall(); err != nil {
				log.Fatalf("Failed to uninstall service: %v", err)
			}
			log.Println("Service uninstalled successfully!")
			return
		case "start":
			if err := s.Start(); err != nil {
				log.Fatalf("Failed to start service: %v", err)
			}
			log.Println("Service started successfully!")
			return
		case "stop":
			if err := s.Stop(); err != nil {
				log.Fatalf("Failed to stop service: %v", err)
			}
			log.Println("Service stopped successfully!")
			return
		case "restart":
			if err := s.Restart(); err != nil {
				log.Fatalf("Failed to restart service: %v", err)
			}
			log.Println("Service restarted successfully!")
			return
		default:
			log.Fatalf("Unknown service command: %s. Use: install, uninstall, start, stop, restart", *svcFlag)
		}
		return
	}

	// Check if running as a service
	isService, err := s.Status()
	if err == nil && isService == service.StatusRunning {
		// Running as service - use service logger
		serviceLogger, err = s.Logger(nil)
		if err != nil {
			log.Fatalf("Failed to create service logger: %v", err)
		}

		// Run as service
		if err := s.Run(); err != nil {
			serviceLogger.Error("Service run failed: ", err)
		}
		return
	}

	// Running in console mode (not as service)
	log.Printf("========================================")
	log.Printf("Windows Network Manager v%s", version.Version)
	log.Printf("========================================")
	
	// Get executable directory for web files
	if err := os.Chdir(exeDir); err != nil {
		log.Printf("Warning: Failed to change directory: %v", err)
	}

	// Initialize packet stats
	packetStats.StartTime = time.Now()

	// Initialize upgrade manager
	if err := initUpgradeManager(); err != nil {
		log.Printf("Warning: Failed to initialize upgrade manager: %v", err)
	}

	// Initialize scheduler
	scheduler = NewScheduler()
	if err := scheduler.LoadConfig(); err != nil {
		log.Printf("Warning: Failed to load schedule config: %v", err)
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
	http.HandleFunc("/api/domains", handleDomains)

	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Bind to all interfaces (0.0.0.0) to allow network access
	bindAddr := "0.0.0.0:" + *port
	log.Printf("Starting web server on http://localhost:%s", *port)

	// Get and display local IP addresses for network access
	localIPs := getLocalIPs()
	if len(localIPs) > 0 {
		log.Println("Web interface accessible from network at:")
		for _, ip := range localIPs {
			log.Printf("  http://%s:%s", ip, *port)
		}
	} else {
		log.Println("Note: Could not detect local IP addresses for network access")
		log.Println("The web interface is still accessible at http://localhost:18080")
		log.Println("To find your IP address, run: ipconfig")
		log.Println("Or access http://localhost:18080/api/network from the web interface")
	}

	// Only show admin privilege note if not running as admin
	if !isRunningAsAdmin() {
		log.Println("WARNING: Not running as Administrator - packet interception may not work")
		log.Println("Please run as Administrator for full functionality")
	}
	
	log.Printf("Note: Windows Firewall may need to allow incoming connections on port %s", *port)
	log.Println("To run as a Windows Service, use: WindowsNetworkManager.exe -service install")

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/index.html")
}

// activeDelayMilliseconds returns the packet engine's applied delay, or 0 if not running.
func activeDelayMilliseconds() int64 {
	runningMutex.RLock()
	running := isRunning
	runningMutex.RUnlock()
	if !running || packetEngine == nil {
		return 0
	}
	return packetEngine.GetDelay().Milliseconds()
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		delayMutex.RLock()
		delayMs := configuredDelay.Milliseconds()
		delayMutex.RUnlock()

		randomDelayMutex.RLock()
		randomDelay := useRandomDelay
		randomDelayMutex.RUnlock()

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		autoRestartDelayMutex.RLock()
		autoRestartDelay := autoRestartDelayMinutes
		autoRestartDelayMutex.RUnlock()

		domainFilterMutex.RLock()
		domains := make([]string, len(filteredDomains))
		copy(domains, filteredDomains)
		domainFilterEnabled := domainFilterEnabled
		domainFilterMutex.RUnlock()

		// Calculate duration and remaining time
		sessionEndTimeMutex.RLock()
		endTime := sessionEndTime
		sessionEndTimeMutex.RUnlock()

		var durationMinutes int64
		var remainingMinutes float64
		if !endTime.IsZero() && running {
			now := time.Now()
			if endTime.After(now) {
				duration := endTime.Sub(now)
				durationMinutes = int64(duration.Minutes())
				remainingMinutes = duration.Minutes()
			} else {
				// Duration expired, should stop
				remainingMinutes = 0
			}
		}

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:                 delayMs,
			ActiveDelayMs:           activeDelayMilliseconds(),
			RandomDelay:             randomDelay,
			IsRunning:               running,
			DurationMinutes:         durationMinutes,
			RemainingMinutes:        remainingMinutes,
			AutoRestartDelayMinutes: autoRestartDelay,
			FilteredDomains:         domains,
			DomainFilterEnabled:     domainFilterEnabled,
		})

	case "POST":
		var req struct {
			DelayMs                 int64 `json:"delay_ms"`
			RandomDelay             bool  `json:"random_delay"`
			AutoRestartDelayMinutes int64 `json:"auto_restart_delay_minutes"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] Invalid config request: %v", err)
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		log.Printf("[HTTP] POST /api/config - Setting delay to %d ms, random delay: %v", req.DelayMs, req.RandomDelay)

		if req.DelayMs < 0 || req.DelayMs > 10000 {
			log.Printf("[ERROR] Invalid delay value: %d ms (must be 0-10000)", req.DelayMs)
			json.NewEncoder(w).Encode(ConfigResponse{
				Error: "Delay must be between 0 and 10000 milliseconds",
			})
			return
		}

		// Update configured delay
		delayMutex.Lock()
		configuredDelay = time.Duration(req.DelayMs) * time.Millisecond
		delayMutex.Unlock()

		// Update random delay
		randomDelayMutex.Lock()
		useRandomDelay = req.RandomDelay
		randomDelayMutex.Unlock()

		// Update auto-restart delay if provided
		if req.AutoRestartDelayMinutes > 0 {
			if req.AutoRestartDelayMinutes > 60 {
				log.Printf("[ERROR] Invalid auto-restart delay: %d minutes (must be 0-60)", req.AutoRestartDelayMinutes)
				json.NewEncoder(w).Encode(ConfigResponse{
					Error: "Auto-restart delay must be between 0 and 60 minutes",
				})
				return
			}
			autoRestartDelayMutex.Lock()
			autoRestartDelayMinutes = req.AutoRestartDelayMinutes
			autoRestartDelayMutex.Unlock()
			log.Printf("[INFO] Auto-restart delay updated to %d minutes", req.AutoRestartDelayMinutes)
		}

		// Update delay and random delay mode in running packet engine
		if packetEngine != nil {
			packetEngine.SetDelay(configuredDelay)
			packetEngine.SetRandomDelay(req.RandomDelay)
			log.Printf("[INFO] Delay and random delay mode updated in running packet engine")
		} else {
			log.Printf("[INFO] Delay and random delay mode will be applied when packet interception starts")
		}

		log.Printf("[INFO] Delay updated to %d ms, random delay: %v", req.DelayMs, req.RandomDelay)

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		autoRestartDelayMutex.RLock()
		autoRestartDelay := autoRestartDelayMinutes
		autoRestartDelayMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:                 req.DelayMs,
			ActiveDelayMs:           activeDelayMilliseconds(),
			RandomDelay:             req.RandomDelay,
			IsRunning:               running,
			AutoRestartDelayMinutes: autoRestartDelay,
		})

	default:
		log.Printf("[ERROR] Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statsMutex.RLock()
	stats := *packetStats
	statsMutex.RUnlock()

	uptime := time.Since(stats.StartTime).Seconds()

	json.NewEncoder(w).Encode(StatsResponse{
		TotalPackets:    stats.TotalPackets,
		DelayedPackets:  stats.DelayedPackets,
		BytesProcessed:  stats.BytesProcessed,
		UptimeSeconds:   uptime,
		ActiveDelayMs:   activeDelayMilliseconds(),
	})
}

func handleStatsStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering for nginx if used

	// Flush headers immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Create a channel to handle client disconnection
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case <-ticker.C:
			// Check if service is running
			runningMutex.RLock()
			running := isRunning
			runningMutex.RUnlock()

			if !running {
				// Service stopped, send empty or skip update
				continue
			}

			// Get current stats
			statsMutex.RLock()
			stats := *packetStats
			statsMutex.RUnlock()

			uptime := time.Since(stats.StartTime).Seconds()

			// Create response
			response := StatsResponse{
				TotalPackets:    stats.TotalPackets,
				DelayedPackets:  stats.DelayedPackets,
				BytesProcessed:  stats.BytesProcessed,
				UptimeSeconds:   uptime,
				ActiveDelayMs:   activeDelayMilliseconds(),
			}

			// Marshal to JSON
			jsonData, err := json.Marshal(response)
			if err != nil {
				log.Printf("[ERROR] Failed to marshal stats: %v", err)
				continue
			}

			// Send SSE formatted data
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Printf("[HTTP] POST /api/start - Starting packet interception")

	runningMutex.Lock()
	if isRunning {
		runningMutex.Unlock()
		log.Printf("[ERROR] Packet interception is already running")
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: "Packet interception is already running",
		})
		return
	}
	isRunning = true
	runningMutex.Unlock()

	// Parse request body for delay, random delay, and duration
	// Always read latest values from request (frontend will send them)
	var req struct {
		DelayMs         int64 `json:"delay_ms"`
		RandomDelay     bool  `json:"random_delay"`
		DurationMinutes int64 `json:"duration_minutes"`
	}
	
	// Decode request body - if empty or decode fails, use in-memory values as fallback
	var delay time.Duration
	var randomDelay bool
	
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		// Successfully decoded, use values from request and update in-memory state
		delay = time.Duration(req.DelayMs) * time.Millisecond
		randomDelay = req.RandomDelay
		
		// Update in-memory variables
		delayMutex.Lock()
		configuredDelay = delay
		delayMutex.Unlock()
		
		randomDelayMutex.Lock()
		useRandomDelay = randomDelay
		randomDelayMutex.Unlock()
		
		log.Printf("[INFO] Using values from request: delay=%dms, random_delay=%v", req.DelayMs, randomDelay)
	} else {
		// Failed to decode or empty body, use in-memory values as fallback
		delayMutex.RLock()
		delay = configuredDelay
		delayMutex.RUnlock()
		
		randomDelayMutex.RLock()
		randomDelay = useRandomDelay
		randomDelayMutex.RUnlock()
		
		log.Printf("[INFO] Using in-memory values (request decode failed or empty): delay=%v, random_delay=%v", delay, randomDelay)
	}

	log.Printf("Attempting to start packet interception with delay: %v, random delay: %v", delay, randomDelay)

	// Start packet interception
	var err error
	packetEngine, err = NewPacketEngine(delay)
	if err != nil {
		runningMutex.Lock()
		isRunning = false
		runningMutex.Unlock()
		errorMsg := fmt.Sprintf("Failed to start packet interception: %v", err)
		log.Printf("[ERROR] %s", errorMsg)
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: errorMsg,
		})
		return
	}

	// Set random delay mode
	packetEngine.SetRandomDelay(randomDelay)

	// Apply domain filter settings
	domainFilterMutex.RLock()
	domains := make([]string, len(filteredDomains))
	copy(domains, filteredDomains)
	enabled := domainFilterEnabled
	domainFilterMutex.RUnlock()
	packetEngine.SetDomainFilter(domains, enabled)

	go packetEngine.Start()

	// Handle session duration if specified
	var durationMinutes int64
	if req.DurationMinutes > 0 {
		durationMinutes = req.DurationMinutes
		sessionEndTimeMutex.Lock()
		sessionEndTime = time.Now().Add(time.Duration(req.DurationMinutes) * time.Minute)
		sessionEndTimeMutex.Unlock()

		// Start timer goroutine to auto-stop when duration expires
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					sessionEndTimeMutex.RLock()
					endTime := sessionEndTime
					sessionEndTimeMutex.RUnlock()

					if endTime.IsZero() {
						// Session was stopped manually, exit
						return
					}

					if time.Now().After(endTime) || time.Now().Equal(endTime) {
						log.Printf("[INFO] Session duration expired, stopping packet interception")
						stopPacketInterception()
						return
					}
				}
			}
		}()

		log.Printf("[INFO] Session duration set to %d minutes", req.DurationMinutes)
	} else {
		// Clear session end time if no duration
		sessionEndTimeMutex.Lock()
		sessionEndTime = time.Time{}
		sessionEndTimeMutex.Unlock()
	}

	log.Println("[INFO] Packet interception started successfully")

	// Save state for auto-restart after reboot
	if err := saveState(); err != nil {
		log.Printf("[WARN] Failed to save state: %v", err)
	}

	delayMutex.RLock()
	cfgMs := configuredDelay.Milliseconds()
	delayMutex.RUnlock()

	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning:       true,
		DurationMinutes: durationMinutes,
		DelayMs:         cfgMs,
		ActiveDelayMs:   activeDelayMilliseconds(),
	})
}

// stopPacketInterception stops packet interception programmatically (used by duration timer)
func stopPacketInterception() {
	runningMutex.Lock()
	if !isRunning {
		runningMutex.Unlock()
		return
	}
	isRunning = false
	runningMutex.Unlock()

	// Clear session end time
	sessionEndTimeMutex.Lock()
	sessionEndTime = time.Time{}
	sessionEndTimeMutex.Unlock()

	// Stop packet interception
	if packetEngine != nil {
		log.Println("Stopping packet engine...")
		packetEngine.Stop()
		packetEngine = nil
		log.Println("[INFO] Packet engine stopped")
	}

	log.Println("[INFO] Packet interception stopped successfully")
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Printf("[HTTP] POST /api/stop - Stopping packet interception")

	runningMutex.Lock()
	if !isRunning {
		runningMutex.Unlock()
		log.Printf("[ERROR] Packet interception is not running")
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: "Packet interception is not running",
		})
		return
	}
	runningMutex.Unlock()

	stopPacketInterception()

	// Clear state file since this is a manual stop (won't auto-start after reboot)
	if err := clearState(); err != nil {
		log.Printf("[WARN] Failed to clear state: %v", err)
	}

	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning: false,
	})
}

func handleNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	localIPs := getLocalIPs()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"local_ips": localIPs,
		"port":      "18080",
		"hostname":  machineHostname(),
	})
}

func handleDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	localIPs := getLocalIPs()
	runningMutex.RLock()
	running := isRunning
	runningMutex.RUnlock()

	delayMutex.RLock()
	delayMs := configuredDelay.Milliseconds()
	delayMutex.RUnlock()

	randomDelayMutex.RLock()
	randomDelay := useRandomDelay
	randomDelayMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":           version.ServiceName,
		"version":           version.Version,
		"port":              18080,
		"local_ips":         localIPs,
		"hostname":          machineHostname(),
		"is_running":        running,
		"delay_ms":          delayMs,
		"active_delay_ms":   activeDelayMilliseconds(),
		"random_delay":      randomDelay,
	})
}

func handleSchedule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if scheduler == nil {
		http.Error(w, `{"error": "Scheduler not initialized"}`, http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case "GET":
		config := scheduler.GetConfig()
		status := scheduler.GetScheduleStatus()
		
		// Combine config and status in response
		response := make(map[string]interface{})
		configJSON, _ := json.Marshal(config)
		json.Unmarshal(configJSON, &response)
		
		// Add status fields
		if status.NextSessionTime != nil {
			response["next_session_time"] = status.NextSessionTime.Format(time.RFC3339)
			response["next_session_time_local"] = status.NextSessionTime.Format("15:04:05")
		}
		response["sessions_completed"] = status.SessionsCompleted
		response["is_within_schedule"] = status.IsWithinSchedule
		response["has_active_session"] = status.HasActiveSession
		
		json.NewEncoder(w).Encode(response)

	case "POST":
		var req ScheduleConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] Invalid schedule request: %v", err)
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		// Validate config
		if req.MaxDelayMs < 1 || req.MaxDelayMs > 10000 {
			http.Error(w, `{"error": "Max delay must be between 1 and 10000 milliseconds"}`, http.StatusBadRequest)
			return
		}

		if req.MaxSessionsPerHour < 2 || req.MaxSessionsPerHour > 60 {
			http.Error(w, `{"error": "Max sessions per hour must be between 2 and 60"}`, http.StatusBadRequest)
			return
		}

		if len(req.Days) == 0 {
			http.Error(w, `{"error": "At least one day must be selected"}`, http.StatusBadRequest)
			return
		}

		// Validate time format
		if _, _, err := ParseTime(req.StartTime); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Invalid start time: %v"}`, err), http.StatusBadRequest)
			return
		}
		if _, _, err := ParseTime(req.EndTime); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Invalid end time: %v"}`, err), http.StatusBadRequest)
			return
		}

		// Update config
		if err := scheduler.SetConfig(req); err != nil {
			log.Printf("[ERROR] Failed to save schedule config: %v", err)
			http.Error(w, fmt.Sprintf(`{"error": "Failed to save schedule: %v"}`, err), http.StatusInternalServerError)
			return
		}

		log.Printf("[HTTP] POST /api/schedule - Updated schedule: enabled=%v, days=%v, time=%s-%s, max_delay=%dms, max_sessions_per_hour=%d",
			req.Enabled, req.Days, req.StartTime, req.EndTime, req.MaxDelayMs, req.MaxSessionsPerHour)

		json.NewEncoder(w).Encode(req)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func updateStats(packets, bytes uint64) {
	statsMutex.Lock()
	packetStats.TotalPackets += packets
	packetStats.DelayedPackets += packets
	packetStats.BytesProcessed += bytes
	statsMutex.Unlock()
}

// getLocalIPs returns a list of local IP addresses (excluding loopback)
func getLocalIPs() []string {
	var ips []string
	
	// Method 1: Try net.InterfaceAddrs() first
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			// Skip loopback and link-local addresses
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Only include IPv4 addresses
			if ip.To4() != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	
	// Method 2: If no IPs found, try net.Interfaces() for more detailed detection
	if len(ips) == 0 {
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				// Skip loopback and down interfaces
				if iface.Flags&net.FlagLoopback != 0 {
					continue
				}
				if iface.Flags&net.FlagUp == 0 {
					continue
				}

				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}

				for _, addr := range addrs {
					ipNet, ok := addr.(*net.IPNet)
					if !ok {
						continue
					}

					ip := ipNet.IP
					// Skip loopback and link-local addresses
					if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
						continue
					}

					// Only include IPv4 addresses
					if ip.To4() != nil {
						ips = append(ips, ip.String())
					}
				}
			}
		}
	}

	return ips
}

// machineHostname returns the OS hostname for identifying this PC in APIs and UIs.
func machineHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}

// getStateFilePath returns the path to the state file
func getStateFilePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "state.json"), nil
}

// saveState saves the current state to state.json
func saveState() error {
	statePath, err := getStateFilePath()
	if err != nil {
		return err
	}

	runningMutex.RLock()
	wasRunning := isRunning
	runningMutex.RUnlock()

	if !wasRunning {
		// Don't save state if not running
		return nil
	}

	delayMutex.RLock()
	delayMs := configuredDelay.Milliseconds()
	delayMutex.RUnlock()

	randomDelayMutex.RLock()
	randomDelay := useRandomDelay
	randomDelayMutex.RUnlock()

	autoRestartDelayMutex.RLock()
	autoRestartDelay := autoRestartDelayMinutes
	autoRestartDelayMutex.RUnlock()

	domainFilterMutex.RLock()
	domains := make([]string, len(filteredDomains))
	copy(domains, filteredDomains)
	domainFilterEnabled := domainFilterEnabled
	domainFilterMutex.RUnlock()

	state := StateFile{
		WasRunning:              true,
		DelayMs:                 delayMs,
		RandomDelay:             randomDelay,
		AutoRestartDelayMinutes: autoRestartDelay,
		FilteredDomains:         domains,
		DomainFilterEnabled:     domainFilterEnabled,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	log.Printf("[STATE] Saved state: delay=%dms, random_delay=%v, auto_restart_delay=%dmin, domain_filter_enabled=%v, domains=%v", delayMs, randomDelay, autoRestartDelay, domainFilterEnabled, domains)
	return nil
}

// loadState loads the state from state.json
func loadState() (*StateFile, error) {
	statePath, err := getStateFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file, return nil (not an error)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %v", err)
	}

	// Restore domain filter settings if present
	if state.FilteredDomains != nil {
		domainFilterMutex.Lock()
		filteredDomains = state.FilteredDomains
		domainFilterEnabled = state.DomainFilterEnabled
		domainFilterMutex.Unlock()
	}

	log.Printf("[STATE] Loaded state: was_running=%v, delay=%dms, random_delay=%v, auto_restart_delay=%dmin, domain_filter_enabled=%v, domains=%v",
		state.WasRunning, state.DelayMs, state.RandomDelay, state.AutoRestartDelayMinutes, state.DomainFilterEnabled, state.FilteredDomains)
	return &state, nil
}

// clearState removes the state file
func clearState() error {
	statePath, err := getStateFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(statePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, that's fine
			return nil
		}
		return fmt.Errorf("failed to remove state file: %v", err)
	}

	log.Printf("[STATE] Cleared state file")
	return nil
}

// getEventLogs retrieves log entries from Windows Event Log
func getEventLogs(count int) ([]LogEntry, error) {
	if runtime.GOOS != "windows" {
		return []LogEntry{}, fmt.Errorf("event logs are only available on Windows")
	}

	// Validate count
	if count < 1 {
		count = 50
	}
	if count > 1000 {
		count = 1000 // Limit to prevent performance issues
	}

	// PowerShell command to get logs as JSON
	psCmd := fmt.Sprintf(
		"Get-EventLog -LogName Application -Source 'WindowsNetworkManager' -Newest %d -ErrorAction SilentlyContinue | "+
			"Select-Object TimeGenerated, EntryType, EventID, Message | "+
			"ConvertTo-Json -Compress",
		count,
	)

	cmd := exec.Command("powershell", "-Command", psCmd)
	output, err := cmd.Output()
	if err != nil {
		// If PowerShell fails, return empty array with error
		return []LogEntry{}, fmt.Errorf("failed to retrieve logs: %v", err)
	}

	// Check if output is empty or just whitespace
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || outputStr == "null" || outputStr == "[]" {
		// No logs found, return empty array (not an error)
		return []LogEntry{}, nil
	}

	// Parse JSON output from PowerShell
	var rawEntries []map[string]interface{}
	if err := json.Unmarshal([]byte(outputStr), &rawEntries); err != nil {
		// Try parsing as single object (if only one entry)
		var singleEntry map[string]interface{}
		if err2 := json.Unmarshal([]byte(outputStr), &singleEntry); err2 == nil {
			rawEntries = []map[string]interface{}{singleEntry}
		} else {
			// If parsing fails completely, return empty array with error
			previewLen := 100
			if len(outputStr) < previewLen {
				previewLen = len(outputStr)
			}
			return []LogEntry{}, fmt.Errorf("failed to parse log output: %v (output: %s)", err, outputStr[:previewLen])
		}
	}

	// Convert to LogEntry structs
	// Compile regex once for message cleaning (reused in loop)
	newlineRegex := regexp.MustCompile(`\n{2,}`)
	
	entries := make([]LogEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entry := LogEntry{}

		// Parse timestamp
		if tsInterface := raw["TimeGenerated"]; tsInterface != nil {
			var ts string
			switch v := tsInterface.(type) {
			case string:
				ts = v
			case float64:
				// Handle Unix timestamp (seconds)
				entry.Timestamp = time.Unix(int64(v), 0).Format("2006-01-02 15:04:05")
			default:
				ts = fmt.Sprintf("%v", v)
			}

			if ts != "" {
				// Try different date formats that PowerShell might return
				dateFormats := []string{
					"2006-01-02T15:04:05",
					"2006-01-02T15:04:05.0000000",
					"2006-01-02T15:04:05.0000000Z",
					"2006-01-02T15:04:05Z",
					"2006-01-02T15:04:05-07:00",
					time.RFC3339,
					time.RFC3339Nano,
					"1/2/2006 3:04:05 PM",
					"1/2/2006 15:04:05",
					"01/02/2006 3:04:05 PM",
					"01/02/2006 15:04:05",
				}
				parsed := false
				for _, format := range dateFormats {
					if t, err := time.Parse(format, ts); err == nil {
						entry.Timestamp = t.Format("2006-01-02 15:04:05")
						parsed = true
						break
					}
				}
				if !parsed {
					// Try parsing in local timezone
					for _, format := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
						if t, err := time.ParseInLocation(format, ts, time.Local); err == nil {
							entry.Timestamp = t.Format("2006-01-02 15:04:05")
							parsed = true
							break
						}
					}
				}
				if !parsed {
					// Use original if parsing fails, but limit length for readability
					if len(ts) > 50 {
						entry.Timestamp = ts[:47] + "..."
					} else {
						entry.Timestamp = ts
					}
				}
			}
		}

		// Parse level (EntryType)
		if level, ok := raw["EntryType"].(string); ok {
			entry.Level = level
		}

		// Parse EventID
		if eventID, ok := raw["EventID"].(float64); ok {
			entry.EventID = int(eventID)
		}

		// Parse message and clean it
		if msg, ok := raw["Message"].(string); ok {
			// Clean message: collapse multiple consecutive newlines (2+) to a single newline
			msg = newlineRegex.ReplaceAllString(msg, "\n")
			// Remove leading/trailing whitespace (including newlines)
			msg = strings.TrimSpace(msg)
			entry.Message = msg
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func handleUpgradeCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get update URL from query parameter or use default
	updateURL := r.URL.Query().Get("url")
	if updateURL == "" {
		updateURL = DefaultUpdateURL
	}

	status, err := CheckForUpdates(updateURL)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
			"status": status,
		})
		return
	}

	json.NewEncoder(w).Encode(status)
}

func handleUpgrade(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if upgrade is already in progress
	upgradeMutex.RLock()
	if upgradeStatus != nil && upgradeStatus.Status != "idle" && upgradeStatus.Status != "completed" && upgradeStatus.Status != "error" {
		upgradeMutex.RUnlock()
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Upgrade already in progress",
		})
		return
	}
	upgradeMutex.RUnlock()

	var req struct {
		DownloadURL string `json:"download_url,omitempty"`
		UpdateURL   string `json:"update_url,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, check for updates first
		updateURL := DefaultUpdateURL
		if req.UpdateURL != "" {
			updateURL = req.UpdateURL
		}

		status, err := CheckForUpdates(updateURL)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		if !status.UpdateAvailable {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "No update available",
				"status": status,
			})
			return
		}

		req.DownloadURL = status.DownloadURL
	}

	if req.DownloadURL == "" {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Download URL required",
		})
		return
	}

	// Start upgrade in background
	go runUpgrade(req.DownloadURL)

	json.NewEncoder(w).Encode(map[string]string{
		"status": "upgrade_started",
	})
}

func runUpgrade(downloadURL string) {
	// Download the update
	if err := DownloadUpdate(downloadURL); err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Download failed: %v", err)
		upgradeMutex.Unlock()
		return
	}

	// Create upgrade helper script that will run in a separate process
	// This is necessary because we can't replace the running executable
	// The helper script will handle stopping service, replacing files, and restarting
	if err := createUpgradeHelperScript(); err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Failed to create upgrade helper: %v", err)
		upgradeMutex.Unlock()
		return
	}

	// Launch the upgrade helper script in a separate detached process
	// This script will continue running even after the service stops
	upgradeMutex.Lock()
	upgradeStatus.Progress = "Launching upgrade helper..."
	upgradeMutex.Unlock()

	if err := launchUpgradeHelper(); err != nil {
		upgradeMutex.Lock()
		upgradeStatus.Status = "error"
		upgradeStatus.Error = fmt.Sprintf("Failed to launch upgrade helper: %v", err)
		upgradeMutex.Unlock()
		return
	}

	// Mark as installing - the helper script will complete the upgrade
	upgradeMutex.Lock()
	upgradeStatus.Status = "installing"
	upgradeStatus.Progress = "Upgrade helper launched. Service will restart shortly..."
	upgradeMutex.Unlock()
	
	// Give the helper script a moment to start and initialize
	log.Printf("[UPGRADE] Waiting for helper script to initialize...")
	time.Sleep(3 * time.Second)
	
	// Stop the service so the helper script can replace files
	// The helper script is already running and will handle the upgrade
	if serviceInstalled := isServiceInstalled(); serviceInstalled {
		log.Printf("[UPGRADE] Stopping service to allow upgrade...")
		if err := stopService(); err != nil {
			log.Printf("[UPGRADE] Warning: Failed to stop service: %v", err)
			log.Printf("[UPGRADE] Helper script will attempt to stop it")
		}
	} else {
		log.Printf("[UPGRADE] Service not installed, helper script will handle file replacement")
	}
	
	log.Printf("[UPGRADE] Upgrade process initiated. Helper script will complete the upgrade.")
}

func handleUpgradeStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // Prevent caching
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	status := GetUpgradeStatus()
	
	// Add verification info about the actual compiled version
	status.CompiledVersion = getCompiledVersion()
	
	json.NewEncoder(w).Encode(status)
}

// getCompiledVersion returns the version that was compiled into this binary
// This helps verify that the running binary has the correct version
func getCompiledVersion() string {
	// The version.Version constant is compiled into the binary at build time
	// This will always return the version that was set when the binary was built
	compiledVer := version.Version
	
	// Try to get additional build info from runtime/debug
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		// Log build info for debugging
		log.Printf("[VERSION] Build info - Go version: %s, Path: %s", buildInfo.GoVersion, buildInfo.Path)
		// Check if there are any build settings that might indicate version
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" || setting.Key == "vcs.time" {
				log.Printf("[VERSION] Build setting: %s = %s", setting.Key, setting.Value)
			}
		}
	}
	
	// Log the compiled version for verification
	log.Printf("[VERSION] Compiled version in binary: %s", compiledVer)
	
	return compiledVer
}

// PingResponse represents a ping result line
type PingResponse struct {
	Timestamp string `json:"timestamp"`
	Line      string `json:"line"`
	Type      string `json:"type"` // "response", "error", "info"
}

func handlePingStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Invalid ping start request: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		log.Printf("[ERROR] Empty domain in ping start request")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Domain is required",
		})
		return
	}

	// Basic domain validation
	if len(domain) > 255 {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Domain name too long",
		})
		return
	}

	pingMutex.Lock()
	if pingRunning {
		pingMutex.Unlock()
		log.Printf("[ERROR] Ping is already running")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Ping is already running",
		})
		return
	}
	pingRunning = true
	pingDomain = domain
	pingMutex.Unlock()

	log.Printf("[HTTP] POST /api/ping/start - Starting ping to %s", domain)

	// Create output channel for ping results (with mutex protection)
	pingMutex.Lock()
	// Clean up any existing channel first
	if pingOutputChan != nil {
		// Channel might already exist, close it first
		oldChan := pingOutputChan
		pingOutputChan = nil
		pingMutex.Unlock()
		// Close outside mutex
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic closing old ping channel: %v", r)
			}
		}()
		close(oldChan)
		pingMutex.Lock()
	}
	pingOutputChan = make(chan string, 100)
	pingMutex.Unlock()

	// Start ping command (Windows: ping -t <domain> for continuous ping)
	cmd := exec.Command("ping", "-t", domain)
	
	// Capture stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		pingMutex.Lock()
		pingRunning = false
		// Clean up channel if it was created
		if pingOutputChan != nil {
			oldChan := pingOutputChan
			pingOutputChan = nil
			pingMutex.Unlock()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Panic closing ping channel on error: %v", r)
				}
			}()
			close(oldChan)
		} else {
			pingMutex.Unlock()
		}
		log.Printf("[ERROR] Failed to create stdout pipe: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Failed to start ping: %v", err),
		})
		return
	}

	// Capture stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		pingMutex.Lock()
		pingRunning = false
		// Clean up channel if it was created
		if pingOutputChan != nil {
			oldChan := pingOutputChan
			pingOutputChan = nil
			pingMutex.Unlock()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Panic closing ping channel on error: %v", r)
				}
			}()
			close(oldChan)
		} else {
			pingMutex.Unlock()
		}
		log.Printf("[ERROR] Failed to create stderr pipe: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Failed to start ping: %v", err),
		})
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		pingMutex.Lock()
		pingRunning = false
		// Clean up channel if it was created
		if pingOutputChan != nil {
			oldChan := pingOutputChan
			pingOutputChan = nil
			pingMutex.Unlock()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Panic closing ping channel on error: %v", r)
				}
			}()
			close(oldChan)
		} else {
			pingMutex.Unlock()
		}
		log.Printf("[ERROR] Failed to start ping command: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("Failed to start ping: %v", err),
		})
		return
	}

	pingMutex.Lock()
	pingProcess = cmd
	pingMutex.Unlock()

	// Read stdout and send to channel
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in ping stdout reader: %v", r)
			}
		}()
		
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			pingMutex.RLock()
			outputChan := pingOutputChan
			pingMutex.RUnlock()
			
			if outputChan == nil {
				// Channel was closed/cleared, stop reading
				break
			}
			
			select {
			case outputChan <- line:
			default:
				// Channel full, skip this line
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[ERROR] Error reading ping stdout: %v", err)
		}
	}()

	// Read stderr and send to channel
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in ping stderr reader: %v", r)
			}
		}()
		
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			pingMutex.RLock()
			outputChan := pingOutputChan
			pingMutex.RUnlock()
			
			if outputChan == nil {
				// Channel was closed/cleared, stop reading
				break
			}
			
			select {
			case outputChan <- line:
			default:
				// Channel full, skip this line
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[ERROR] Error reading ping stderr: %v", err)
		}
	}()

	// Wait for command to finish in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in ping wait goroutine: %v", r)
				// Clean up state on panic
				pingMutex.Lock()
				pingRunning = false
				pingOutputChan = nil
				pingMutex.Unlock()
			}
		}()
		
		err := cmd.Wait()
		pingMutex.Lock()
		pingRunning = false
		outputChan := pingOutputChan
		pingMutex.Unlock()
		
		// Exit status 1 is normal for ping when stopped (Ctrl+C equivalent)
		// Only log actual errors, not normal termination
		if err != nil {
			// Check if it's just an exit code (normal for stopped ping)
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode := exitError.ExitCode()
				if exitCode == 1 {
					// Exit code 1 is normal when ping is stopped
					log.Printf("[INFO] Ping command stopped (exit code 1)")
				} else {
					log.Printf("[INFO] Ping command finished with exit code %d: %v", exitCode, err)
				}
			} else {
				log.Printf("[INFO] Ping command finished with error: %v", err)
			}
		} else {
			log.Printf("[INFO] Ping command finished")
		}
		
		// Safely close channel if it exists
		if outputChan != nil {
			pingMutex.Lock()
			// Double-check channel still exists and hasn't been closed
			if pingOutputChan == outputChan {
				close(pingOutputChan)
				pingOutputChan = nil
			}
			pingMutex.Unlock()
		}
	}()

	log.Printf("[INFO] Ping started successfully to %s", domain)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"domain":  domain,
	})
}

func handlePingStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] POST /api/ping/stop - Stopping ping")

	pingMutex.Lock()
	if !pingRunning {
		pingMutex.Unlock()
		log.Printf("[ERROR] Ping is not running")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Ping is not running",
		})
		return
	}

	cmd := pingProcess
	pingMutex.Unlock()

	if cmd != nil && cmd.Process != nil {
		// Kill the ping process (with error handling)
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] Panic killing ping process: %v", r)
				}
			}()
			if err := cmd.Process.Kill(); err != nil {
				// Log error but don't fail - process might already be dead
				log.Printf("[WARN] Error killing ping process (may already be stopped): %v", err)
			} else {
				log.Printf("[INFO] Ping process killed")
			}
		}()
	}

	pingMutex.Lock()
	pingRunning = false
	outputChan := pingOutputChan
	pingProcess = nil
	pingOutputChan = nil // Set to nil first to prevent writes
	pingMutex.Unlock()
	
	// Close channel outside of mutex to avoid deadlock
	if outputChan != nil {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic closing ping channel: %v", r)
			}
		}()
		close(outputChan)
	}

	log.Printf("[INFO] Ping stopped successfully")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func handlePingStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pingMutex.RLock()
	running := pingRunning
	domain := pingDomain
	pingMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_running": running,
		"domain":     domain,
	})
}

func handlePingStream(w http.ResponseWriter, r *http.Request) {
	// Add panic recovery to prevent crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] Panic in handlePingStream: %v", r)
		}
	}()
	
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Flush headers immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Create a channel to handle client disconnection
	ctx := r.Context()

	// Regex patterns for parsing ping output
	pingReplyPattern := regexp.MustCompile(`Reply from .+?: bytes=\d+ time=(\d+)ms TTL=\d+`)
	pingErrorPattern := regexp.MustCompile(`(Request timed out|Destination host unreachable|Ping request could not find host|TTL expired|General failure)`)
	pingInfoPattern := regexp.MustCompile(`(Pinging|Ping statistics|Packets:|Approximate round trip times)`)
	
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastChannel chan string

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case <-ticker.C:
			// Check if ping is running and get channel
			pingMutex.RLock()
			running := pingRunning
			outputChan := pingOutputChan
			pingMutex.RUnlock()

			if !running || outputChan == nil {
				// Ping not running, reset channel reference and continue polling
				lastChannel = nil
				continue
			}

			// If channel changed (new ping started), update reference
			if lastChannel != outputChan {
				lastChannel = outputChan
			}

			// Read available lines from channel (non-blocking)
			for {
				select {
				case line, ok := <-outputChan:
					if !ok {
						// Channel closed, ping stopped
						pingMutex.RLock()
						stillRunning := pingRunning
						pingMutex.RUnlock()
						
						if !stillRunning {
							// Send final message (with error handling)
							func() {
								defer func() {
									if r := recover(); r != nil {
										log.Printf("[ERROR] Panic sending final ping message: %v", r)
									}
								}()
								response := PingResponse{
									Timestamp: time.Now().Format("15:04:05"),
									Line:      "Ping stopped",
									Type:      "info",
								}
								jsonData, _ := json.Marshal(response)
								fmt.Fprintf(w, "data: %s\n\n", jsonData)
								flusher.Flush()
							}()
						}
						// Channel closed, reset reference and break out of inner loop
						lastChannel = nil
						goto continueLoop
					}
					
					// Parse and format the line
					lineType := "info"
					trimmedLine := strings.TrimSpace(line)
					
					if trimmedLine == "" {
						continue
					}
					
					if pingReplyPattern.MatchString(trimmedLine) {
						lineType = "response"
					} else if pingErrorPattern.MatchString(trimmedLine) {
						lineType = "error"
					} else if pingInfoPattern.MatchString(trimmedLine) {
						lineType = "info"
					}
					
					response := PingResponse{
						Timestamp: time.Now().Format("15:04:05"),
						Line:      trimmedLine,
						Type:      lineType,
					}
					
					jsonData, err := json.Marshal(response)
					if err != nil {
						log.Printf("[ERROR] Failed to marshal ping response: %v", err)
						continue
					}
					
					// Send SSE formatted data (with error handling)
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("[ERROR] Panic sending ping data: %v", r)
							}
						}()
						fmt.Fprintf(w, "data: %s\n\n", jsonData)
						flusher.Flush()
					}()
				default:
					// No more data available right now
					goto continueLoop
				}
			}
		continueLoop:
		}
	}
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] GET /api/logs - Request received")

	// Get count from query parameter (default: 50)
	count := 50
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if parsedCount, err := fmt.Sscanf(countStr, "%d", &count); err != nil || parsedCount != 1 {
			count = 50 // Default on parse error
		}
	}

	// Validate count range
	if count < 1 {
		count = 50
	}
	if count > 1000 {
		count = 1000
	}

	log.Printf("[HTTP] GET /api/logs - Retrieving %d log entries", count)

	// Retrieve logs
	entries, err := getEventLogs(count)
	if err != nil {
		log.Printf("[HTTP] GET /api/logs - Error: %v", err)
		// Return error response but still include any entries we got
		json.NewEncoder(w).Encode(LogsResponse{
			Entries: entries,
			Count:   len(entries),
			Error:   err.Error(),
		})
		return
	}

	log.Printf("[HTTP] GET /api/logs - Successfully retrieved %d log entries", len(entries))

	// Return success response
	json.NewEncoder(w).Encode(LogsResponse{
		Entries: entries,
		Count:   len(entries),
	})
}

type LocalLogsResponse struct {
	Files   []LocalLogFile `json:"files"`
	Content string         `json:"content,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type LocalLogFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

func handleLocalLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("[HTTP] GET /api/logs/local - Request received")

	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		json.NewEncoder(w).Encode(LocalLogsResponse{
			Error: fmt.Sprintf("Failed to get executable path: %v", err),
		})
		return
	}
	exeDir := filepath.Dir(exePath)

	// Check if a specific file was requested
	fileParam := r.URL.Query().Get("file")
	if fileParam != "" {
		// Return content of specific file
		filePath := filepath.Join(exeDir, fileParam)
		// Security: ensure the file is within the executable directory
		if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(exeDir)) {
			json.NewEncoder(w).Encode(LocalLogsResponse{
				Error: "Invalid file path",
			})
			return
		}

		content, err := readLogFile(filePath)
		if err != nil {
			json.NewEncoder(w).Encode(LocalLogsResponse{
				Error: fmt.Sprintf("Failed to read log file: %v", err),
			})
			return
		}

		json.NewEncoder(w).Encode(LocalLogsResponse{
			Content: content,
		})
		return
	}

	// List available log files
	files, err := findLogFiles(exeDir)
	if err != nil {
		log.Printf("[HTTP] GET /api/logs/local - Error finding log files: %v", err)
		json.NewEncoder(w).Encode(LocalLogsResponse{
			Files: files,
			Error: fmt.Sprintf("Error finding log files: %v", err),
		})
		return
	}

	log.Printf("[HTTP] GET /api/logs/local - Found %d log files", len(files))
	json.NewEncoder(w).Encode(LocalLogsResponse{
		Files: files,
	})
}

func findLogFiles(exeDir string) ([]LocalLogFile, error) {
	var logFiles []LocalLogFile

	// Check executable directory
	dirsToCheck := []string{
		exeDir,
		filepath.Join(exeDir, "logs"),
	}

	// Common log file patterns
	logPatterns := []string{"*.log", "*.txt"}

	for _, dir := range dirsToCheck {
		// Check if directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// Search for log files
		for _, pattern := range logPatterns {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil {
					continue
				}

				// Skip if it's a directory
				if info.IsDir() {
					continue
				}

				// Get relative path from executable directory
				relPath, err := filepath.Rel(exeDir, match)
				if err != nil {
					relPath = filepath.Base(match)
				}

				logFiles = append(logFiles, LocalLogFile{
					Name:     filepath.Base(match),
					Path:     relPath,
					Size:     info.Size(),
					Modified: info.ModTime().Format("2006-01-02 15:04:05"),
				})
			}
		}
	}

	return logFiles, nil
}

func readLogFile(filePath string) (string, error) {
	// Limit file size to 10MB to prevent memory issues
	const maxSize = 10 * 1024 * 1024

	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	if info.Size() > maxSize {
		// Read only the last 10MB
		file, err := os.Open(filePath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		// Seek to position (size - maxSize) from the end
		offset := info.Size() - maxSize
		if offset < 0 {
			offset = 0
		}
		file.Seek(offset, 0)

		content := make([]byte, maxSize)
		n, err := file.Read(content)
		if err != nil && err.Error() != "EOF" {
			return "", err
		}

		return fmt.Sprintf("... (showing last %d bytes of %d total bytes)\n\n%s", n, info.Size(), string(content[:n])), nil
	}

	// Read entire file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func handleDomains(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		domainFilterMutex.RLock()
		domains := make([]string, len(filteredDomains))
		copy(domains, filteredDomains)
		enabled := domainFilterEnabled
		domainFilterMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"filtered_domains":     domains,
			"domain_filter_enabled": enabled,
		})

	case "POST":
		var req struct {
			FilteredDomains     []string `json:"filtered_domains"`
			DomainFilterEnabled bool     `json:"domain_filter_enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] Invalid domain filter request: %v", err)
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		// Validate and normalize domains
		validDomains := make([]string, 0)
		for _, domain := range req.FilteredDomains {
			domain = strings.TrimSpace(domain)
			if domain != "" {
				// Basic validation - allow alphanumeric, dots, hyphens, wildcards
				if len(domain) <= 255 {
					validDomains = append(validDomains, domain)
				}
			}
		}

		// Update domain filter
		domainFilterMutex.Lock()
		filteredDomains = validDomains
		domainFilterEnabled = req.DomainFilterEnabled
		domainFilterMutex.Unlock()

		log.Printf("[HTTP] POST /api/domains - Updated: enabled=%v, domains=%v", req.DomainFilterEnabled, validDomains)

		// Update running packet engine if it exists
		if packetEngine != nil {
			packetEngine.SetDomainFilter(validDomains, req.DomainFilterEnabled)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"filtered_domains":     validDomains,
			"domain_filter_enabled": req.DomainFilterEnabled,
			"success":              true,
		})

	case "DELETE":
		// Extract domain from URL path
		path := r.URL.Path
		parts := strings.Split(path, "/")
		if len(parts) < 4 {
			http.Error(w, `{"error": "Domain not specified"}`, http.StatusBadRequest)
			return
		}
		domainToRemove := strings.TrimSpace(parts[len(parts)-1])

		domainFilterMutex.Lock()
		newDomains := make([]string, 0)
		for _, domain := range filteredDomains {
			if strings.ToLower(strings.TrimSpace(domain)) != strings.ToLower(domainToRemove) {
				newDomains = append(newDomains, domain)
			}
		}
		filteredDomains = newDomains
		domainFilterMutex.Unlock()

		log.Printf("[HTTP] DELETE /api/domains/%s - Removed domain", domainToRemove)

		// Update running packet engine if it exists
		if packetEngine != nil {
			domainFilterMutex.RLock()
			domains := make([]string, len(filteredDomains))
			copy(domains, filteredDomains)
			enabled := domainFilterEnabled
			domainFilterMutex.RUnlock()
			packetEngine.SetDomainFilter(domains, enabled)
		}

		domainFilterMutex.RLock()
		domains := make([]string, len(filteredDomains))
		copy(domains, filteredDomains)
		enabled := domainFilterEnabled
		domainFilterMutex.RUnlock()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"filtered_domains":     domains,
			"domain_filter_enabled": enabled,
			"success":              true,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
