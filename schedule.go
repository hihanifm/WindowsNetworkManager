package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ScheduleConfig represents the schedule configuration
type ScheduleConfig struct {
	Enabled    bool     `json:"enabled"`
	Days       []int    `json:"days"`        // 0=Sunday, 1=Monday, ..., 6=Saturday
	StartTime  string   `json:"start_time"`  // Format: "HH:MM" (24-hour)
	EndTime    string   `json:"end_time"`    // Format: "HH:MM" (24-hour)
	MaxDelayMs int64    `json:"max_delay_ms"` // Max delay for sessions
}

// Session represents a scheduled disruption session
type Session struct {
	StartTime time.Time
	EndTime   time.Time
}

// Scheduler manages scheduled disruptions
type Scheduler struct {
	config       ScheduleConfig
	configMutex  sync.RWMutex
	stopChan     chan struct{}
	running      bool
	runningMutex sync.RWMutex
	activeSession *Session
	sessionMutex sync.RWMutex
}

// getScheduleFilePath returns the path to the schedule config file
func getScheduleFilePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "schedule.json"), nil
}

// NewScheduler creates a new scheduler instance
func NewScheduler() *Scheduler {
	return &Scheduler{
		stopChan: make(chan struct{}),
		config: ScheduleConfig{
			Enabled:    false,
			Days:       []int{1, 2, 3, 4, 5}, // Monday-Friday
			StartTime:  "09:00",
			EndTime:    "18:00",
			MaxDelayMs: 1000,
		},
	}
}

// LoadConfig loads the schedule configuration from schedule.json
func (s *Scheduler) LoadConfig() error {
	configPath, err := getScheduleFilePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return nil
		}
		return fmt.Errorf("failed to read schedule config: %v", err)
	}

	var config ScheduleConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal schedule config: %v", err)
	}

	s.configMutex.Lock()
	s.config = config
	s.configMutex.Unlock()

	log.Printf("[SCHEDULE] Loaded schedule config: enabled=%v, days=%v, time=%s-%s, max_delay=%dms",
		config.Enabled, config.Days, config.StartTime, config.EndTime, config.MaxDelayMs)
	return nil
}

// SaveConfig saves the schedule configuration to schedule.json
func (s *Scheduler) SaveConfig() error {
	s.configMutex.RLock()
	config := s.config
	s.configMutex.RUnlock()

	configPath, err := getScheduleFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schedule config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schedule config: %v", err)
	}

	log.Printf("[SCHEDULE] Saved schedule config: enabled=%v, days=%v, time=%s-%s, max_delay=%dms",
		config.Enabled, config.Days, config.StartTime, config.EndTime, config.MaxDelayMs)
	return nil
}

// GetConfig returns a copy of the current schedule configuration
func (s *Scheduler) GetConfig() ScheduleConfig {
	s.configMutex.RLock()
	defer s.configMutex.RUnlock()
	return s.config
}

// SetConfig updates the schedule configuration
func (s *Scheduler) SetConfig(config ScheduleConfig) error {
	s.configMutex.Lock()
	s.config = config
	s.configMutex.Unlock()

	if err := s.SaveConfig(); err != nil {
		return err
	}
	return nil
}

// ParseTime parses a time string in "HH:MM" format
func ParseTime(timeStr string) (hour, minute int, err error) {
	var h, m int
	_, err = fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time: %s (hour must be 0-23, minute must be 0-59)", timeStr)
	}
	return h, m, nil
}

// IsWithinSchedule checks if the current time is within the schedule
func (s *Scheduler) IsWithinSchedule(now time.Time) bool {
	s.configMutex.RLock()
	config := s.config
	s.configMutex.RUnlock()

	if !config.Enabled {
		return false
	}

	// Check if current day is in schedule
	// Go's time.Weekday: 0=Sunday, 1=Monday, ..., 6=Saturday (matches our config)
	currentDay := int(now.Weekday())
	
	dayInSchedule := false
	for _, day := range config.Days {
		if day == currentDay {
			dayInSchedule = true
			break
		}
	}
	if !dayInSchedule {
		return false
	}

	// Check if current time is within time range
	startHour, startMin, err := ParseTime(config.StartTime)
	if err != nil {
		log.Printf("[SCHEDULE] Error parsing start time: %v", err)
		return false
	}
	endHour, endMin, err := ParseTime(config.EndTime)
	if err != nil {
		log.Printf("[SCHEDULE] Error parsing end time: %v", err)
		return false
	}

	startTime := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMin, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMin, 0, 0, now.Location())

	// Handle case where end time is next day (e.g., 22:00-02:00)
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		endTime = endTime.Add(24 * time.Hour)
		// If now is before startTime, we might be on the previous day's end period
		if now.Before(startTime) {
			startTime = startTime.Add(-24 * time.Hour)
		}
	}

	// Check if now is within the time range
	return (now.After(startTime) || now.Equal(startTime)) && (now.Before(endTime) || now.Equal(endTime))
}

