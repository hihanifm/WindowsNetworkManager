package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type InstanceInfo struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Service   string `json:"service"`
	Version   string `json:"version,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	IsRunning bool   `json:"is_running"`
	DelayMs   int64  `json:"delay_ms"`
	LocalIPs  []string `json:"local_ips,omitempty"`
}

type DiscoverResponse struct {
	Service    string   `json:"service"`
	Version    string   `json:"version,omitempty"`
	Hostname   string   `json:"hostname,omitempty"`
	Port       int      `json:"port"`
	LocalIPs   []string `json:"local_ips"`
	IsRunning  bool     `json:"is_running"`
	DelayMs    int64    `json:"delay_ms"`
}

// checkInstance checks if an IP address is running WindowsNetworkManager
func checkInstance(ip string, port int, timeout time.Duration) (*InstanceInfo, error) {
	url := fmt.Sprintf("http://%s:%d/api/discover", ip, port)
	
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var discoverResp DiscoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&discoverResp); err != nil {
		return nil, err
	}

	// Verify it's actually WindowsNetworkManager
	if discoverResp.Service != ServiceName {
		return nil, fmt.Errorf("not a WindowsNetworkManager instance")
	}

	return &InstanceInfo{
		IP:        ip,
		Port:      port,
		Service:   discoverResp.Service,
		Version:   discoverResp.Version,
		Hostname:  discoverResp.Hostname,
		IsRunning: discoverResp.IsRunning,
		DelayMs:   discoverResp.DelayMs,
		LocalIPs:  discoverResp.LocalIPs,
	}, nil
}
