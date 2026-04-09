package main

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// ProgressCallback is a function type for reporting scan progress
type ProgressCallback func(scanned, total, found int, message string)

// FoundCallback is invoked for each discovered instance as the scan runs.
type FoundCallback func(InstanceInfo)

// ScanNetwork scans the local network for WindowsNetworkManager instances
func ScanNetwork(workers int, timeout time.Duration) ([]InstanceInfo, error) {
	return ScanNetworkWithProgress(workers, timeout, nil, nil)
}

// ScanNetworkWithProgress scans the local network with progress reporting
func ScanNetworkWithProgress(workers int, timeout time.Duration, progressCallback ProgressCallback, onFound FoundCallback) ([]InstanceInfo, error) {
	subnets, err := getAllLocalSubnets()
	if err != nil {
		return nil, fmt.Errorf("failed to detect local subnets: %v", err)
	}

	if len(subnets) == 0 {
		return nil, fmt.Errorf("no network interfaces found")
	}

	var allInstances []InstanceInfo
	totalIPs := 0
	scannedIPs := 0
	foundCount := 0

	// Scan each subnet
	for i, subnet := range subnets {
		ipRange, err := parseSubnet(subnet.CIDR)
		if err != nil {
			if progressCallback != nil {
				progressCallback(scannedIPs, totalIPs, foundCount, 
					fmt.Sprintf("Skipping network %s: %v", subnet.CIDR, err))
			}
			continue
		}

		totalIPs += len(ipRange)
		
		if progressCallback != nil {
			networkInfo := fmt.Sprintf("Scanning network %d/%d: %s (range: %s - %s, %d IPs)", 
				i+1, len(subnets), subnet.CIDR, subnet.FirstIP, subnet.LastIP, len(ipRange))
			progressCallback(scannedIPs, totalIPs, foundCount, networkInfo)
		}

		// Create progress callback for this subnet
		subnetProgressCallback := func(scanned, total, found int, message string) {
			currentScanned := scannedIPs + scanned
			currentFound := foundCount + found
			networkInfo := fmt.Sprintf("Network %d/%d: %s | %s", 
				i+1, len(subnets), subnet.CIDR, message)
			if progressCallback != nil {
				progressCallback(currentScanned, totalIPs, currentFound, networkInfo)
			}
		}

		instances, err := scanIPRange(ipRange, DefaultPort, workers, timeout, subnetProgressCallback, onFound)
		if err != nil {
			if progressCallback != nil {
				progressCallback(scannedIPs+len(ipRange), totalIPs, foundCount,
					fmt.Sprintf("Error scanning %s: %v", subnet.CIDR, err))
			}
			continue
		}

		scannedIPs += len(ipRange)
		foundCount += len(instances)
		allInstances = append(allInstances, instances...)
	}

	if progressCallback != nil {
		progressCallback(totalIPs, totalIPs, foundCount,
			fmt.Sprintf("Scan complete! Scanned %d network(s), found %d instance(s)", 
				len(subnets), len(allInstances)))
	}

	return allInstances, nil
}

// NetworkSubnet represents a network subnet with metadata
type NetworkSubnet struct {
	CIDR    string // e.g., "192.168.1.0/24"
	FirstIP string // First IP in range
	LastIP  string // Last IP in range
	Interface string // Network interface name
}

// getAllLocalSubnets detects all local network subnets
func getAllLocalSubnets() ([]NetworkSubnet, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var subnets []NetworkSubnet

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
			ones, bits := mask.Size()
			if ones > 0 && ones <= 24 {
				networkIP := ipNet.IP.Mask(mask)
				cidr := fmt.Sprintf("%s/%d", networkIP.String(), ones)
				
				// Calculate first and last IP
				hostBits := bits - ones
				if hostBits > 8 {
					hostBits = 8 // Limit to /24
				}
				
				firstIP := make(net.IP, len(networkIP))
				copy(firstIP, networkIP)
				firstIP[3] = 1 // First usable IP
				
				lastIP := make(net.IP, len(networkIP))
				copy(lastIP, networkIP)
				maxHosts := (1 << hostBits) - 2 // Exclude .0 and .255
				if maxHosts > 254 {
					maxHosts = 254
				}
				lastIP[3] = byte(maxHosts) // Last usable IP
				
				subnets = append(subnets, NetworkSubnet{
					CIDR:      cidr,
					FirstIP:   firstIP.String(),
					LastIP:    lastIP.String(),
					Interface: iface.Name,
				})
			}
		}
	}

	if len(subnets) == 0 {
		return nil, fmt.Errorf("no suitable network interface found")
	}

	return subnets, nil
}

// getLocalSubnet detects the first local network subnet (for backward compatibility)
func getLocalSubnet() (string, error) {
	subnets, err := getAllLocalSubnets()
	if err != nil {
		return "", err
	}
	if len(subnets) == 0 {
		return "", fmt.Errorf("no network interfaces found")
	}
	return subnets[0].CIDR, nil
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
func scanIPRange(ipRange []string, port int, workers int, timeout time.Duration, progressCallback ProgressCallback, onFound FoundCallback) ([]InstanceInfo, error) {
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
	totalIPs := len(ipRange)
	
	// Progress reporting ticker - updates every 200ms for smooth progress
	progressTicker := time.NewTicker(200 * time.Millisecond)
	defer progressTicker.Stop()
	
	// Channel to signal completion
	done := make(chan bool)
	
	// Progress reporter goroutine
	go func() {
		for {
			select {
			case <-progressTicker.C:
				mu.Lock()
				currentScanned := scanned
				currentFound := found
				mu.Unlock()
				
				if progressCallback != nil && currentScanned <= totalIPs {
					var message string
					if currentScanned < totalIPs {
						percentage := (currentScanned * 100) / totalIPs
						message = fmt.Sprintf("Scanning network... %d/%d IPs checked (%d%%) - %d instance(s) found", 
							currentScanned, totalIPs, percentage, currentFound)
					} else {
						// Final update when all IPs are scanned
						message = fmt.Sprintf("Finalizing scan... %d/%d IPs checked (100%%) - %d instance(s) found", 
							currentScanned, totalIPs, currentFound)
					}
					progressCallback(currentScanned, totalIPs, currentFound, message)
				}
			case <-done:
				return
			}
		}
	}()

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range ipChan {
				instance, err := checkInstance(ip, port, timeout)
				var foundInst InstanceInfo
				var reportFound bool
				mu.Lock()
				scanned++
				if err == nil && instance != nil {
					instances = append(instances, *instance)
					found++
					foundInst = *instance
					reportFound = true
				}
				mu.Unlock()
				if reportFound && onFound != nil {
					onFound(foundInst)
				}
			}
		}()
	}

	wg.Wait()
	
	// Get final counts with lock
	mu.Lock()
	finalScanned := scanned
	finalFound := found
	mu.Unlock()
	
	// Send final progress update (100%) before stopping ticker
	if progressCallback != nil {
		finalMessage := fmt.Sprintf("Scanning network... %d/%d IPs checked (100%%) - %d instance(s) found", 
			finalScanned, totalIPs, finalFound)
		progressCallback(finalScanned, totalIPs, finalFound, finalMessage)
	}
	
	// Stop ticker and signal completion
	progressTicker.Stop()
	done <- true
	
	// Small delay to let progress reporter exit cleanly
	time.Sleep(100 * time.Millisecond)

	return instances, nil
}
