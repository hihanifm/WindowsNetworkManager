package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ScanNetwork scans the local network for WindowsNetworkManager instances
func ScanNetwork(workers int, timeout time.Duration) ([]InstanceInfo, error) {
	subnet, err := getLocalSubnet()
	if err != nil {
		return nil, fmt.Errorf("failed to detect local subnet: %v", err)
	}

	// Note: Console output suppressed when called from web server
	// (output is handled via web_server.go progress updates)

	ipRange, err := parseSubnet(subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subnet: %v", err)
	}

	return scanIPRange(ipRange, DefaultPort, workers, timeout)
}

// getLocalSubnet detects the local network subnet
func getLocalSubnet() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

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
			// Only consider IPv4 addresses
			if ip.To4() == nil {
				continue
			}

			// Skip loopback and link-local
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Calculate subnet
			mask := ipNet.Mask
			ones, _ := mask.Size()
			if ones > 0 && ones <= 24 {
				// Return subnet in CIDR notation
				return fmt.Sprintf("%s/%d", ipNet.IP.Mask(mask).String(), ones), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable network interface found")
}

// parseSubnet parses a CIDR subnet and returns the IP range
func parseSubnet(subnet string) ([]string, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, err
	}

	var ips []string
	
	// Get network and broadcast addresses
	ones, bits := ipNet.Mask.Size()
	if ones == 0 || ones > 24 {
		return nil, fmt.Errorf("subnet too large or invalid")
	}

	// Calculate number of IPs (excluding network and broadcast for /24)
	hostBits := bits - ones
	if hostBits > 8 {
		hostBits = 8 // Limit to /24 for safety
	}
	
	maxIPs := 1 << hostBits
	if maxIPs > 254 {
		maxIPs = 254 // Limit to reasonable range
	}

	// Generate IP addresses (skip .0 and .255 for /24)
	for i := 1; i < maxIPs && i < 255; i++ {
		ip := make(net.IP, len(ipNet.IP))
		copy(ip, ipNet.IP)
		
		// Increment IP address
		ip[3] = byte(i)
		
		ips = append(ips, ip.String())
	}

	return ips, nil
}

// scanIPRange scans a range of IP addresses in parallel
func scanIPRange(ipRange []string, port int, workers int, timeout time.Duration) ([]InstanceInfo, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var instances []InstanceInfo
	
	// Channel for IPs to scan
	ipChan := make(chan string, len(ipRange))
	
	// Fill channel with IPs
	for _, ip := range ipRange {
		ipChan <- ip
	}
	close(ipChan)

	// Progress tracking
	var scanned int
	var found int
	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()

	// Progress reporting is handled by web_server.go when called from web interface
	// Console output suppressed for cleaner web experience

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipChan {
				instance, err := checkInstance(ip, port, timeout)
				mu.Lock()
				scanned++
				if err == nil && instance != nil {
					instances = append(instances, *instance)
					found++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	progressTicker.Stop()

	return instances, nil
}
