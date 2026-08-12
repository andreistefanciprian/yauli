package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParsePumpInsightsRangeDays(t *testing.T) {
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
			got, ok := parsePumpInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildPumpInsightsDayAggregation(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	events := []store.Event{
		pumpEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 80, intPtr(18)),
		pumpEvent(babyID, time.Date(2026, 7, 20, 11, 0, 0, 0, loc), 100, intPtr(22)),
		pumpEvent(babyID, time.Date(2026, 7, 21, 9, 0, 0, 0, loc), 90, intPtr(20)),
	}

	resp := buildPumpInsights(events, 7, rangeStart, rangeEnd)

	if len(resp.Days) != 7 {
		t.Fatalf("len(Days) = %d, want 7", len(resp.Days))
	}
	first := resp.Days[0]
	if first.LocalDate != "2026-07-20" {
		t.Fatalf("Days[0].LocalDate = %q, want 2026-07-20", first.LocalDate)
	}
	if !first.HasData || first.SessionCount != 2 || first.TotalMl != 180 || first.TotalMinutes != 40 {
		t.Fatalf("Days[0] = %#v, want 2 sessions totalling 180 ml / 40 min", first)
	}
	if first.DurationLabel != "40m" {
		t.Fatalf("Days[0].DurationLabel = %q, want 40m", first.DurationLabel)
	}
	if len(first.Events) != 2 || first.Events[0].VolumeLabel != "80 ml" || first.Events[0].DurationLabel != "18m" {
		t.Fatalf("Days[0].Events[0] = %#v, want 80 ml / 18m", first.Events[0])
	}

	second := resp.Days[1]
	if !second.HasData || second.SessionCount != 1 || second.TotalMl != 90 {
		t.Fatalf("Days[1] = %#v, want a single 90 ml session", second)
	}

	third := resp.Days[2]
	if third.HasData || third.SessionCount != 0 {
		t.Fatalf("Days[2] = %#v, want an empty day", third)
	}

	if !resp.Aggregate.HasAnyData || resp.Aggregate.SessionCount != 3 || resp.Aggregate.RecordedDays != 2 {
		t.Fatalf("Aggregate = %#v, want 3 sessions across 2 recorded days", resp.Aggregate)
	}
	if resp.Aggregate.TotalMlLabel != "270 ml" {
		t.Fatalf("TotalMlLabel = %q, want 270 ml", resp.Aggregate.TotalMlLabel)
	}
	if resp.Aggregate.TotalDurationLabel != "1h" {
		t.Fatalf("TotalDurationLabel = %q, want 1h", resp.Aggregate.TotalDurationLabel)
	}
	if resp.Aggregate.AveragePerDayLabel != "1.5" {
		t.Fatalf("AveragePerDayLabel = %q, want 1.5", resp.Aggregate.AveragePerDayLabel)
	}
	if resp.Aggregate.AverageSessionMlLabel != "90 ml" {
		t.Fatalf("AverageSessionMlLabel = %q, want 90 ml", resp.Aggregate.AverageSessionMlLabel)
	}
	if !resp.Aggregate.HasAverageGap {
		t.Fatalf("HasAverageGap = false, want true with 3 recorded sessions")
	}
}

func TestBuildPumpInsightsEmptyRange(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	resp := buildPumpInsights(nil, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAnyData {
		t.Fatalf("HasAnyData = true, want false for an empty range")
	}
	if resp.Aggregate.TotalMlLabel != "" {
		t.Fatalf("TotalMlLabel = %q, want empty with no data", resp.Aggregate.TotalMlLabel)
	}
	if len(resp.Observations) != 0 {
		t.Fatalf("Observations = %v, want none with no data", resp.Observations)
	}
	for _, day := range resp.Days {
		if day.HasData {
			t.Fatalf("day %q HasData = true, want false", day.LocalDate)
		}
	}
}

func TestBuildPumpInsightsSingleSessionHasNoAverageGap(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	events := []store.Event{
		pumpEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 80, intPtr(18)),
	}

	resp := buildPumpInsights(events, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAverageGap {
		t.Fatalf("HasAverageGap = true, want false with only one recorded session")
	}
	if resp.Aggregate.AverageGapLabel != "Not yet available" {
		t.Fatalf("AverageGapLabel = %q, want fallback text", resp.Aggregate.AverageGapLabel)
	}
}

func TestBuildPumpInsightsPreservesMissingDuration(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	resp := buildPumpInsights([]store.Event{
		pumpEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 80, nil),
	}, 7, rangeStart, rangeEnd)

	day := resp.Days[0]
	if day.SessionCount != 1 || day.TotalMl != 80 {
		t.Fatalf("day = %#v, want 1 session of 80 ml", day)
	}
	if day.TotalMinutes != 0 || day.DurationLabel != "" {
		t.Fatalf("day duration = %d/%q, want no recorded duration", day.TotalMinutes, day.DurationLabel)
	}
	if len(day.Events) != 1 || day.Events[0].DurationLabel != "Ongoing" {
		t.Fatalf("Events = %#v, want an ongoing session", day.Events)
	}
	if resp.Aggregate.TotalDurationLabel != "Not yet available" {
		t.Fatalf("TotalDurationLabel = %q, want missing-duration fallback", resp.Aggregate.TotalDurationLabel)
	}
	if resp.Aggregate.AverageSessionDurationLabel != "Not yet available" {
		t.Fatalf("AverageSessionDurationLabel = %q, want missing-duration fallback", resp.Aggregate.AverageSessionDurationLabel)
	}
	if resp.Aggregate.SessionsWithDurationCount != 0 {
		t.Fatalf("SessionsWithDurationCount = %d, want 0", resp.Aggregate.SessionsWithDurationCount)
	}
}

func pumpEvent(babyID uuid.UUID, occurredAt time.Time, amountMl int, durationMinutes *int) store.Event {
	attributes := map[string]any{"amount_ml": amountMl}
	if durationMinutes != nil {
		attributes["duration_minutes"] = *durationMinutes
	}
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypePump,
		OccurredAt: occurredAt,
		Attributes: attributes,
	}
}
