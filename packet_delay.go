package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/deblasis/godivert"
)

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// PacketEngine handles packet interception and delay
type PacketEngine struct {
	handle          *godivert.WinDivertHandle
	delay           time.Duration
	delayMutex      sync.RWMutex
	randomDelay     bool
	randomDelayMutex sync.RWMutex
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// initWinDivertDLL initializes the WinDivert DLL by loading it from the executable directory
func initWinDivertDLL() error {
	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	
	// Construct DLL path - DLL should be in the same directory as the executable
	dllPath := filepath.Join(exeDir, "WinDivert.dll")
	
	// Check if DLL file exists
	if _, err := os.Stat(dllPath); os.IsNotExist(err) {
		return fmt.Errorf("WinDivert.dll not found at: %s. Please ensure WinDivert.dll is in the same directory as WindowsNetworkManager.exe", dllPath)
	}
	
	// Check for driver file (.sys) - WinDivert requires both DLL and driver
	var driverPath string
	switch runtime.GOARCH {
	case "amd64":
		driverPath = filepath.Join(exeDir, "WinDivert64.sys")
	case "arm64":
		driverPath = filepath.Join(exeDir, "WinDivert64.sys") // ARM64 also uses 64-bit driver
	default:
		driverPath = filepath.Join(exeDir, "WinDivert32.sys")
	}
	
	// Note: Driver file check is informational - the driver might be in system32
	// But we should warn if it's not in the executable directory
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		log.Printf("Warning: WinDivert driver file (%s) not found in executable directory", filepath.Base(driverPath))
		log.Printf("Note: The driver file may be in System32 or loaded from another location")
		log.Printf("If you encounter 'invalid handle' errors, ensure WinDivert driver is properly installed")
	} else {
		log.Printf("WinDivert driver file found: %s", driverPath)
	}
	
	// For LoadDLL, we need to provide both 64-bit and 32-bit paths
	var path64, path32 string
	
	switch runtime.GOARCH {
	case "amd64":
		// x64 (AMD64) architecture
		path64 = dllPath
		path32 = dllPath // Required parameter, but won't be used
	case "arm64":
		// ARM64 architecture - also 64-bit
		path64 = dllPath
		path32 = dllPath
	default:
		return fmt.Errorf("unsupported architecture: %s (only amd64 and arm64 are supported)", runtime.GOARCH)
	}
	
	// Load the DLL
	if err := godivert.LoadDLL(path64, path32); err != nil {
		return fmt.Errorf("failed to load WinDivert DLL from %s (architecture: %s): %v. Please verify the DLL is correct for your system architecture", dllPath, runtime.GOARCH, err)
	}
	
	log.Printf("WinDivert DLL loaded successfully from: %s (architecture: %s)", dllPath, runtime.GOARCH)
	return nil
}

