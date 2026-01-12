package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/deblasis/godivert"
)

// PacketEngine handles packet interception and delay
type PacketEngine struct {
	handle    *godivert.WinDivertHandle
	delay     time.Duration
	delayMutex sync.RWMutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
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
	// This works for both regular execution and service mode
	dllPath := filepath.Join(exeDir, "WinDivert.dll")
	
	// For LoadDLL, we need to provide both 64-bit and 32-bit paths
	// Note: The godivert library checks runtime.GOARCH == "amd64" to choose path64 vs path32
	// For ARM64, it will incorrectly choose path32, so we need to work around this
	var path64, path32 string
	
	switch runtime.GOARCH {
	case "amd64":
		// x64 (AMD64) architecture
		path64 = dllPath
		path32 = dllPath // Required parameter, but won't be used
	case "arm64":
		// ARM64 architecture - also 64-bit, but library doesn't handle it correctly
		// Pass the DLL path as path64 since ARM64 needs 64-bit DLL
		// Note: WinDivert may not have official ARM64 support - user needs ARM64 DLL
		path64 = dllPath
		path32 = dllPath
		log.Printf("Warning: Running on ARM64 - ensure WinDivert.dll is ARM64-compatible")
	default:
		// 32-bit architecture (386)
		path64 = dllPath // Required parameter, but won't be used
		path32 = dllPath
	}
	
	// Load the DLL
	// Note: For ARM64, the library will use path32 due to its logic, but we've set both to the same path
	// This is a workaround until the library properly supports ARM64
	if err := godivert.LoadDLL(path64, path32); err != nil {
		return fmt.Errorf("failed to load WinDivert DLL from %s (architecture: %s): %v", dllPath, runtime.GOARCH, err)
	}
	
	log.Printf("WinDivert DLL loaded successfully from: %s (architecture: %s)", dllPath, runtime.GOARCH)
	return nil
}

// NewPacketEngine creates a new packet engine instance
func NewPacketEngine(delay time.Duration) (*PacketEngine, error) {
	// Ensure DLL is loaded before creating handle
	if !godivert.IsDLLLoaded() {
		if err := initWinDivertDLL(); err != nil {
			return nil, fmt.Errorf("WinDivert DLL not initialized: %v", err)
		}
	}
	
	// WinDivert filter: capture all outbound packets
	// "outbound" captures all outgoing packets
	filter := "outbound"
	
	handle, err := godivert.NewWinDivertHandle(filter)
	if err != nil {
		return nil, err
	}

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
			return
		default:
			// Receive packet
			packet, err := pe.handle.Recv()
			if err != nil {
				// Check if it's a timeout or stop condition
				select {
				case <-pe.stopChan:
					return
				default:
					log.Printf("Error receiving packet: %v", err)
					time.Sleep(10 * time.Millisecond) // Brief pause on error
					continue
				}
			}

			// Get current delay
			delay := pe.GetDelay()
			
			if delay > 0 {
				// Queue packet for delayed sending
				dp := &delayedPacket{
					packet: packet,
					sendAt: time.Now().Add(delay),
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

