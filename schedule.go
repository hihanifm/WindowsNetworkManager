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

	"WindowsNetworkManager/sched"
)

// applyScheduledImpairmentToEngine sets delay/random/drop from schedule override or global profile.
func applyScheduledImpairmentToEngine(s *Scheduler) {
	if packetEngine == nil {
		return
	}
	s.configMutex.RLock()
	cfg := s.config
	s.configMutex.RUnlock()

	var d time.Duration
	var r bool
	var loss int
	if cfg.ImpairmentOverride.Enabled {
		d = time.Duration(cfg.ImpairmentOverride.DelayMs) * time.Millisecond
		r = cfg.ImpairmentOverride.RandomDelay
		loss = cfg.ImpairmentOverride.PacketLossPercent
		if loss < 0 {
			loss = 0
		}
		if loss > 100 {
			loss = 100
		}
	} else {
		delayMutex.RLock()
		d = configuredDelay
		delayMutex.RUnlock()
		randomDelayMutex.RLock()
		r = useRandomDelay
		randomDelayMutex.RUnlock()
		dropPercentMutex.RLock()
		loss = configuredDropPercent
		dropPercentMutex.RUnlock()
	}
	packetEngine.SetDelay(d)
	packetEngine.SetRandomDelay(r)
	packetEngine.SetDropPercent(loss)
}

func clearScheduledImpairmentOnEngine() {
	if packetEngine == nil {
		return
	}
	packetEngine.SetDelay(0)
	packetEngine.SetDropPercent(0)
}

// ScheduleImpairmentOverride applies a custom impairment profile during scheduled sessions when Enabled is true.
type ScheduleImpairmentOverride struct {
	Enabled           bool  `json:"enabled"`
	DelayMs           int64 `json:"delay_ms"`
	RandomDelay       bool  `json:"random_delay"`
	PacketLossPercent int   `json:"packet_loss_percent"`
}

// ScheduleConfig represents the schedule configuration
type ScheduleConfig struct {
	Enabled              bool                       `json:"enabled"`
	Days                 []int                      `json:"days"` // 0=Sunday, 1=Monday, ..., 6=Saturday
	StartTime            string                     `json:"start_time"`
	EndTime              string                     `json:"end_time"`
	MaxSessionsPerHour   int                        `json:"max_sessions_per_hour"`
	ImpairmentOverride   ScheduleImpairmentOverride `json:"impairment_override"`
}

// Session represents a scheduled disruption session
type Session struct {
	StartTime time.Time
	EndTime   time.Time
}

// Scheduler manages scheduled disruptions
type Scheduler struct {
	config         ScheduleConfig
	configMutex    sync.RWMutex
	stopChan       chan struct{}
	running        bool
	runningMutex   sync.RWMutex
	activeSession  *Session
	sessionMutex   sync.RWMutex
	plannedSessions []Session
	plannedSessionsMutex sync.RWMutex
	sessionsCompleted int
	sessionsCompletedMutex sync.RWMutex
	currentHour    time.Time
	currentHourMutex sync.RWMutex
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
			Enabled:            false,
			Days:               []int{1, 2, 3, 4, 5}, // Monday-Friday
			StartTime:          "09:00",
			EndTime:            "18:00",
			MaxSessionsPerHour: 6,
			ImpairmentOverride: ScheduleImpairmentOverride{
				Enabled:           false,
				DelayMs:           1000,
				RandomDelay:       true,
				PacketLossPercent: 0,
			},
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

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal schedule config: %v", err)
	}

	var config ScheduleConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal schedule config: %v", err)
	}

	var legacyMaxDelay int64
	if b, ok := raw["max_delay_ms"]; ok {
		_ = json.Unmarshal(b, &legacyMaxDelay)
	}
	if !config.ImpairmentOverride.Enabled && legacyMaxDelay > 0 {
		config.ImpairmentOverride = ScheduleImpairmentOverride{
			Enabled:           true,
			DelayMs:           legacyMaxDelay,
			RandomDelay:       true,
			PacketLossPercent: 0,
		}
	}

	s.configMutex.Lock()
	s.config = config
	s.configMutex.Unlock()

	log.Printf("[SCHEDULE] Loaded schedule config: enabled=%v, days=%v, time=%s-%s, max_sessions_per_hour=%d, impairment_override_enabled=%v",
		config.Enabled, config.Days, config.StartTime, config.EndTime, config.MaxSessionsPerHour, config.ImpairmentOverride.Enabled)
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

	log.Printf("[SCHEDULE] Saved schedule config: enabled=%v, days=%v, time=%s-%s, max_sessions_per_hour=%d, impairment_override_enabled=%v",
		config.Enabled, config.Days, config.StartTime, config.EndTime, config.MaxSessionsPerHour, config.ImpairmentOverride.Enabled)
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

// ParseTime parses a time string in "HH:MM" format (delegates to sched package).
func ParseTime(timeStr string) (hour, minute int, err error) {
	return sched.ParseTime(timeStr)
}

// IsWithinSchedule checks if the current time is within the schedule
func (s *Scheduler) IsWithinSchedule(now time.Time) bool {
	s.configMutex.RLock()
	config := s.config
	s.configMutex.RUnlock()
	return sched.WithinSchedule(config.Enabled, config.Days, config.StartTime, config.EndTime, now)
}

