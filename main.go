package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	currentDelay    time.Duration
	delayMutex      sync.RWMutex
	packetStats     = &PacketStats{}
	statsMutex      sync.RWMutex
	isRunning       bool
	runningMutex    sync.RWMutex
	packetEngine    *PacketEngine
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
	port := flag.String("port", "8080", "Web server port")
	flag.Parse()

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

	log.Printf("Starting web server on http://localhost:%s", *port)
	log.Println("Note: This application requires Administrator privileges to intercept packets")
	
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
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

func updateStats(packets, bytes uint64) {
	statsMutex.Lock()
	packetStats.TotalPackets += packets
	packetStats.DelayedPackets += packets
	packetStats.BytesProcessed += bytes
	statsMutex.Unlock()
}

