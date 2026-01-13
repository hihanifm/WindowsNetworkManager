package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

// UDP discovery constants
const (
	DiscoveryPort     = "18081"
	DiscoveryMagic    = "WNM-DISCOVER-v1"
	IPv6MulticastAddr = "ff02::1"
)

var (
	currentDelay      time.Duration
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
	udpConnIPv4       *net.UDPConn
	udpConnIPv6       *net.UDPConn
	udpMutex          sync.RWMutex
)

type PacketStats struct {
	TotalPackets   uint64    `json:"total_packets"`
	DelayedPackets uint64    `json:"delayed_packets"`
	BytesProcessed uint64    `json:"bytes_processed"`
	StartTime      time.Time `json:"start_time"`
}

type ConfigResponse struct {
	DelayMs          int64   `json:"delay_ms"`
	RandomDelay      bool    `json:"random_delay"`
	IsRunning        bool    `json:"is_running"`
	DurationMinutes  int64   `json:"duration_minutes,omitempty"`
	RemainingMinutes float64 `json:"remaining_minutes,omitempty"`
	Error            string  `json:"error,omitempty"`
}

type StatsResponse struct {
	TotalPackets   uint64  `json:"total_packets"`
	DelayedPackets uint64  `json:"delayed_packets"`
	BytesProcessed uint64  `json:"bytes_processed"`
	UptimeSeconds  float64 `json:"uptime_seconds"`
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

	// Start UDP discovery server
	startUDPDiscoveryServer(DiscoveryPort)

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./web/index.html")
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		delayMutex.RLock()
		delayMs := currentDelay.Milliseconds()
		delayMutex.RUnlock()

		randomDelayMutex.RLock()
		randomDelay := useRandomDelay
		randomDelayMutex.RUnlock()

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

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
			DelayMs:          delayMs,
			RandomDelay:      randomDelay,
			IsRunning:        running,
			DurationMinutes:  durationMinutes,
			RemainingMinutes: remainingMinutes,
		})

	case "POST":
		var req struct {
			DelayMs     int64 `json:"delay_ms"`
			RandomDelay bool  `json:"random_delay"`
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

		delayMutex.Lock()
		currentDelay = time.Duration(req.DelayMs) * time.Millisecond
		delayMutex.Unlock()

		randomDelayMutex.Lock()
		useRandomDelay = req.RandomDelay
		randomDelayMutex.Unlock()

		// Update delay and random delay mode in running packet engine
		if packetEngine != nil {
			packetEngine.SetDelay(currentDelay)
			packetEngine.SetRandomDelay(req.RandomDelay)
			log.Printf("[INFO] Delay and random delay mode updated in running packet engine")
		} else {
			log.Printf("[INFO] Delay and random delay mode will be applied when packet interception starts")
		}

		log.Printf("[INFO] Delay updated to %d ms, random delay: %v", req.DelayMs, req.RandomDelay)

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:     req.DelayMs,
			RandomDelay: req.RandomDelay,
			IsRunning:   running,
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
		TotalPackets:   stats.TotalPackets,
		DelayedPackets: stats.DelayedPackets,
		BytesProcessed: stats.BytesProcessed,
		UptimeSeconds:  uptime,
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
				TotalPackets:   stats.TotalPackets,
				DelayedPackets: stats.DelayedPackets,
				BytesProcessed: stats.BytesProcessed,
				UptimeSeconds:  uptime,
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

	// Parse request body for optional duration
	var req struct {
		DurationMinutes int64 `json:"duration_minutes"`
	}
	// Ignore decode errors - duration is optional, empty body is fine
	json.NewDecoder(r.Body).Decode(&req)

	// Get current delay and random delay mode
	delayMutex.RLock()
	delay := currentDelay
	delayMutex.RUnlock()

	randomDelayMutex.RLock()
	randomDelay := useRandomDelay
	randomDelayMutex.RUnlock()

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
	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning:       true,
		DurationMinutes: durationMinutes,
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
	})
}

func handleDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	localIPs := getLocalIPs()
	runningMutex.RLock()
	running := isRunning
	runningMutex.RUnlock()

	delayMutex.RLock()
	delayMs := currentDelay.Milliseconds()
	delayMutex.RUnlock()

	randomDelayMutex.RLock()
	randomDelay := useRandomDelay
	randomDelayMutex.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":      version.ServiceName,
		"version":      version.Version,
		"port":         18080,
		"local_ips":    localIPs,
		"is_running":   running,
		"delay_ms":     delayMs,
		"random_delay": randomDelay,
	})
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

// startUDPDiscoveryServer starts both IPv4 and IPv6 UDP discovery servers
func startUDPDiscoveryServer(port string) {
	// Start IPv4 broadcast listener
	go startIPv4DiscoveryServer(port)
	
	// Start IPv6 multicast listener
	go startIPv6DiscoveryServer(port)
}