// NewPacketEngine creates a new packet engine instance
func NewPacketEngine(delay time.Duration) (*PacketEngine, error) {
	log.Printf("Initializing packet engine with delay: %v", delay)
	
	// Ensure DLL is loaded before creating handle
	if !godivert.IsDLLLoaded() {
		log.Printf("DLL not loaded, initializing...")
		if err := initWinDivertDLL(); err != nil {
			return nil, fmt.Errorf("WinDivert DLL not initialized: %v", err)
		}
	} else {
		log.Printf("WinDivert DLL already loaded")
	}
	
	// WinDivert filter: capture all outbound packets
	// "outbound" captures all outgoing packets
	filter := "outbound"

	log.Printf("Creating WinDivert handle with filter: %s", filter)
	log.Printf("Note: This requires Administrator privileges")
	
	handle, err := godivert.NewWinDivertHandle(filter)
	if err != nil {
		// Provide more helpful error message based on error type
		errStr := err.Error()
		
		if errStr == "WinDivert DLL not loaded" {
			return nil, fmt.Errorf("WinDivert DLL not loaded. Please ensure WinDivert.dll is in the same directory as WindowsNetworkManager.exe. Error: %v", err)
		}
		
		// Check for access denied (errno 5)
		if contains(errStr, "access denied") || contains(errStr, "errno: 5") {
			return nil, fmt.Errorf("Access denied: You must run as Administrator. WinDivert requires Administrator privileges to create a handle. Please right-click and select 'Run as Administrator'")
		}
		
		// Check for driver not installed (errno 577)
		if contains(errStr, "driver not installed") || contains(errStr, "errno: 577") {
			return nil, fmt.Errorf("WinDivert driver not installed or started. The WinDivert.sys driver file may be missing or the driver service is not running. Check Windows Event Log for driver errors")
		}
		
		// Check for invalid parameter (errno 87)
		if contains(errStr, "invalid parameter") || contains(errStr, "errno: 87") {
			return nil, fmt.Errorf("Invalid filter parameter. Filter syntax error in: '%s'. This should not happen with 'outbound' filter", filter)
		}
		
		// Check for invalid handle errors
		if contains(errStr, "invalid handle") || contains(errStr, "INVALID_HANDLE") || contains(errStr, "handle is nil") {
			return nil, fmt.Errorf("WinDivert handle is invalid. Common causes: 1) Not running as Administrator, 2) WinDivert driver (WinDivert.sys) not installed, 3) Another application already using WinDivert, 4) Insufficient privileges, 5) WinDivert driver service not started. Original error: %v", err)
		}
		
		// Generic error with troubleshooting tips
		return nil, fmt.Errorf("failed to create WinDivert handle: %v. Troubleshooting: 1) Ensure you are running as Administrator, 2) Verify WinDivert.dll and WinDivert.sys are present, 3) Check Windows Event Log for driver errors, 4) Try restarting the application", err)
	}
	
	// Verify handle is valid
	if handle == nil {
		return nil, fmt.Errorf("WinDivert handle is nil after creation. This usually means: 1) Insufficient privileges (not running as Administrator), 2) WinDivert driver issue, 3) System resource limitation. Please run as Administrator and check Windows Event Log")
	}
	
	log.Printf("WinDivert handle created successfully")
	
	// Verify handle is actually valid by checking if it's not nil
	if handle == nil {
		return nil, fmt.Errorf("WinDivert handle is nil after creation - this should not happen")
	}
	
	// Small delay to ensure handle is fully initialized before use
	// Sometimes there's a brief delay between handle creation and it being ready
	time.Sleep(50 * time.Millisecond)
	
	log.Printf("Handle validation: Handle object created and stored, ready for use")

	return &PacketEngine{
		handle:   handle,
		delay:    delay,
		stopChan: make(chan struct{}),
	}, nil
}

// SetDelay updates the delay duration
func (pe *PacketEngine) SetDelay(delay time.Duration) {
	pe.delayMutex.Lock()
	pe.delay = delay
	pe.delayMutex.Unlock()
}

// GetDelay returns the current delay duration
func (pe *PacketEngine) GetDelay() time.Duration {
	pe.delayMutex.RLock()
	defer pe.delayMutex.RUnlock()
	return pe.delay
}

// SetRandomDelay updates the random delay mode
func (pe *PacketEngine) SetRandomDelay(randomDelay bool) {
	pe.randomDelayMutex.Lock()
	pe.randomDelay = randomDelay
	pe.randomDelayMutex.Unlock()
}

// GetRandomDelay returns the current random delay mode
func (pe *PacketEngine) GetRandomDelay() bool {
	pe.randomDelayMutex.RLock()
	defer pe.randomDelayMutex.RUnlock()
	return pe.randomDelay
}

// Start begins packet interception and delay processing
func (pe *PacketEngine) Start() {
	log.Println("Starting packet interception engine...")

	// Start packet processing goroutine
	pe.wg.Add(1)
	go pe.processPackets()

	// Wait for stop signal
	<-pe.stopChan
	pe.wg.Wait()
}

// Stop stops packet interception
func (pe *PacketEngine) Stop() {
	log.Println("Stopping packet interception engine...")
	close(pe.stopChan)
	pe.wg.Wait()

	if pe.handle != nil {
		pe.handle.Close()
		log.Println("Packet interception engine stopped")
	}
}

