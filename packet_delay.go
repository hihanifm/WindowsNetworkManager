package main

import (
	"log"
	"sync"
	"time"

	"github.com/deblasis/godivert"
)

// PacketEngine handles packet interception and delay
type PacketEngine struct {
	handle     *godivert.WinDivertHandle
	delay      time.Duration
	delayMutex sync.RWMutex
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewPacketEngine creates a new packet engine instance
func NewPacketEngine(delay time.Duration) (*PacketEngine, error) {
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
