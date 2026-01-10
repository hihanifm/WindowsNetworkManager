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
	"sync"
	"time"

	"github.com/kardianos/service"
	"WindowsNetworkManager/version"
)

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
	flag.Parse()

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
	}

	log.Println("Note: This application requires Administrator privileges to intercept packets")
	log.Println("Note: Windows Firewall may need to allow incoming connections on port", *port)
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
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		if req.DelayMs < 0 || req.DelayMs > 10000 {
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
		}

		log.Printf("Delay updated to %d ms", req.DelayMs)

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:   req.DelayMs,
			IsRunning: running,
		})

	default:
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

	runningMutex.Lock()
	if isRunning {
		runningMutex.Unlock()
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

	// Start packet interception
	var err error
	packetEngine, err = NewPacketEngine(delay)
	if err != nil {
		runningMutex.Lock()
		isRunning = false
		runningMutex.Unlock()
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: fmt.Sprintf("Failed to start packet interception: %v", err),
		})
		return
	}

	go packetEngine.Start()

	log.Println("Packet interception started")
	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning: true,
	})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	runningMutex.Lock()
	if !isRunning {
		runningMutex.Unlock()
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: "Packet interception is not running",
		})
		return
	}
	isRunning = false
	runningMutex.Unlock()

	// Stop packet interception
	if packetEngine != nil {
		packetEngine.Stop()
		packetEngine = nil
	}

	log.Println("Packet interception stopped")
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
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
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

	status := GetUpgradeStatus()
	json.NewEncoder(w).Encode(status)
}