// planSessionsForHour plans random disruption sessions for the given hour
func (s *Scheduler) planSessionsForHour(hourStart time.Time) []Session {
	// Get max sessions per hour from config
	s.configMutex.RLock()
	maxSessionsPerHour := s.config.MaxSessionsPerHour
	s.configMutex.RUnlock()

	// Default to 6 if not set (for backward compatibility)
	if maxSessionsPerHour <= 0 {
		maxSessionsPerHour = 6
	}

	// Calculate min sessions as 50% of max (integer division)
	minSessions := maxSessionsPerHour / 2
	if minSessions < 1 {
		minSessions = 1
	}

	// Random number of sessions between min (50% of max) and max
	// Example: if max=10, min=5, then range is rand.Intn(6)+5 = 5-10
	numSessions := rand.Intn(maxSessionsPerHour-minSessions+1) + minSessions

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

	// Clear planned sessions and reset counters
	s.plannedSessionsMutex.Lock()
	s.plannedSessions = nil
	s.plannedSessionsMutex.Unlock()

	s.sessionsCompletedMutex.Lock()
	s.sessionsCompleted = 0
	s.sessionsCompletedMutex.Unlock()

	s.currentHourMutex.Lock()
	s.currentHour = time.Time{}
	s.currentHourMutex.Unlock()

	clearScheduledImpairmentOnEngine()

	log.Printf("[SCHEDULE] Scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.runningMutex.RLock()
	defer s.runningMutex.RUnlock()
	return s.running
}

// ScheduleStatus represents the current status of the scheduler
type ScheduleStatus struct {
	NextSessionTime    *time.Time `json:"next_session_time,omitempty"`
	SessionsCompleted  int        `json:"sessions_completed"`
	IsWithinSchedule   bool       `json:"is_within_schedule"`
	HasActiveSession   bool       `json:"has_active_session"`
}

// GetScheduleStatus returns the current status including next session time and completed count
func (s *Scheduler) GetScheduleStatus() ScheduleStatus {
	now := time.Now()
	
	s.plannedSessionsMutex.RLock()
	plannedSessions := s.plannedSessions
	s.plannedSessionsMutex.RUnlock()
	
	s.sessionsCompletedMutex.RLock()
	sessionsCompleted := s.sessionsCompleted
	s.sessionsCompletedMutex.RUnlock()
	
	s.sessionMutex.RLock()
	hasActiveSession := s.activeSession != nil
	s.sessionMutex.RUnlock()
	
	isWithinSchedule := s.IsWithinSchedule(now)
	
	// Find next session from planned sessions
	var nextSessionTime *time.Time
	if isWithinSchedule && len(plannedSessions) > 0 {
		for _, session := range plannedSessions {
			if session.StartTime.After(now) {
				nextSessionTime = &session.StartTime
				break
			}
		}
	}
	
	return ScheduleStatus{
		NextSessionTime:   nextSessionTime,
		SessionsCompleted: sessionsCompleted,
		IsWithinSchedule:  isWithinSchedule,
		HasActiveSession:  hasActiveSession,
	}
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
					clearScheduledImpairmentOnEngine()
					log.Printf("[SCHEDULE] Outside schedule, stopped active session")
				}
				plannedSessions = nil
				
				// Clear planned sessions when outside schedule
				s.plannedSessionsMutex.Lock()
				s.plannedSessions = nil
				s.plannedSessionsMutex.Unlock()
				
				continue
			}

			// Within schedule - plan sessions for current hour if needed
			currentHourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
			if len(plannedSessions) == 0 || !currentHourStart.Equal(currentHour) {
				plannedSessions = s.planSessionsForHour(currentHourStart)
				currentHour = currentHourStart
				sessionIndex = 0
				
				// Store planned sessions and reset completed count for new hour
				s.plannedSessionsMutex.Lock()
				s.plannedSessions = plannedSessions
				s.plannedSessionsMutex.Unlock()
				
				s.currentHourMutex.Lock()
				s.currentHour = currentHourStart
				s.currentHourMutex.Unlock()
				
				s.sessionsCompletedMutex.Lock()
				s.sessionsCompleted = 0
				s.sessionsCompletedMutex.Unlock()
				
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

						applyScheduledImpairmentToEngine(s)
						s.configMutex.RLock()
						ov := s.config.ImpairmentOverride
						s.configMutex.RUnlock()
						src := "global"
						if ov.Enabled {
							src = "override"
						}
						log.Printf("[SCHEDULE] Session active (%s impairment until %s)", src, session.EndTime.Format("15:04:05"))
						break
					} else {
						// Session has ended, move to next
						// Don't increment here - the active session check below handles completed sessions
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

				clearScheduledImpairmentOnEngine()

				// Increment completed count
				s.sessionsCompletedMutex.Lock()
				s.sessionsCompleted++
				completedCount := s.sessionsCompleted
				s.sessionsCompletedMutex.Unlock()
				
				log.Printf("[SCHEDULE] Session ended (completed: %d)", completedCount)
				sessionIndex++
			}
		}
	}
}
