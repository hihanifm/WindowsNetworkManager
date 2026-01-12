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

// logger is a unified logging interface that works for both console and service modes
type logger struct {
	useServiceLogger bool
}

var appLogger = &logger{useServiceLogger: false}

// logInfo logs an informational message
func (l *logger) Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if l.useServiceLogger && serviceLogger != nil {
		serviceLogger.Info(msg)
	} else {
		log.Printf("[INFO] %s", msg)
	}
}

// logError logs an error message
func (l *logger) Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if l.useServiceLogger && serviceLogger != nil {
		serviceLogger.Error(msg)
	} else {
		log.Printf("[ERROR] %s", msg)
	}
}

// logDebug logs a debug message
func (l *logger) Debug(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if l.useServiceLogger && serviceLogger != nil {
		serviceLogger.Info("[DEBUG] " + msg)
	} else {
		log.Printf("[DEBUG] %s", msg)
	}
}

// logHTTP logs HTTP request/response
func (l *logger) HTTP(method, path string, statusCode int, duration time.Duration) {
	msg := fmt.Sprintf("%s %s - %d - %v", method, path, statusCode, duration)
	if l.useServiceLogger && serviceLogger != nil {
		serviceLogger.Info("[HTTP] " + msg)
	} else {
		log.Printf("[HTTP] %s", msg)
	}
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
	appLogger.useServiceLogger = false
	appLogger.Info("Starting Windows Network Manager in console mode")
	appLogger.Info("Version: %s", version.Version)
	
	// Get executable directory for web files
	if err := os.Chdir(exeDir); err != nil {
		appLogger.Error("Failed to change directory: %v", err)
	} else {
		appLogger.Debug("Working directory: %s", exeDir)
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

	// Bind to all interfaces (0.0.0.0) to allow network access
	bindAddr := "0.0.0.0:" + *port
	appLogger.Info("Starting web server on http://localhost:%s", *port)

	// Get and display local IP addresses for network access
	localIPs := getLocalIPs()
	if len(localIPs) > 0 {
		appLogger.Info("Web interface accessible from network at:")
		for _, ip := range localIPs {
			appLogger.Info("  http://%s:%s", ip, *port)
		}
	} else {
		appLogger.Info("Note: Could not detect local IP addresses for network access")
		appLogger.Info("The web interface is still accessible at http://localhost:18080")
		appLogger.Info("To find your IP address, run: ipconfig")
		appLogger.Info("Or access http://localhost:18080/api/network from the web interface")
	}

	appLogger.Info("Note: This application requires Administrator privileges to intercept packets")
	appLogger.Info("Note: Windows Firewall may need to allow incoming connections on port %s", *port)
	appLogger.Info("To run as a Windows Service, use: WindowsNetworkManager.exe -service install")
	appLogger.Info("Server ready and listening on %s", bindAddr)

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		appLogger.Error("Failed to start server: %v", err)
		log.Fatalf("Failed to start server: %v", err)
	}
}

