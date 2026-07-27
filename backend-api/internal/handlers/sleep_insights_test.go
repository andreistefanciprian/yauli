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

func TestSleepInsightsWindowExcludesToday(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)

	rangeStart, rangeEnd := sleepInsightsWindow(7, loc, now)

	if !rangeEnd.Equal(time.Date(2026, 7, 27, 0, 0, 0, 0, loc)) {
		t.Fatalf("rangeEnd = %s, want today's local midnight", rangeEnd)
	}
	if !rangeStart.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, loc)) {
		t.Fatalf("rangeStart = %s, want seven completed days before today", rangeStart)
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

func TestBuildSleepInsightsSplitsCompletedSleepsAcrossLocalDays(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, rangeEnd := sleepInsightsWindow(3, loc, now)

	events := []store.Event{
		// Starts before the selected range and contributes four hours to its
		// first day.
		sleepEvent(babyID, time.Date(2026, 7, 23, 20, 0, 0, 0, loc), "night", intPtr(480)),
		sleepEvent(babyID, time.Date(2026, 7, 24, 9, 0, 0, 0, loc), "nap", intPtr(60)),
		// Spans two selected days and contributes two hours to each.
		sleepEvent(babyID, time.Date(2026, 7, 25, 22, 0, 0, 0, loc), "night", intPtr(240)),
	}

	resp := buildSleepInsights(events, 3, rangeStart, rangeEnd)

	if len(resp.Days) != 3 {
		t.Fatalf("len(Days) = %d, want 3", len(resp.Days))
	}
	if resp.RangeLabel != "Jul 24 – Jul 26" {
		t.Fatalf("RangeLabel = %q, want completed-day range", resp.RangeLabel)
	}

	first, second, third := resp.Days[0], resp.Days[1], resp.Days[2]
	if first.TotalMinutes != 300 || first.NightMinutes != 240 || first.NapMinutes != 60 {
		t.Fatalf("first day = %#v, want 240 night + 60 nap minutes", first)
	}
	if second.TotalMinutes != 120 || second.NightMinutes != 120 {
		t.Fatalf("second day = %#v, want first 120 minutes of overnight sleep", second)
	}
	if third.TotalMinutes != 120 || third.NightMinutes != 120 {
		t.Fatalf("third day = %#v, want final 120 minutes of overnight sleep", third)
	}
	if len(second.Periods) != 1 || second.Periods[0].TimeRangeLabel != "10:00 PM – 12:00 AM" {
		t.Fatalf("second-day periods = %#v, want clipped 10 PM-midnight segment", second.Periods)
	}
	if len(third.Periods) != 1 || third.Periods[0].TimeRangeLabel != "12:00 AM – 2:00 AM" {
		t.Fatalf("third-day periods = %#v, want clipped midnight-2 AM segment", third.Periods)
	}
	if resp.Aggregate.AverageTotalLabel != "3 hr" {
		t.Fatalf("AverageTotalLabel = %q, want 540 minutes / 3 days", resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.AverageCompletedLabel != "1.3" {
		t.Fatalf("AverageCompletedLabel = %q, want four daily period overlaps / 3 days", resp.Aggregate.AverageCompletedLabel)
	}
}

func TestBuildSleepInsightsExcludesTodayAndOngoingDurations(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, rangeEnd := sleepInsightsWindow(3, loc, now)

	events := []store.Event{
		sleepEvent(babyID, time.Date(2026, 7, 24, 9, 0, 0, 0, loc), "nap", intPtr(60)),
		sleepEvent(babyID, time.Date(2026, 7, 25, 13, 45, 0, 0, loc), "night", nil),
		// A current-day event must never enter the completed-day response.
		sleepEvent(babyID, time.Date(2026, 7, 27, 9, 0, 0, 0, loc), "nap", intPtr(120)),
	}

	resp := buildSleepInsights(events, 3, rangeStart, rangeEnd)

	if len(resp.Days) != 3 || resp.Days[2].LocalDate != "2026-07-26" {
		t.Fatalf("Days = %#v, want three completed days ending July 26", resp.Days)
	}
	ongoingDay := resp.Days[1]
	if !ongoingDay.HasData || ongoingDay.TotalMinutes != 0 || ongoingDay.CompletedCount != 0 || ongoingDay.NightMinutes != 0 {
		t.Fatalf("ongoing day = %#v, want visible event excluded from duration metrics", ongoingDay)
	}
	if len(ongoingDay.Periods) != 1 || !ongoingDay.Periods[0].Ongoing || ongoingDay.Periods[0].DurationLabel != "Ongoing" {
		t.Fatalf("ongoing periods = %#v, want non-duration ongoing row", ongoingDay.Periods)
	}
	if resp.Aggregate.AverageTotalLabel != "20 min" {
		t.Fatalf("AverageTotalLabel = %q, want 60 completed minutes / 3 selected days", resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.AverageCompletedLabel != "0.3" {
		t.Fatalf("AverageCompletedLabel = %q, want one completed period / 3 selected days", resp.Aggregate.AverageCompletedLabel)
	}
}

func TestBuildSleepInsightsNoDataYieldsEmptyAggregate(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, rangeEnd := sleepInsightsWindow(7, loc, now)

	resp := buildSleepInsights(nil, 7, rangeStart, rangeEnd)

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
	rangeStart, rangeEnd := sleepInsightsWindow(7, loc, now)

	events := []store.Event{
		sleepEvent(babyID, time.Date(2026, 7, 26, 10, 0, 0, 0, loc), "nap", intPtr(90)),
	}

	resp := buildSleepInsights(events, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasWakeWindow {
		t.Fatalf("HasWakeWindow = true, want false with only one sleep period")
	}
	if resp.Aggregate.AverageWakeWindowLabel != "Not yet available" {
		t.Fatalf("AverageWakeWindowLabel = %q, want %q", resp.Aggregate.AverageWakeWindowLabel, "Not yet available")
	}

	found := false
	for _, observation := range resp.Observations {
		if observation == "Not enough recorded sleep yet to calculate an average wake window." {
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

func TestSleepPeriodTimeRangeLabelUsesLocalClockAcrossDST(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	start := time.Date(2026, 10, 4, 1, 30, 0, 0, loc)
	end := start.Add(2 * time.Hour)

	if got := sleepPeriodTimeRangeLabel(start, end, false); got != "1:30 AM – 4:30 AM" {
		t.Fatalf("sleepPeriodTimeRangeLabel = %q, want local clocks across DST", got)
	}
}
