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

func TestClampSleepInsightsStartToBirthDate(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	rangeStart := time.Date(2026, 6, 28, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 28, 0, 0, 0, 0, loc)

	tests := []struct {
		name        string
		birthDate   string
		wantStart   time.Time
		wantClamped bool
	}{
		{name: "no birth date", wantStart: rangeStart},
		{name: "born before range", birthDate: "2026-06-01", wantStart: rangeStart},
		{name: "born on range start", birthDate: "2026-06-28", wantStart: rangeStart, wantClamped: true},
		{name: "born inside range", birthDate: "2026-06-30", wantStart: time.Date(2026, 6, 30, 0, 0, 0, 0, loc), wantClamped: true},
		{name: "born today", birthDate: "2026-07-28", wantStart: rangeEnd, wantClamped: true},
		{name: "future birth date is capped", birthDate: "2026-08-01", wantStart: rangeEnd, wantClamped: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotClamped, err := clampSleepInsightsStartToBirthDate(rangeStart, rangeEnd, tt.birthDate, loc)
			if err != nil {
				t.Fatalf("clampSleepInsightsStartToBirthDate: %v", err)
			}
			if !gotStart.Equal(tt.wantStart) || gotClamped != tt.wantClamped {
				t.Fatalf("start, clamped = %s, %v; want %s, %v", gotStart, gotClamped, tt.wantStart, tt.wantClamped)
			}
		})
	}
}

