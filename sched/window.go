package sched

import (
	"fmt"
	"log"
	"time"
)

// ParseTime parses a time string in "HH:MM" format.
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

// WithinSchedule reports whether now falls in the configured window (same rules as Scheduler.IsWithinSchedule).
func WithinSchedule(enabled bool, days []int, startTimeStr, endTimeStr string, now time.Time) bool {
	if !enabled {
		return false
	}

	currentDay := int(now.Weekday())
	dayOK := false
	for _, day := range days {
		if day == currentDay {
			dayOK = true
			break
		}
	}
	if !dayOK {
		return false
	}

	startHour, startMin, err := ParseTime(startTimeStr)
	if err != nil {
		log.Printf("[SCHEDULE] Error parsing start time: %v", err)
		return false
	}
	endHour, endMin, err := ParseTime(endTimeStr)
	if err != nil {
		log.Printf("[SCHEDULE] Error parsing end time: %v", err)
		return false
	}

	startTime := time.Date(now.Year(), now.Month(), now.Day(), startHour, startMin, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMin, 0, 0, now.Location())

	if endTime.Before(startTime) || endTime.Equal(startTime) {
		endTime = endTime.Add(24 * time.Hour)
		morningEnd := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMin, 0, 0, now.Location())
		// Post-midnight segment only: before evening start and not past morning end (inclusive).
		if now.Before(startTime) && !now.After(morningEnd) {
			startTime = startTime.Add(-24 * time.Hour)
		}
	}

	return (now.After(startTime) || now.Equal(startTime)) && (now.Before(endTime) || now.Equal(endTime))
}