// startIPv4DiscoveryServer starts the IPv4 UDP broadcast discovery server
func startIPv4DiscoveryServer(port string) {
	addr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:"+port)
	if err != nil {
		log.Printf("[UDP] Failed to resolve IPv4 address: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("[UDP] Failed to start IPv4 discovery server: %v", err)
		return
	}

	udpMutex.Lock()
	udpConnIPv4 = conn
	udpMutex.Unlock()

	log.Printf("[UDP] IPv4 discovery server listening on port %s", port)
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// Check if connection was closed
			udpMutex.RLock()
			closed := udpConnIPv4 == nil
			udpMutex.RUnlock()
			if closed {
				return
			}
			log.Printf("[UDP] Error reading IPv4 UDP packet: %v", err)
			continue
		}

		// Check if this is a discovery request
		request := strings.TrimSpace(string(buffer[:n]))
		if request == DiscoveryMagic {
			go handleUDPDiscovery(conn, clientAddr, false)
		}
	}
}

// startIPv6DiscoveryServer starts the IPv6 UDP multicast discovery server
func startIPv6DiscoveryServer(port string) {
	// Try to bind to IPv6 address
	addr, err := net.ResolveUDPAddr("udp6", "[::]:"+port)
	if err != nil {
		log.Printf("[UDP] Failed to resolve IPv6 address: %v (IPv6 may not be available)", err)
		return
	}

	conn, err := net.ListenUDP("udp6", addr)
	if err != nil {
		log.Printf("[UDP] Failed to start IPv6 discovery server: %v (IPv6 may not be available)", err)
		return
	}

	udpMutex.Lock()
	udpConnIPv6 = conn
	udpMutex.Unlock()

	// Note: On Windows, we listen on all interfaces and multicast packets
	// will be received automatically. Platform-specific multicast group
	// joining would require syscalls which we skip for simplicity.

	log.Printf("[UDP] IPv6 discovery server listening on port %s (multicast: %s)", port, IPv6MulticastAddr)
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// Check if connection was closed
			udpMutex.RLock()
			closed := udpConnIPv6 == nil
			udpMutex.RUnlock()
			if closed {
				return
			}
			log.Printf("[UDP] Error reading IPv6 UDP packet: %v", err)
			continue
		}

		// Check if this is a discovery request
		request := strings.TrimSpace(string(buffer[:n]))
		if request == DiscoveryMagic {
			go handleUDPDiscovery(conn, clientAddr, true)
		}
	}
}

// handleUDPDiscovery handles incoming UDP discovery requests
func handleUDPDiscovery(conn *net.UDPConn, clientAddr *net.UDPAddr, isIPv6 bool) {
	// Get current instance information
	localIPs := getLocalIPs()
	runningMutex.RLock()
	running := isRunning
	runningMutex.RUnlock()

	delayMutex.RLock()
	delayMs := currentDelay.Milliseconds()
	delayMutex.RUnlock()

	randomDelayMutex.RLock()
	randomDelay := useRandomDelay
	randomDelayMutex.RUnlock()

	// Get the IP address of the interface that received the request
	responseIP := clientAddr.IP.String()
	if isIPv6 {
		// For IPv6, use the local IP from the interface
		// Try to find a matching local IPv6 address
		localIPv6 := getLocalIPv6ForInterface(clientAddr.IP)
		if localIPv6 != "" {
			responseIP = localIPv6
		}
	} else {
		// For IPv4, find the local IP on the same subnet
		localIPv4 := getLocalIPv4ForInterface(clientAddr.IP)
		if localIPv4 != "" {
			responseIP = localIPv4
		}
	}

	// Build response
	response := map[string]interface{}{
		"service":      version.ServiceName,
		"version":      version.Version,
		"port":         18080,
		"ip":           responseIP,
		"is_running":   running,
		"delay_ms":     delayMs,
		"random_delay": randomDelay,
		"local_ips":    localIPs,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(response)
	if err != nil {
		log.Printf("[UDP] Failed to marshal discovery response: %v", err)
		return
	}

	// Send response back to client
	_, err = conn.WriteToUDP(jsonData, clientAddr)
	if err != nil {
		log.Printf("[UDP] Failed to send discovery response: %v", err)
	}
}

// getLocalIPv4ForInterface finds the local IPv4 address on the same subnet as the given IP
func getLocalIPv4ForInterface(remoteIP net.IP) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
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
			if ip.To4() == nil {
				continue
			}

			// Check if remote IP is in the same subnet
			if ipNet.Contains(remoteIP) {
				return ip.String()
			}
		}
	}

	return ""
}

// getLocalIPv6ForInterface finds a local IPv6 address for the interface
func getLocalIPv6ForInterface(remoteIP net.IP) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
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
			if ip.To4() != nil {
				continue // Skip IPv4
			}

			// Skip link-local and loopback
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Check if remote IP is in the same subnet or if this is a global unicast
			if ipNet.Contains(remoteIP) || ip.IsGlobalUnicast() {
				return ip.String()
			}
		}
	}

	return ""
}