// planSessionsForHour plans 3-6 random disruption sessions for the given hour
func (s *Scheduler) planSessionsForHour(hourStart time.Time) []Session {
	// Random number of sessions between 3 and 6
	numSessions := rand.Intn(4) + 3 // 3-6 sessions

	sessions := make([]Session, 0, numSessions)
	hourEnd := hourStart.Add(1 * time.Hour)

	// Distribute sessions randomly throughout the hour
	for i := 0; i < numSessions; i++ {
		// Random start time within the hour (leaving room for session duration)
		maxStartOffset := 55 * time.Minute // Leave 5 minutes at the end
		startOffset := time.Duration(rand.Int63n(int64(maxStartOffset)))
		sessionStart := hourStart.Add(startOffset)

		// Random session duration between 2-5 minutes
		sessionDuration := time.Duration(rand.Intn(4)+2) * time.Minute // 2-5 minutes
		sessionEnd := sessionStart.Add(sessionDuration)

		// Make sure session ends within the hour (or schedule end time)
		if sessionEnd.After(hourEnd) {
			sessionEnd = hourEnd
			if sessionEnd.Before(sessionStart.Add(2 * time.Minute)) {
				// Session would be too short, skip it
				continue
			}
		}

		sessions = append(sessions, Session{
			StartTime: sessionStart,
			EndTime:   sessionEnd,
		})
	}

	// Sort sessions by start time
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].StartTime.After(sessions[j].StartTime) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	return sessions
}

// Start starts the scheduler goroutine
func (s *Scheduler) Start() {
	s.runningMutex.Lock()
	if s.running {
		s.runningMutex.Unlock()
		return
	}
	s.running = true
	s.runningMutex.Unlock()

	go s.run()
	log.Printf("[SCHEDULE] Scheduler started")
}

// Stop stops the scheduler goroutine
func (s *Scheduler) Stop() {
	s.runningMutex.Lock()
	if !s.running {
		s.runningMutex.Unlock()
		return
	}
	s.running = false
	s.runningMutex.Unlock()

	close(s.stopChan)
	s.stopChan = make(chan struct{})

	// Clear active session
	s.sessionMutex.Lock()
	s.activeSession = nil
	s.sessionMutex.Unlock()

	// Reset delay
	delayMutex.Lock()
	currentDelay = 0
	delayMutex.Unlock()
	if packetEngine != nil {
		packetEngine.SetDelay(0)
	}

	log.Printf("[SCHEDULE] Scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.runningMutex.RLock()
	defer s.runningMutex.RUnlock()
	return s.running
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	var plannedSessions []Session
	var currentHour time.Time
	var sessionIndex int

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			now := time.Now()

			// Check if we're within schedule
			if !s.IsWithinSchedule(now) {
				// Outside schedule - clear active session and reset delay
				s.sessionMutex.Lock()
				hadActiveSession := s.activeSession != nil
				s.activeSession = nil
				s.sessionMutex.Unlock()

				if hadActiveSession {
					delayMutex.Lock()
					currentDelay = 0
					delayMutex.Unlock()
					if packetEngine != nil {
						packetEngine.SetDelay(0)
					}
					log.Printf("[SCHEDULE] Outside schedule, stopped active session")
				}
				plannedSessions = nil
				continue
			}

			// Within schedule - plan sessions for current hour if needed
			currentHourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
			if len(plannedSessions) == 0 || !currentHourStart.Equal(currentHour) {
				plannedSessions = s.planSessionsForHour(currentHourStart)
				currentHour = currentHourStart
				sessionIndex = 0
				log.Printf("[SCHEDULE] Planned %d sessions for hour starting at %s", len(plannedSessions), currentHourStart.Format("15:04"))
			}

			// Check if we should start a new session
			for sessionIndex < len(plannedSessions) {
				session := plannedSessions[sessionIndex]
				if now.After(session.StartTime) || now.Equal(session.StartTime) {
					if now.Before(session.EndTime) || now.Equal(session.EndTime) {
						// Start or continue this session
						s.sessionMutex.Lock()
						s.activeSession = &session
						s.sessionMutex.Unlock()

						// Set random delay between 1ms and max_delay_ms
						s.configMutex.RLock()
						maxDelayMs := s.config.MaxDelayMs
						s.configMutex.RUnlock()

						if maxDelayMs > 0 {
							randomDelayMs := rand.Int63n(maxDelayMs) + 1 // 1 to maxDelayMs
							delay := time.Duration(randomDelayMs) * time.Millisecond

							delayMutex.Lock()
							currentDelay = delay
							delayMutex.Unlock()
							if packetEngine != nil {
								packetEngine.SetDelay(delay)
							}
							log.Printf("[SCHEDULE] Session active: delay=%dms (until %s)", randomDelayMs, session.EndTime.Format("15:04:05"))
						}
						break
					} else {
						// Session has ended, move to next
						sessionIndex++
					}
				} else {
					// Session hasn't started yet
					break
				}
			}

			// Check if current session has ended
			s.sessionMutex.RLock()
			activeSession := s.activeSession
			s.sessionMutex.RUnlock()

			if activeSession != nil && now.After(activeSession.EndTime) {
				// Session ended
				s.sessionMutex.Lock()
				s.activeSession = nil
				s.sessionMutex.Unlock()

				delayMutex.Lock()
				currentDelay = 0
				delayMutex.Unlock()
				if packetEngine != nil {
					packetEngine.SetDelay(0)
				}
				log.Printf("[SCHEDULE] Session ended")
				sessionIndex++
			}
		}
	}
}
