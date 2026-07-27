package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParseSleepInsightsRangeDays(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   int
		wantOK bool
	}{
		{name: "empty defaults to 30", raw: "", want: 30, wantOK: true},
		{name: "7 is allowed", raw: "7", want: 7, wantOK: true},
		{name: "90 is allowed", raw: "90", want: 90, wantOK: true},
		{name: "14 is rejected", raw: "14", wantOK: false},
		{name: "non-numeric is rejected", raw: "abc", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSleepInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSleepInsightsWindow(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)

	rangeStart, todayStart := sleepInsightsWindow(7, loc, now)

	if !todayStart.Equal(time.Date(2026, 7, 27, 0, 0, 0, 0, loc)) {
		t.Fatalf("todayStart = %s, want 2026-07-27 00:00", todayStart)
	}
	if !rangeStart.Equal(time.Date(2026, 7, 21, 0, 0, 0, 0, loc)) {
		t.Fatalf("rangeStart = %s, want 2026-07-21 00:00 (6 days before today)", rangeStart)
	}
}

func sleepEvent(babyID uuid.UUID, occurredAt time.Time, sleepType string, durationMinutes *int) store.Event {
	attrs := map[string]any{"type": sleepType}
	if durationMinutes != nil {
		attrs["duration_minutes"] = float64(*durationMinutes)
	}
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeSleep,
		OccurredAt: occurredAt,
		Attributes: attrs,
	}
}

func intPtr(v int) *int { return &v }

func TestBuildSleepInsightsBucketsEventsByLocalDay(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, todayStart := sleepInsightsWindow(3, loc, now)

	events := []store.Event{
		// yesterday: one completed night sleep, 8 hours
		sleepEvent(babyID, time.Date(2026, 7, 26, 20, 0, 0, 0, loc), "night", intPtr(480)),
		// today: one completed nap, then an ongoing night sleep
		sleepEvent(babyID, time.Date(2026, 7, 27, 9, 0, 0, 0, loc), "nap", intPtr(60)),
		sleepEvent(babyID, time.Date(2026, 7, 27, 13, 45, 0, 0, loc), "night", nil),
	}

	resp := buildSleepInsights(events, 3, rangeStart, todayStart, now)

	if len(resp.Days) != 3 {
		t.Fatalf("len(Days) = %d, want 3", len(resp.Days))
	}

	dayBeforeYesterday, yesterday, today := resp.Days[0], resp.Days[1], resp.Days[2]

	if dayBeforeYesterday.HasData {
		t.Fatalf("day before yesterday should have no data: %#v", dayBeforeYesterday)
	}

	if !yesterday.HasData || yesterday.TotalMinutes != 480 || yesterday.CompletedCount != 1 || yesterday.NightMinutes != 480 || yesterday.NapMinutes != 0 {
		t.Fatalf("yesterday = %#v, want 480 total minutes, 1 completed, all night", yesterday)
	}
	if yesterday.TotalLabel != "8 hr" {
		t.Fatalf("yesterday.TotalLabel = %q, want %q", yesterday.TotalLabel, "8 hr")
	}

	if !today.HasData || today.CompletedCount != 1 || today.NapMinutes != 60 {
		t.Fatalf("today = %#v, want 1 completed period and 60 nap minutes", today)
	}
	wantTodayOngoingMinutes := 60 // 13:45 to 14:45
	if today.NightMinutes != wantTodayOngoingMinutes {
		t.Fatalf("today.NightMinutes = %d, want %d (ongoing elapsed)", today.NightMinutes, wantTodayOngoingMinutes)
	}
	if !today.IsToday {
		t.Fatalf("today.IsToday = false, want true")
	}
	if today.TotalLabel == "" || today.TotalLabel[len(today.TotalLabel)-len(" so far"):] != " so far" {
		t.Fatalf("today.TotalLabel = %q, want suffix %q", today.TotalLabel, " so far")
	}

	if len(today.Periods) != 2 {
		t.Fatalf("len(today.Periods) = %d, want 2", len(today.Periods))
	}
	ongoingPeriod := today.Periods[1]
	if !ongoingPeriod.Ongoing || ongoingPeriod.DurationMinutes != 60 {
		t.Fatalf("ongoing period = %#v, want ongoing with 60 elapsed minutes", ongoingPeriod)
	}
	if ongoingPeriod.TimeRangeLabel != "1:45 PM – ongoing" {
		t.Fatalf("ongoing period TimeRangeLabel = %q, want %q", ongoingPeriod.TimeRangeLabel, "1:45 PM – ongoing")
	}

	if !resp.Aggregate.HasAnyData {
		t.Fatalf("aggregate.HasAnyData = false, want true")
	}
	if resp.Aggregate.NapPercent == nil || resp.Aggregate.NightPercent == nil {
		t.Fatalf("aggregate nap/night percent missing: %#v", resp.Aggregate)
	}
	if *resp.Aggregate.NapPercent+*resp.Aggregate.NightPercent != 100 {
		t.Fatalf("nap+night percent = %d, want 100", *resp.Aggregate.NapPercent+*resp.Aggregate.NightPercent)
	}
	if len(resp.Observations) == 0 {
		t.Fatalf("expected non-empty observations when data is present")
	}
}

