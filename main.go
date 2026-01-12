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
	currentDelay time.Duration
	delayMutex   sync.RWMutex
	packetStats  = &PacketStats{}
	statsMutex   sync.RWMutex
	isRunning    bool
	runningMutex sync.RWMutex
	packetEngine *PacketEngine
)

type PacketStats struct {
	TotalPackets   uint64    `json:"total_packets"`
	DelayedPackets uint64    `json:"delayed_packets"`
	BytesProcessed uint64    `json:"bytes_processed"`
	StartTime      time.Time `json:"start_time"`
}

type ConfigResponse struct {
	DelayMs   int64  `json:"delay_ms"`
	IsRunning bool   `json:"is_running"`
	Error     string `json:"error,omitempty"`
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

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:   delayMs,
			IsRunning: running,
		})

	case "POST":
		var req struct {
			DelayMs int64 `json:"delay_ms"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[ERROR] Invalid config request: %v", err)
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		log.Printf("[HTTP] POST /api/config - Setting delay to %d ms", req.DelayMs)

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

		// Update delay in running packet engine
		if packetEngine != nil {
			packetEngine.SetDelay(currentDelay)
			log.Printf("[INFO] Delay updated in running packet engine")
		} else {
			log.Printf("[INFO] Delay will be applied when packet interception starts")
		}

		log.Printf("[INFO] Delay updated to %d ms", req.DelayMs)

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:   req.DelayMs,
			IsRunning: running,
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

	// Get current delay
	delayMutex.RLock()
	delay := currentDelay
	delayMutex.RUnlock()

	log.Printf("Attempting to start packet interception with delay: %v", delay)

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

	go packetEngine.Start()

	log.Println("[INFO] Packet interception started successfully")
	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning: true,
	})
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
	isRunning = false
	runningMutex.Unlock()

	// Stop packet interception
	if packetEngine != nil {
		log.Println("Stopping packet engine...")
		packetEngine.Stop()
		packetEngine = nil
		log.Println("[INFO] Packet engine stopped")
	}

	log.Println("[INFO] Packet interception stopped successfully")
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"service":    version.ServiceName,
		"version":    version.Version,
		"port":       18080,
		"local_ips":  localIPs,
		"is_running": running,
		"delay_ms":   delayMs,
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
	// Download
	if err := DownloadUpdate(downloadURL); err != nil {
		return
	}

	// Install
	if err := InstallUpdate(); err != nil {
		return
	}
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
