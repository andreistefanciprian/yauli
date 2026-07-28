package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParseGrowthInsightsRangeDays(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   int
		wantOK bool
	}{
		{name: "empty defaults to 180", raw: "", want: 180, wantOK: true},
		{name: "90 is allowed", raw: "90", want: 90, wantOK: true},
		{name: "365 is allowed", raw: "365", want: 365, wantOK: true},
		{name: "0 means all time", raw: "0", want: 0, wantOK: true},
		{name: "30 is rejected", raw: "30", wantOK: false},
		{name: "non-numeric is rejected", raw: "abc", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseGrowthInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseGrowthInsightsMetric(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{name: "empty defaults to weight", raw: "", want: "weight", wantOK: true},
		{name: "length is allowed", raw: "length", want: "length", wantOK: true},
		{name: "head_circumference is allowed", raw: "head_circumference", want: "head_circumference", wantOK: true},
		{name: "unknown is rejected", raw: "height", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseGrowthInsightsMetric(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("metric = %q, want %q", got, tt.want)
			}
		})
	}
}

func growthEvent(babyID uuid.UUID, occurredAt time.Time, weightGrams *int, lengthCM *float64) store.Event {
	attrs := map[string]any{}
	if weightGrams != nil {
		attrs["weight_grams"] = float64(*weightGrams)
	}
	if lengthCM != nil {
		attrs["length_cm"] = *lengthCM
	}
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeGrowthMeasurement,
		OccurredAt: occurredAt,
		Attributes: attrs,
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestBuildGrowthInsightsComputesChangeAndAggregate(t *testing.T) {
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	events := []store.Event{
		growthEvent(babyID, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC), intPtr(3800), nil),
		growthEvent(babyID, time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC), intPtr(4100), nil),
		growthEvent(babyID, time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC), intPtr(4400), nil),
	}

	resp := buildGrowthInsights(events, "weight", 90, now)

	if !resp.HasAnyData || len(resp.Points) != 3 {
		t.Fatalf("resp = %#v, want three points with data", resp)
	}
	if resp.Points[0].ChangeLabel != "First recorded measurement" {
		t.Fatalf("first point change = %q, want the first-measurement fallback", resp.Points[0].ChangeLabel)
	}
	if resp.Points[1].ChangeLabel != "+0.300 kg" {
		t.Fatalf("second point change = %q, want +0.300 kg", resp.Points[1].ChangeLabel)
	}
	if resp.Points[2].ValueLabel != "4.400 kg" {
		t.Fatalf("third point value = %q, want 4.400 kg", resp.Points[2].ValueLabel)
	}
	if resp.Aggregate.ChangeOverallLabel != "+0.600 kg" {
		t.Fatalf("ChangeOverallLabel = %q, want +0.600 kg", resp.Aggregate.ChangeOverallLabel)
	}
	if resp.Aggregate.AverageIntervalDaysLabel != "10 days" {
		t.Fatalf("AverageIntervalDaysLabel = %q, want 10 days", resp.Aggregate.AverageIntervalDaysLabel)
	}
	if len(resp.Observations) == 0 {
		t.Fatalf("observations = %#v, want at least one", resp.Observations)
	}
}

func TestBuildGrowthInsightsChangeSurvivesRangeTrim(t *testing.T) {
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	events := []store.Event{
		// Outside the 90-day window, but still the true "previous" measurement
		// for the point that follows it.
		growthEvent(babyID, time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC), nil, floatPtr(54.0)),
		growthEvent(babyID, time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC), nil, floatPtr(56.1)),
		growthEvent(babyID, time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), nil, floatPtr(58.3)),
	}

	resp := buildGrowthInsights(events, "length", 90, now)

	if len(resp.Points) != 2 {
		t.Fatalf("len(Points) = %d, want 2 (only the two in-range measurements)", len(resp.Points))
	}
	if resp.Points[0].ChangeLabel == "First recorded measurement" {
		t.Fatalf("first in-range point change = %q, want a real change against the out-of-range prior measurement", resp.Points[0].ChangeLabel)
	}
	if resp.Points[0].ChangeLabel != "+2.1 cm" {
		t.Fatalf("first in-range point change = %q, want +2.1 cm", resp.Points[0].ChangeLabel)
	}
}

func TestBuildGrowthInsightsSingleMeasurementFallsBack(t *testing.T) {
	babyID := uuid.New()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	events := []store.Event{
		growthEvent(babyID, time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), intPtr(3800), nil),
	}

	resp := buildGrowthInsights(events, "weight", 0, now)

	if resp.Aggregate.AverageIntervalCaption != "Needs more than one recorded measurement" {
		t.Fatalf("AverageIntervalCaption = %q, want the fallback caption", resp.Aggregate.AverageIntervalCaption)
	}
	if resp.Aggregate.ChangeOverallCaption != "Needs more than one recorded measurement" {
		t.Fatalf("ChangeOverallCaption = %q, want the fallback caption", resp.Aggregate.ChangeOverallCaption)
	}
	if resp.RangeLabel != "All time" {
		t.Fatalf("RangeLabel = %q, want %q for range 0", resp.RangeLabel, "All time")
	}
}

func TestBuildGrowthInsightsNoDataYieldsEmptyResponse(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	resp := buildGrowthInsights(nil, "weight", 180, now)

	if resp.HasAnyData {
		t.Fatalf("HasAnyData = true, want false for no events")
	}
	if len(resp.Points) != 0 || len(resp.Observations) != 0 {
		t.Fatalf("resp = %#v, want empty points and observations", resp)
	}
}

func TestGrowthInsightShowLabel(t *testing.T) {
	tests := []struct {
		total int
		index int
		want  bool
	}{
		{total: 1, index: 0, want: true},
		{total: 5, index: 2, want: true},
		{total: 10, index: 1, want: false},
		{total: 10, index: 2, want: true},
		{total: 10, index: 9, want: true},
	}
	for _, tt := range tests {
		if got := growthInsightShowLabel(tt.index, tt.total); got != tt.want {
			t.Fatalf("growthInsightShowLabel(%d, %d) = %v, want %v", tt.index, tt.total, got, tt.want)
		}
	}
}
