package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
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

	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
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
		Progress: "Detecting network...",
	}
	scanMutex.Unlock()

	// Progress updates handled via status polling from frontend

	// Run the actual scan
	instances, err := ScanNetwork(workers, timeout)

	scanMutex.Lock()
	if err != nil {
		currentScan = &ScanResult{
			Status: "error",
			Error:  err.Error(),
		}
	} else {
		currentScan = &ScanResult{
			Status:      "completed",
			Progress:    fmt.Sprintf("Found %d instance(s)", len(instances)),
			Instances:   instances,
			CompletedAt: time.Now(),
		}
		// Update instances list
		instancesMutex.Lock()
		instancesList = instances
		instancesMutex.Unlock()
	}
	scanMutex.Unlock()
}

func handleInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scanMutex.RLock()
	scanStatus := currentScan
	scanMutex.RUnlock()

	instancesMutex.RLock()
	instances := instancesList
	instancesMutex.RUnlock()

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