func TestBuildSleepInsightsUsesOnlyVisiblePostBirthDays(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 3, 0, 0, 0, 0, loc)
	events := []store.Event{
		sleepEvent(babyID, time.Date(2026, 7, 1, 9, 0, 0, 0, loc), "nap", intPtr(60)),
	}

	resp := buildSleepInsights(events, 30, rangeStart, rangeEnd)

	if resp.RangeDays != 30 {
		t.Fatalf("RangeDays = %d, want selected 30-day period retained", resp.RangeDays)
	}
	if len(resp.Days) != 3 {
		t.Fatalf("len(Days) = %d, want only three post-birth completed days", len(resp.Days))
	}
	if resp.Days[0].LocalDate != "2026-06-30" || resp.Days[2].LocalDate != "2026-07-02" {
		t.Fatalf("Days = %#v, want Jun 30 through Jul 2", resp.Days)
	}
	if resp.RangeLabel != "Jun 30 – Jul 2" {
		t.Fatalf("RangeLabel = %q, want visible post-birth range", resp.RangeLabel)
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
	if len(second.Periods) != 1 || second.Periods[0].TimeRangeLabel != "10:00 PM – Next day" || !second.Periods[0].ContinuesNextDay {
		t.Fatalf("second-day periods = %#v, want start time with next-day boundary", second.Periods)
	}
	if len(third.Periods) != 1 || third.Periods[0].TimeRangeLabel != "Previous day – 2:00 AM" || !third.Periods[0].StartedPreviousDay {
		t.Fatalf("third-day periods = %#v, want previous-day boundary with stop time", third.Periods)
	}
	if third.CarryoverNote != "1 listed sleep period started the previous day and continued into this day. Its recorded time is included in the totals, but it is not counted under Sleep periods started." {
		t.Fatalf("third-day CarryoverNote = %q, want previous-day context", third.CarryoverNote)
	}
	if second.CompletedCount != 1 || third.CompletedCount != 0 {
		t.Fatalf("completed counts = %d, %d, want overnight sleep counted once on its start day", second.CompletedCount, third.CompletedCount)
	}
	if resp.Aggregate.AverageTotalLabel != "3h" {
		t.Fatalf("AverageTotalLabel = %q, want 540 minutes / 3 days", resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.AverageCompletedLabel != "0.7" {
		t.Fatalf("AverageCompletedLabel = %q, want two sleeps starting in the range / 3 days", resp.Aggregate.AverageCompletedLabel)
	}
	if resp.Aggregate.RecordedDays != 3 {
		t.Fatalf("RecordedDays = %d, want 3", resp.Aggregate.RecordedDays)
	}
}

func TestBuildSleepInsightsFormatsUTCEventsInBabyTimezone(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 28, 0, 0, 0, 0, loc)
	rangeEnd := rangeStart.AddDate(0, 0, 1)

	events := []store.Event{
		// Production event timestamps come back from PostgreSQL in UTC. This
		// sleep contributes only its post-midnight Adelaide portion.
		sleepEvent(babyID, time.Date(2026, 7, 27, 23, 0, 0, 0, loc).UTC(), "night", intPtr(147)),
		sleepEvent(babyID, time.Date(2026, 7, 28, 2, 49, 0, 0, loc).UTC(), "night", intPtr(178)),
	}

	resp := buildSleepInsights(events, 1, rangeStart, rangeEnd)

	if len(resp.Days) != 1 || len(resp.Days[0].Periods) != 2 {
		t.Fatalf("Days = %#v, want one day with two overlapping periods", resp.Days)
	}
	day := resp.Days[0]
	if day.Periods[0].TimeRangeLabel != "Previous day – 1:27 AM" || !day.Periods[0].StartedPreviousDay {
		t.Fatalf("carryover period = %#v, want previous-day boundary and Adelaide stop time", day.Periods[0])
	}
	if day.Periods[1].TimeRangeLabel != "2:49 AM – 5:47 AM" {
		t.Fatalf("started-today TimeRangeLabel = %q, want Adelaide local clock", day.Periods[1].TimeRangeLabel)
	}
	if day.Periods[1].StartedPreviousDay || day.Periods[1].ContinuesNextDay {
		t.Fatalf("started-today period = %#v, want no calendar-boundary flags", day.Periods[1])
	}
	if day.TotalMinutes != 265 || day.CompletedCount != 1 {
		t.Fatalf("total, started = %d, %d; want 265 overlapping minutes and one period started", day.TotalMinutes, day.CompletedCount)
	}
	if day.CarryoverNote == "" {
		t.Fatal("CarryoverNote is empty, want context for the extra listed period")
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
	if resp.Aggregate.AverageTotalLabel != "1h" {
		t.Fatalf("AverageTotalLabel = %q, want 60 completed minutes / 1 recorded day", resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.AverageCompletedLabel != "1.0" {
		t.Fatalf("AverageCompletedLabel = %q, want one completed period / 1 recorded day", resp.Aggregate.AverageCompletedLabel)
	}
	if resp.Aggregate.RecordedDays != 1 {
		t.Fatalf("RecordedDays = %d, want ongoing-only and empty days excluded", resp.Aggregate.RecordedDays)
	}
}

func TestBuildSleepInsightsExcludesMissingDaysFromAverages(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, rangeEnd := sleepInsightsWindow(3, loc, now)

	events := []store.Event{
		sleepEvent(babyID, time.Date(2026, 7, 24, 9, 0, 0, 0, loc), "nap", intPtr(60)),
		// July 25 is deliberately missing.
		sleepEvent(babyID, time.Date(2026, 7, 26, 9, 0, 0, 0, loc), "nap", intPtr(120)),
	}

	resp := buildSleepInsights(events, 3, rangeStart, rangeEnd)

	if resp.Days[1].HasData {
		t.Fatalf("middle day = %#v, want visible no-data gap", resp.Days[1])
	}
	if resp.Aggregate.RecordedDays != 2 {
		t.Fatalf("RecordedDays = %d, want 2", resp.Aggregate.RecordedDays)
	}
	if resp.Aggregate.AverageTotalLabel != "1h 30m" {
		t.Fatalf("AverageTotalLabel = %q, want 180 minutes / 2 recorded days", resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.AverageCompletedLabel != "1.0" {
		t.Fatalf("AverageCompletedLabel = %q, want two periods / 2 recorded days", resp.Aggregate.AverageCompletedLabel)
	}
}

func TestBuildSleepInsightsKeepsFullLongestSleepAcrossTodayBoundary(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 14, 45, 0, 0, loc)
	rangeStart, rangeEnd := sleepInsightsWindow(3, loc, now)

	events := []store.Event{
		// The completed sleep belongs to the selected range because it started
		// yesterday, but only its pre-midnight portion belongs in yesterday's
		// calendar-day total.
		sleepEvent(babyID, time.Date(2026, 7, 26, 20, 0, 0, 0, loc), "night", intPtr(480)),
	}

	resp := buildSleepInsights(events, 3, rangeStart, rangeEnd)

	lastDay := resp.Days[len(resp.Days)-1]
	if lastDay.LocalDate != "2026-07-26" || lastDay.TotalMinutes != 240 {
		t.Fatalf("last day = %#v, want July 26 with four pre-midnight hours", lastDay)
	}
	if lastDay.CompletedCount != 1 || resp.Aggregate.AverageCompletedLabel != "1.0" {
		t.Fatalf("completed count = %d, average = %q, want one sleep counted once on one recorded day", lastDay.CompletedCount, resp.Aggregate.AverageCompletedLabel)
	}
	if resp.Aggregate.RecordedDays != 1 || resp.Aggregate.AverageTotalLabel != "4h" {
		t.Fatalf("recorded days = %d, average total = %q, want four hours over one recorded day", resp.Aggregate.RecordedDays, resp.Aggregate.AverageTotalLabel)
	}
	if resp.Aggregate.LongestOverallLabel != "8h" {
		t.Fatalf("LongestOverallLabel = %q, want full eight-hour sleep period", resp.Aggregate.LongestOverallLabel)
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
	if resp.Aggregate.RecordedDays != 0 {
		t.Fatalf("RecordedDays = %d, want 0", resp.Aggregate.RecordedDays)
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