// logHTTPMiddleware wraps HTTP handlers with logging
func logHTTPMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// Call the actual handler
		next(rw, r)
		
		// Log the request
		duration := time.Since(start)
		appLogger.HTTP(r.Method, r.URL.Path, rw.statusCode, duration)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	appLogger.Debug("Serving index page")
	http.ServeFile(w, r, "./web/index.html")
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		appLogger.Debug("GET /api/config - retrieving current configuration")
		delayMutex.RLock()
		delayMs := currentDelay.Milliseconds()
		delayMutex.RUnlock()

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		appLogger.Debug("Current config - Delay: %d ms, Running: %v", delayMs, running)
		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:   delayMs,
			IsRunning: running,
		})

	case "POST":
		var req struct {
			DelayMs int64 `json:"delay_ms"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			appLogger.Error("POST /api/config - Invalid request: %v", err)
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		appLogger.Debug("POST /api/config - Setting delay to %d ms", req.DelayMs)

		if req.DelayMs < 0 || req.DelayMs > 10000 {
			appLogger.Error("POST /api/config - Invalid delay value: %d ms", req.DelayMs)
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
			appLogger.Debug("Updated delay in running packet engine")
		} else {
			appLogger.Debug("Packet engine not running, delay will be applied when started")
		}

		appLogger.Info("Delay updated to %d ms", req.DelayMs)

		runningMutex.RLock()
		running := isRunning
		runningMutex.RUnlock()

		json.NewEncoder(w).Encode(ConfigResponse{
			DelayMs:   req.DelayMs,
			IsRunning: running,
		})

	default:
		appLogger.Error("Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	appLogger.Debug("GET /api/stats - retrieving statistics")

	statsMutex.RLock()
	stats := *packetStats
	statsMutex.RUnlock()

	uptime := time.Since(stats.StartTime).Seconds()

	appLogger.Debug("Stats - Packets: %d, Delayed: %d, Bytes: %d, Uptime: %.2fs",
		stats.TotalPackets, stats.DelayedPackets, stats.BytesProcessed, uptime)

	json.NewEncoder(w).Encode(StatsResponse{
		TotalPackets:   stats.TotalPackets,
		DelayedPackets: stats.DelayedPackets,
		BytesProcessed: stats.BytesProcessed,
		UptimeSeconds:  uptime,
	})
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	appLogger.Info("POST /api/start - Starting packet interception")

	runningMutex.Lock()
	if isRunning {
		runningMutex.Unlock()
		appLogger.Error("Packet interception already running")
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

	appLogger.Debug("Starting packet engine with delay: %v", delay)

	// Start packet interception
	var err error
	packetEngine, err = NewPacketEngine(delay)
	if err != nil {
		runningMutex.Lock()
		isRunning = false
		runningMutex.Unlock()
		appLogger.Error("Failed to start packet interception: %v", err)
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: fmt.Sprintf("Failed to start packet interception: %v", err),
		})
		return
	}

	go packetEngine.Start()

	appLogger.Info("Packet interception started successfully")
	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning: true,
	})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	appLogger.Info("POST /api/stop - Stopping packet interception")

	runningMutex.Lock()
	if !isRunning {
		runningMutex.Unlock()
		appLogger.Error("Packet interception is not running")
		json.NewEncoder(w).Encode(ConfigResponse{
			Error: "Packet interception is not running",
		})
		return
	}
	isRunning = false
	runningMutex.Unlock()

	// Stop packet interception
	if packetEngine != nil {
		appLogger.Debug("Stopping packet engine")
		packetEngine.Stop()
		packetEngine = nil
		appLogger.Debug("Packet engine stopped")
	}

	appLogger.Info("Packet interception stopped successfully")
	json.NewEncoder(w).Encode(ConfigResponse{
		IsRunning: false,
	})
}

func handleNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	appLogger.Debug("GET /api/network - Retrieving network information")

	localIPs := getLocalIPs()
	appLogger.Debug("Detected %d local IP addresses", len(localIPs))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"local_ips": localIPs,
		"port":      "18080",
	})
}

func handleDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	appLogger.Debug("GET /api/discover - Discovery request from %s", r.RemoteAddr)

	localIPs := getLocalIPs()
	runningMutex.RLock()
	running := isRunning
	runningMutex.RUnlock()

	delayMutex.RLock()
	delayMs := currentDelay.Milliseconds()
	delayMutex.RUnlock()

	appLogger.Debug("Discovery response - Version: %s, Running: %v, Delay: %d ms", version.Version, running, delayMs)
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
	total := packetStats.TotalPackets
	statsMutex.Unlock()
	
	// Log stats periodically (every 1000 packets)
	if total%1000 == 0 && total > 0 {
		appLogger.Debug("Packet stats - Total: %d, Delayed: %d, Bytes: %d", 
			packetStats.TotalPackets, packetStats.DelayedPackets, packetStats.BytesProcessed)
	}
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

	status := GetUpgradeStatus()
	json.NewEncoder(w).Encode(status)
}
