package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type InstanceInfo struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Service   string `json:"service"`
	Version   string `json:"version,omitempty"`
	IsRunning bool   `json:"is_running"`
	DelayMs   int64  `json:"delay_ms"`
	LocalIPs  []string `json:"local_ips,omitempty"`
}

type DiscoverResponse struct {
	Service    string   `json:"service"`
	Version    string   `json:"version,omitempty"`
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
		IsRunning: discoverResp.IsRunning,
		DelayMs:   discoverResp.DelayMs,
		LocalIPs:  discoverResp.LocalIPs,
	}, nil
}

const (
	DiscoveryPort  = 18081
	DiscoveryMagic = "WNM-DISCOVER-v1"
	IPv6Multicast  = "ff02::1"
)

// discoverViaBroadcast discovers instances using UDP broadcast (IPv4) and multicast (IPv6)
func discoverViaBroadcast(timeout time.Duration) ([]InstanceInfo, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var instances []InstanceInfo
	var errors []error

	// Channel to collect discovered instances
	instanceChan := make(chan InstanceInfo, 100)

	// Start IPv4 broadcast discovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		ipv4Instances, err := discoverIPv4Broadcast(timeout)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("IPv4 broadcast: %v", err))
			mu.Unlock()
		} else {
			for _, inst := range ipv4Instances {
				instanceChan <- inst
			}
		}
	}()

	// Start IPv6 multicast discovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		ipv6Instances, err := discoverIPv6Multicast(timeout)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("IPv6 multicast: %v", err))
			mu.Unlock()
		} else {
			for _, inst := range ipv6Instances {
				instanceChan <- inst
			}
		}
	}()

	// Close channel when both goroutines are done
	go func() {
		wg.Wait()
		close(instanceChan)
	}()

	// Collect all instances
	seenIPs := make(map[string]bool)
	for inst := range instanceChan {
		// Deduplicate by IP address
		key := fmt.Sprintf("%s:%d", inst.IP, inst.Port)
		if !seenIPs[key] {
			seenIPs[key] = true
			instances = append(instances, inst)
		}
	}

	// Log errors but don't fail if at least one method worked
	if len(errors) > 0 && len(instances) == 0 {
		return nil, fmt.Errorf("broadcast discovery failed: %v", errors)
	}

	return instances, nil
}

// discoverIPv4Broadcast sends UDP broadcast packets on IPv4 subnets
func discoverIPv4Broadcast(timeout time.Duration) ([]InstanceInfo, error) {
	subnets, err := getAllLocalSubnets()
	if err != nil {
		return nil, err
	}

	var instances []InstanceInfo
	var mu sync.Mutex

	// Create UDP socket for receiving responses
	recvAddr, err := net.ResolveUDPAddr("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve receive address: %v", err)
	}

	recvConn, err := net.ListenUDP("udp4", recvAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %v", err)
	}
	defer recvConn.Close()

	// Set read timeout
	recvConn.SetReadDeadline(time.Now().Add(timeout))

	// Channel to signal when we're done sending
	done := make(chan bool)
	responseChan := make(chan InstanceInfo, 100)

	// Goroutine to receive responses
	go func() {
		buffer := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
				recvConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, addr, err := recvConn.ReadFromUDP(buffer)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}

				// Parse JSON response
				var response map[string]interface{}
				if err := json.Unmarshal(buffer[:n], &response); err != nil {
					continue
				}

				// Verify it's WindowsNetworkManager
				if service, ok := response["service"].(string); !ok || service != ServiceName {
					continue
				}

				// Extract instance info
				instance := InstanceInfo{
					IP:        addr.IP.String(),
					Port:      DefaultPort,
					Service:   ServiceName,
					IsRunning: false,
				}

				if version, ok := response["version"].(string); ok {
					instance.Version = version
				}
				if port, ok := response["port"].(float64); ok {
					instance.Port = int(port)
				}
				if running, ok := response["is_running"].(bool); ok {
					instance.IsRunning = running
				}
				if delay, ok := response["delay_ms"].(float64); ok {
					instance.DelayMs = int64(delay)
				}
				if localIPs, ok := response["local_ips"].([]interface{}); ok {
					for _, ip := range localIPs {
						if ipStr, ok := ip.(string); ok {
							instance.LocalIPs = append(instance.LocalIPs, ipStr)
						}
					}
				}

				// Use IP from response if available
				if respIP, ok := response["ip"].(string); ok && respIP != "" {
					instance.IP = respIP
				}

				responseChan <- instance
			}
		}
	}()

	// Send broadcast packets to each subnet
	for _, subnet := range subnets {
		// Only process IPv4 subnets
		if strings.Contains(subnet.CIDR, ":") {
			continue
		}

		// Calculate broadcast address
		broadcastIP := getBroadcastAddress(subnet.CIDR)
		if broadcastIP == "" {
			continue
		}

		// Create UDP connection for sending
		sendAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", broadcastIP, DiscoveryPort))
		if err != nil {
			continue
		}

		sendConn, err := net.DialUDP("udp4", nil, sendAddr)
		if err != nil {
			continue
		}

		// Send discovery packet
		_, err = sendConn.Write([]byte(DiscoveryMagic))
		sendConn.Close()
		if err != nil {
			continue
		}
	}

	// Wait for responses
	time.Sleep(timeout)

	// Signal receiver to stop
	close(done)

	// Collect responses
	for {
		select {
		case inst := <-responseChan:
			mu.Lock()
			instances = append(instances, inst)
			mu.Unlock()
		default:
			return instances, nil
		}
	}
}

