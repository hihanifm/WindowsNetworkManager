package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	scanMutex     sync.RWMutex
	currentScan   *ScanResult
	instancesList []InstanceInfo
	instancesMutex sync.RWMutex
)

type ScanResult struct {
	Status      string    `json:"status"` // "idle", "scanning", "completed"
	Progress    string    `json:"progress"`
	NetworkInfo string    `json:"network_info,omitempty"` // Current network being scanned
	Instances   []InstanceInfo `json:"instances"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type ScanRequest struct {
	Workers int           `json:"workers,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty"`
}

// startWebServer starts the web server
func startWebServer(port int) {
	// Setup routes
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/scan", handleScan)
	http.HandleFunc("/api/instances", handleInstances)
	http.HandleFunc("/api/status", handleStatus)

	// Serve static files - use executable directory to find web files
	var staticDir string
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		staticPath := filepath.Join(exeDir, "web", "static")
		if _, err := os.Stat(staticPath); err == nil {
			staticDir = staticPath
		}
	}
	if staticDir == "" {
		staticDir = "./web/static"
	}
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	bindAddr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("Scanner web server starting on http://localhost:%d", port)
	
	// Get and display local IPs
	localIPs := getLocalIPs()
	if len(localIPs) > 0 {
		log.Println("Web interface accessible from network at:")
		for _, ip := range localIPs {
			log.Printf("  http://%s:%d", ip, port)
		}
	}

	if err := http.ListenAndServe(bindAddr, nil); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	// Only serve the UI at "/" (and "/index.html"). For any unknown path,
	// return 404 so API clients don't receive HTML unexpectedly.
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}

	// Get executable directory to ensure we serve files relative to the binary location
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		indexPath := filepath.Join(exeDir, "web", "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
	}
	// Fallback to relative path
	http.ServeFile(w, r, "./web/index.html")
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if scan is already running
	scanMutex.RLock()
	if currentScan != nil && currentScan.Status == "scanning" {
		scanMutex.RUnlock()
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Scan already in progress",
		})
		return
	}
	scanMutex.RUnlock()

	// Parse request
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = ScanRequest{} // Use defaults
	}

	workers := req.Workers
	if workers == 0 {
		workers = DefaultWorkers
	}

	timeout := req.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	// Start scan in background
	go runScan(workers, timeout)

	json.NewEncoder(w).Encode(map[string]string{
		"status": "scan_started",
	})
}

func runScan(workers int, timeout time.Duration) {
	scanMutex.Lock()
	currentScan = &ScanResult{
		Status:   "scanning",
		Progress: "Initializing scan...",
	}
	scanMutex.Unlock()

	instancesMutex.Lock()
	instancesList = nil
	instancesMutex.Unlock()

	// Channel to signal scan completion so we can stop progress updates
	scanComplete := make(chan struct{})
	
	// Create progress callback to update scan progress in real-time
	progressCallback := func(scanned, total, found int, message string) {
		// Check if scan has already completed to avoid race conditions
		select {
		case <-scanComplete:
			// Scan already completed, ignore this callback
			return
		default:
		}
		
		scanMutex.Lock()
		if currentScan != nil && currentScan.Status == "scanning" {
			currentScan.Progress = message
			// Extract network info from message (format: "Network X/Y: CIDR | ...")
			if networkMatch := extractNetworkInfo(message); networkMatch != "" {
				currentScan.NetworkInfo = networkMatch
			}
		}
		scanMutex.Unlock()
	}

	onFound := func(inst InstanceInfo) {
		instancesMutex.Lock()
		instancesList = append(instancesList, inst)
		instancesMutex.Unlock()
	}

	// Run the actual scan with progress callback
	instances, err := ScanNetworkWithProgress(workers, timeout, progressCallback, onFound)

	// Signal that scan is complete to stop progress callbacks
	close(scanComplete)
	
	// Small delay to ensure any final progress callbacks have completed
	time.Sleep(150 * time.Millisecond)

	scanMutex.Lock()
	defer scanMutex.Unlock()
	
	if err != nil {
		currentScan = &ScanResult{
			Status: "error",
			Error:  err.Error(),
		}
		log.Printf("Scan failed: %v", err)
	} else {
		currentScan = &ScanResult{
			Status:      "completed",
			Progress:    fmt.Sprintf("Scan complete! Found %d instance(s)", len(instances)),
			Instances:   instances,
			CompletedAt: time.Now(),
		}
		log.Printf("Scan completed successfully: found %d instance(s)", len(instances))
		// Update instances list
		instancesMutex.Lock()
		instancesList = instances
		instancesMutex.Unlock()
	}
}

func handleInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scanMutex.RLock()
	scanStatus := currentScan
	scanMutex.RUnlock()

	instancesMutex.RLock()
	instances := instancesList
	instancesMutex.RUnlock()

	// If scan is completed, prefer instances from scan result
	if scanStatus != nil && scanStatus.Status == "completed" && len(scanStatus.Instances) > 0 {
		instances = scanStatus.Instances
	}

	response := map[string]interface{}{
		"scan":      scanStatus,
		"instances": instances,
	}

	json.NewEncoder(w).Encode(response)
}

// getLocalIPs returns local IP addresses for network access
func getLocalIPs() []string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
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

			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	return ips
}

// handleStatus returns scanner status and version information
func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"service": ServiceName,
		"version": Version,
		"port":    DefaultWebPort,
	}

	json.NewEncoder(w).Encode(response)
}

// extractNetworkInfo extracts network CIDR from progress message
func extractNetworkInfo(message string) string {
	// Look for patterns like "Network X/Y: CIDR |" or "Scanning network X/Y: CIDR"
	// Try to extract the CIDR part
	if idx := strings.Index(message, ":"); idx != -1 {
		afterColon := message[idx+1:]
		if idx2 := strings.Index(afterColon, "|"); idx2 != -1 {
			networkPart := strings.TrimSpace(afterColon[:idx2])
			// Check if it looks like a CIDR (contains /)
			if strings.Contains(networkPart, "/") {
				return networkPart
			}
		}
		// Try to find CIDR pattern directly
		parts := strings.Fields(afterColon)
		for _, part := range parts {
			if strings.Contains(part, "/") {
				return part
			}
		}
	}
	return ""
}