// processPackets captures packets, delays them, and reinjects them
func (pe *PacketEngine) processPackets() {
	defer pe.wg.Done()

	// Channel for delayed packets
	type delayedPacket struct {
		packet *godivert.Packet
		sendAt time.Time
	}

	packetQueue := make(chan *delayedPacket, 1000)

	// Goroutine to send delayed packets
	pe.wg.Add(1)
	go func() {
		defer pe.wg.Done()
		for {
			select {
			case <-pe.stopChan:
				// Send remaining packets before stopping
				for {
					select {
					case dp := <-packetQueue:
						pe.sendPacket(dp.packet)
					default:
						return
					}
				}
			case dp := <-packetQueue:
				now := time.Now()
				if dp.sendAt.After(now) {
					// Wait until it's time to send
					select {
					case <-time.After(dp.sendAt.Sub(now)):
						pe.sendPacket(dp.packet)
					case <-pe.stopChan:
						pe.sendPacket(dp.packet)
						return
					}
				} else {
					// Send immediately if delay has passed
					pe.sendPacket(dp.packet)
				}
			}
		}
	}()

	// Main packet capture loop
	for {
		select {
		case <-pe.stopChan:
			log.Printf("Packet receive stopped due to stop signal")
			return
		default:
			// Verify handle is still valid before receiving
			if pe.handle == nil {
				log.Printf("[ERROR] Handle is nil, cannot receive packets")
				return
			}
			
			// Receive packet
			packet, err := pe.handle.Recv()
			if err != nil {
				// Check if it's a timeout or stop condition
				select {
				case <-pe.stopChan:
					log.Printf("Packet receive stopped due to stop signal")
					return
				default:
					errStr := err.Error()
					// Check for invalid handle errors
					if contains(errStr, "invalid handle") || contains(errStr, "handle is invalid") || contains(errStr, "handle isn't open") {
						log.Printf("[ERROR] WinDivert handle became invalid: %v", err)
						log.Printf("[ERROR] This usually means: 1) Handle was closed, 2) WinDivert driver issue, 3) Insufficient privileges")
						log.Printf("[ERROR] Stopping packet interception due to invalid handle")
						return
					}
					log.Printf("[ERROR] Error receiving packet: %v", err)
					time.Sleep(10 * time.Millisecond) // Brief pause on error
					continue
				}
			}

			// Get current delay and random delay mode
			delay := pe.GetDelay()
			useRandomDelay := pe.GetRandomDelay()

			if delay > 0 {
				// Calculate actual delay to apply
				actualDelay := delay
				if useRandomDelay {
					// Generate random delay between 1ms and configured delay (inclusive)
					delayMs := delay.Milliseconds()
					if delayMs > 0 {
						randomMs := rand.Int63n(delayMs) + 1 // 1 to delayMs (inclusive)
						actualDelay = time.Duration(randomMs) * time.Millisecond
					}
				}

				// Queue packet for delayed sending
				dp := &delayedPacket{
					packet: packet,
					sendAt: time.Now().Add(actualDelay),
				}

				select {
				case packetQueue <- dp:
					// Packet queued successfully
				default:
					// Queue full, send immediately to avoid blocking
					log.Println("Warning: Packet queue full, sending immediately")
					pe.sendPacket(packet)
				}
			} else {
				// No delay, send immediately
				pe.sendPacket(packet)
			}
		}
	}
}

// sendPacket sends a packet back to the network stack
func (pe *PacketEngine) sendPacket(packet *godivert.Packet) {
	// Note: The godivert API may vary. If handle.Send doesn't work,
	// try: packet.Send(pe.handle) instead
	_, err := pe.handle.Send(packet)
	if err != nil {
		log.Printf("Error sending packet: %v", err)
		return
	}

	// Get packet length for statistics
	// The packet structure may have Raw, Bytes, or Data field
	// We'll use a safe approach to get the length
	packetLen := uint64(1500) // Default estimate
	if packet.Raw != nil {
		packetLen = uint64(len(packet.Raw))
	}

	// Update statistics
	updateStats(1, packetLen)
}