// discoverIPv6Multicast sends UDP multicast packets on IPv6
func discoverIPv6Multicast(timeout time.Duration) ([]InstanceInfo, error) {
	var instances []InstanceInfo
	var mu sync.Mutex

	// Create UDP socket for receiving responses
	recvAddr, err := net.ResolveUDPAddr("udp6", "[::]:0")
	if err != nil {
		// IPv6 not available, return empty
		return nil, nil
	}

	recvConn, err := net.ListenUDP("udp6", recvAddr)
	if err != nil {
		// IPv6 not available, return empty
		return nil, nil
	}
	defer recvConn.Close()

	// Set read timeout
	recvConn.SetReadDeadline(time.Now().Add(timeout))

	// Channel to signal when we're done sending
	done := make(chan bool)
	responseChan := make(chan InstanceInfo, 100)

	// Goroutine to receive responses
	go func() {
		buffer := make([]byte, 4096)
		for {
			select {
			case <-done:
				return
			default:
				recvConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, addr, err := recvConn.ReadFromUDP(buffer)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
				}

				// Parse JSON response
				var response map[string]interface{}
				if err := json.Unmarshal(buffer[:n], &response); err != nil {
					continue
				}

				// Verify it's WindowsNetworkManager
				if service, ok := response["service"].(string); !ok || service != ServiceName {
					continue
				}

				// Extract instance info
				instance := InstanceInfo{
					IP:        addr.IP.String(),
					Port:      DefaultPort,
					Service:   ServiceName,
					IsRunning: false,
				}

				if version, ok := response["version"].(string); ok {
					instance.Version = version
				}
				if port, ok := response["port"].(float64); ok {
					instance.Port = int(port)
				}
				if running, ok := response["is_running"].(bool); ok {
					instance.IsRunning = running
				}
				if delay, ok := response["delay_ms"].(float64); ok {
					instance.DelayMs = int64(delay)
				}
				if localIPs, ok := response["local_ips"].([]interface{}); ok {
					for _, ip := range localIPs {
						if ipStr, ok := ip.(string); ok {
							instance.LocalIPs = append(instance.LocalIPs, ipStr)
						}
					}
				}

				// Use IP from response if available
				if respIP, ok := response["ip"].(string); ok && respIP != "" {
					instance.IP = respIP
				}

				responseChan <- instance
			}
		}
	}()

	// Send multicast packet
	multicastAddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s]:%d", IPv6Multicast, DiscoveryPort))
	if err != nil {
		return nil, nil
	}

	sendConn, err := net.DialUDP("udp6", nil, multicastAddr)
	if err != nil {
		// IPv6 not available, return empty
		return nil, nil
	}

	// Send discovery packet
	_, err = sendConn.Write([]byte(DiscoveryMagic))
	sendConn.Close()
	if err != nil {
		return nil, nil
	}

	// Wait for responses
	time.Sleep(timeout)

	// Signal receiver to stop
	close(done)

	// Collect responses
	for {
		select {
		case inst := <-responseChan:
			mu.Lock()
			instances = append(instances, inst)
			mu.Unlock()
		default:
			return instances, nil
		}
	}
}

// getBroadcastAddress calculates the broadcast address for a CIDR subnet
func getBroadcastAddress(cidr string) string {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}

	// Only handle IPv4
	ip := ipNet.IP
	if ip.To4() == nil {
		return ""
	}

	// Calculate broadcast address
	mask := ipNet.Mask
	broadcast := make(net.IP, len(ip))
	copy(broadcast, ip)

	for i := range ip {
		broadcast[i] = ip[i] | ^mask[i]
	}

	return broadcast.String()
}
