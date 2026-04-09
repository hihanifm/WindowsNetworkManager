package sched

import (
	"testing"
	"time"
)

var testLoc = time.FixedZone("TestLoc", 0)

func mustParseTime(t *testing.T, year int, month time.Month, day, hour, min, sec int) time.Time {
	t.Helper()
	return time.Date(year, month, day, hour, min, sec, 0, testLoc)
}

func allWeekdays() []int {
	return []int{0, 1, 2, 3, 4, 5, 6}
}

func TestWithinSchedule_overnight_daytimeMustBeInactive(t *testing.T) {
	daytimeCases := []struct {
		name string
		when time.Time
	}{
		{"Tuesday noon", mustParseTime(t, 2026, time.April, 7, 12, 0, 0)},
		{"Tuesday 14:00", mustParseTime(t, 2026, time.April, 7, 14, 0, 0)},
		{"Tuesday 15:30", mustParseTime(t, 2026, time.April, 7, 15, 30, 0)},
		{"Tuesday 20:59", mustParseTime(t, 2026, time.April, 7, 20, 59, 0)},
		{"Wednesday 09:00", mustParseTime(t, 2026, time.April, 8, 9, 0, 0)},
		{"Saturday 18:00", mustParseTime(t, 2026, time.April, 11, 18, 0, 0)},
	}
	for _, tc := range daytimeCases {
		t.Run(tc.name, func(t *testing.T) {
			if WithinSchedule(true, allWeekdays(), "21:00", "06:00", tc.when) {
				t.Fatalf("expected outside overnight window, got inside (bug): %v", tc.when)
			}
		})
	}
}

func TestWithinSchedule_overnight_eveningAndNightActive(t *testing.T) {
	activeCases := []struct {
		name string
		when time.Time
	}{
		{"Tuesday 21:00 start", mustParseTime(t, 2026, time.April, 7, 21, 0, 0)},
		{"Tuesday 23:00", mustParseTime(t, 2026, time.April, 7, 23, 0, 0)},
		{"Wednesday 00:30 after midnight", mustParseTime(t, 2026, time.April, 8, 0, 30, 0)},
		{"Wednesday 03:00 tail", mustParseTime(t, 2026, time.April, 8, 3, 0, 0)},
		{"Wednesday 06:00 inclusive end", mustParseTime(t, 2026, time.April, 8, 6, 0, 0)},
	}
	for _, tc := range activeCases {
		t.Run(tc.name, func(t *testing.T) {
			if !WithinSchedule(true, allWeekdays(), "21:00", "06:00", tc.when) {
				t.Fatalf("expected inside overnight window: %v", tc.when)
			}
		})
	}
}

func TestWithinSchedule_overnight_justAfterEndInactive(t *testing.T) {
	when := mustParseTime(t, 2026, time.April, 8, 6, 0, 1)
	if WithinSchedule(true, allWeekdays(), "21:00", "06:00", when) {
		t.Fatalf("expected outside schedule immediately after end: %v", when)
	}
}

func TestWithinSchedule_sameDay_window(t *testing.T) {
	if !WithinSchedule(true, allWeekdays(), "09:00", "18:00", mustParseTime(t, 2026, time.April, 7, 9, 0, 0)) {
		t.Fatal("09:00 should be active (inclusive start)")
	}
	if !WithinSchedule(true, allWeekdays(), "09:00", "18:00", mustParseTime(t, 2026, time.April, 7, 12, 0, 0)) {
		t.Fatal("midday should be active")
	}
	if !WithinSchedule(true, allWeekdays(), "09:00", "18:00", mustParseTime(t, 2026, time.April, 7, 18, 0, 0)) {
		t.Fatal("18:00 should be active (inclusive end)")
	}
	if WithinSchedule(true, allWeekdays(), "09:00", "18:00", mustParseTime(t, 2026, time.April, 7, 8, 59, 0)) {
		t.Fatal("08:59 should be inactive")
	}
	if WithinSchedule(true, allWeekdays(), "09:00", "18:00", mustParseTime(t, 2026, time.April, 7, 18, 0, 1)) {
		t.Fatal("18:00:01 should be inactive")
	}
}

func TestWithinSchedule_disabled(t *testing.T) {
	if WithinSchedule(false, allWeekdays(), "21:00", "06:00", mustParseTime(t, 2026, time.April, 8, 2, 0, 0)) {
		t.Fatal("disabled schedule should never be active")
	}
}

func TestWithinSchedule_dayNotInSchedule(t *testing.T) {
	tue22 := mustParseTime(t, 2026, time.April, 7, 22, 0, 0)
	if WithinSchedule(true, []int{1}, "21:00", "06:00", tue22) {
		t.Fatal("Tuesday should not match Monday-only days")
	}
}

func TestWithinSchedule_overnight_crossCalendarDay_edge(t *testing.T) {
	mon0100 := mustParseTime(t, 2026, time.April, 6, 1, 0, 0)
	if !WithinSchedule(true, []int{1}, "22:00", "02:00", mon0100) {
		t.Fatal("Monday 01:00 should be inside Sun 22:00–Mon 02:00 tail when Monday is scheduled")
	}
}