func TestBuildSleepInsightsNoDataYieldsEmptyAggregate(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, todayStart := sleepInsightsWindow(7, loc, now)

	resp := buildSleepInsights(nil, 7, rangeStart, todayStart, now)

	if resp.Aggregate.HasAnyData {
		t.Fatalf("aggregate.HasAnyData = true, want false for no events")
	}
	if len(resp.Observations) != 0 {
		t.Fatalf("observations = %#v, want empty when there is no data", resp.Observations)
	}
	for _, day := range resp.Days {
		if day.HasData {
			t.Fatalf("day %s has data, want none", day.LocalDate)
		}
	}
}

func TestBuildSleepInsightsWakeWindowFallback(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, todayStart := sleepInsightsWindow(7, loc, now)

	// Only a single completed sleep in range: not enough gaps to average a
	// wake window from.
	events := []store.Event{
		sleepEvent(babyID, time.Date(2026, 7, 26, 20, 0, 0, 0, loc), "night", intPtr(480)),
	}

	resp := buildSleepInsights(events, 7, rangeStart, todayStart, now)

	if resp.Aggregate.HasWakeWindow {
		t.Fatalf("HasWakeWindow = true, want false with only one sleep period")
	}
	if resp.Aggregate.AverageWakeWindowLabel != "Not yet available" {
		t.Fatalf("AverageWakeWindowLabel = %q, want %q", resp.Aggregate.AverageWakeWindowLabel, "Not yet available")
	}

	found := false
	for _, o := range resp.Observations {
		if o == "Not enough recorded sleep yet to calculate an average wake window." {
			found = true
		}
	}
	if !found {
		t.Fatalf("observations = %#v, want the wake-window fallback sentence", resp.Observations)
	}
}

func TestSleepInsightShowLabel(t *testing.T) {
	tests := []struct {
		rangeDays int
		index     int
		want      bool
	}{
		{rangeDays: 7, index: 3, want: true},
		{rangeDays: 30, index: 0, want: true},
		{rangeDays: 30, index: 5, want: true},
		{rangeDays: 30, index: 6, want: false},
		{rangeDays: 30, index: 29, want: true},
		{rangeDays: 90, index: 15, want: true},
		{rangeDays: 90, index: 16, want: false},
		{rangeDays: 90, index: 89, want: true},
	}
	for _, tt := range tests {
		if got := sleepInsightShowLabel(tt.index, tt.rangeDays); got != tt.want {
			t.Fatalf("sleepInsightShowLabel(%d, %d) = %v, want %v", tt.index, tt.rangeDays, got, tt.want)
		}
	}
}

func TestSleepClockLabel(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{minutes: 0, want: "12:00 AM"},
		{minutes: 60, want: "1:00 AM"},
		{minutes: 12 * 60, want: "12:00 PM"},
		{minutes: 13*60 + 45, want: "1:45 PM"},
		{minutes: 23*60 + 59, want: "11:59 PM"},
	}
	for _, tt := range tests {
		if got := sleepClockLabel(tt.minutes); got != tt.want {
			t.Fatalf("sleepClockLabel(%d) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}
