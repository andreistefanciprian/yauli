package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParseNappyInsightsRangeDays(t *testing.T) {
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
			got, ok := parseNappyInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNappyInsightKind(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{kind: "wet", want: "wee"},
		{kind: "poo", want: "poo"},
		{kind: "both", want: "mixed"},
		{kind: "", want: "wee"},
	}
	for _, tt := range tests {
		got := nappyInsightKind(map[string]any{"kind": tt.kind})
		if got != tt.want {
			t.Fatalf("nappyInsightKind(%q) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestBuildNappyInsightsDayAggregation(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	events := []store.Event{
		nappyEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), "wet"),
		nappyEvent(babyID, time.Date(2026, 7, 20, 11, 0, 0, 0, loc), "poo"),
		nappyEvent(babyID, time.Date(2026, 7, 20, 14, 0, 0, 0, loc), "both"),
		nappyEvent(babyID, time.Date(2026, 7, 21, 9, 0, 0, 0, loc), "wet"),
	}

	resp := buildNappyInsights(events, 7, rangeStart, rangeEnd)

	if len(resp.Days) != 7 {
		t.Fatalf("len(Days) = %d, want 7", len(resp.Days))
	}
	first := resp.Days[0]
	if first.LocalDate != "2026-07-20" {
		t.Fatalf("Days[0].LocalDate = %q, want 2026-07-20", first.LocalDate)
	}
	if !first.HasData || first.TotalCount != 3 || first.WeeCount != 1 || first.PooCount != 1 || first.MixedCount != 1 {
		t.Fatalf("Days[0] = %#v, want total 3 (1 wee, 1 poo, 1 mixed)", first)
	}
	if len(first.Events) != 3 {
		t.Fatalf("len(Days[0].Events) = %d, want 3", len(first.Events))
	}
	if first.Events[0].Kind != "wee" || first.Events[0].TimeLabel != "8:00 AM" {
		t.Fatalf("Days[0].Events[0] = %#v, want wee at 8:00 AM", first.Events[0])
	}

	second := resp.Days[1]
	if !second.HasData || second.TotalCount != 1 || second.WeeCount != 1 {
		t.Fatalf("Days[1] = %#v, want a single wee", second)
	}

	third := resp.Days[2]
	if third.HasData || third.TotalCount != 0 {
		t.Fatalf("Days[2] = %#v, want an empty day", third)
	}

	if !resp.Aggregate.HasAnyData || resp.Aggregate.TotalCount != 4 || resp.Aggregate.RecordedDays != 2 {
		t.Fatalf("Aggregate = %#v, want 4 total across 2 recorded days", resp.Aggregate)
	}
	if resp.Aggregate.AveragePerDayLabel != "2.0" {
		t.Fatalf("AveragePerDayLabel = %q, want 2.0", resp.Aggregate.AveragePerDayLabel)
	}
	if resp.Aggregate.WeePercent == nil || *resp.Aggregate.WeePercent != 50 {
		t.Fatalf("WeePercent = %v, want 50", resp.Aggregate.WeePercent)
	}
	if resp.Aggregate.PooPercent == nil || *resp.Aggregate.PooPercent != 25 {
		t.Fatalf("PooPercent = %v, want 25", resp.Aggregate.PooPercent)
	}
	if resp.Aggregate.MixedPercent == nil || *resp.Aggregate.MixedPercent != 25 {
		t.Fatalf("MixedPercent = %v, want 25", resp.Aggregate.MixedPercent)
	}
	if !resp.Aggregate.HasAverageGap {
		t.Fatalf("HasAverageGap = false, want true with 4 recorded changes")
	}
}

func TestNappyInsightSize(t *testing.T) {
	tests := []struct {
		size string
		want string
	}{
		{size: "smear", want: "smear"},
		{size: "small", want: "small"},
		{size: "medium", want: "medium"},
		{size: "large", want: "large"},
		{size: "blowout", want: "blowout"},
		{size: "", want: ""},
		{size: "bogus", want: ""},
	}
	for _, tt := range tests {
		got := nappyInsightSize(map[string]any{"poo_size": tt.size})
		if got != tt.want {
			t.Fatalf("nappyInsightSize(%q) = %q, want %q", tt.size, got, tt.want)
		}
	}
	if got := nappyInsightSize(map[string]any{}); got != "" {
		t.Fatalf("nappyInsightSize(missing attribute) = %q, want empty", got)
	}
}

func TestBuildNappyInsightsMarkerCountsAndSizes(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 22, 0, 0, 0, 0, loc)

	events := []store.Event{
		nappyEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), "wet"),
		nappyEventWithSize(babyID, time.Date(2026, 7, 20, 11, 0, 0, 0, loc), "poo", "blowout"),
		nappyEventWithSize(babyID, time.Date(2026, 7, 20, 14, 0, 0, 0, loc), "poo", "large"),
		nappyEventWithSize(babyID, time.Date(2026, 7, 21, 9, 0, 0, 0, loc), "both", "small"),
		nappyEventWithSize(babyID, time.Date(2026, 7, 21, 12, 0, 0, 0, loc), "poo", "large"),
	}

	resp := buildNappyInsights(events, 7, rangeStart, rangeEnd)

	first := resp.Days[0]
	if first.Events[0].Size != "" {
		t.Fatalf("Days[0].Events[0].Size = %q, want empty for a wee-only event", first.Events[0].Size)
	}
	if first.Events[1].Size != "blowout" {
		t.Fatalf("Days[0].Events[1].Size = %q, want blowout", first.Events[1].Size)
	}
	if first.blowoutCount != 1 || first.largeCount != 1 {
		t.Fatalf("Days[0] blowoutCount/largeCount = %d/%d, want 1/1", first.blowoutCount, first.largeCount)
	}

	second := resp.Days[1]
	if second.Events[0].Size != "small" {
		t.Fatalf("Days[1].Events[0].Size = %q, want small", second.Events[0].Size)
	}
	if second.blowoutCount != 0 || second.largeCount != 1 {
		t.Fatalf("Days[1] blowoutCount/largeCount = %d/%d, want 0/1", second.blowoutCount, second.largeCount)
	}

	if resp.Aggregate.BlowoutCount != 1 || resp.Aggregate.LargeCount != 2 {
		t.Fatalf("Aggregate blowout/large = %d/%d, want 1/2", resp.Aggregate.BlowoutCount, resp.Aggregate.LargeCount)
	}
}

func TestBuildNappyInsightsEmptyRange(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	resp := buildNappyInsights(nil, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAnyData {
		t.Fatalf("HasAnyData = true, want false for an empty range")
	}
	if resp.Aggregate.WeePercent != nil {
		t.Fatalf("WeePercent = %v, want nil with no data", resp.Aggregate.WeePercent)
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

func TestBuildNappyInsightsSingleChangeHasNoAverageGap(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	events := []store.Event{
		nappyEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), "wet"),
	}

	resp := buildNappyInsights(events, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAverageGap {
		t.Fatalf("HasAverageGap = true, want false with only one recorded change")
	}
	if resp.Aggregate.AverageGapLabel != "Not yet available" {
		t.Fatalf("AverageGapLabel = %q, want fallback text", resp.Aggregate.AverageGapLabel)
	}
}

func TestNappyInsightPercentsStayValidAfterRounding(t *testing.T) {
	tests := []struct {
		name                        string
		wee, poo, mixed             int
		wantWee, wantPoo, wantMixed int
	}{
		{
			name:      "two categories do not make an absent category negative",
			wee:       1,
			poo:       7,
			wantWee:   13,
			wantPoo:   87,
			wantMixed: 0,
		},
		{
			name:      "equal thirds allocate the rounding remainder once",
			wee:       1,
			poo:       1,
			mixed:     1,
			wantWee:   34,
			wantPoo:   33,
			wantMixed: 33,
		},
		{
			name: "no data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wee, poo, mixed := nappyInsightPercents(tt.wee, tt.poo, tt.mixed)
			if wee != tt.wantWee || poo != tt.wantPoo || mixed != tt.wantMixed {
				t.Fatalf("nappyInsightPercents(%d, %d, %d) = %d, %d, %d; want %d, %d, %d",
					tt.wee, tt.poo, tt.mixed,
					wee, poo, mixed,
					tt.wantWee, tt.wantPoo, tt.wantMixed,
				)
			}
		})
	}
}

func nappyEvent(babyID uuid.UUID, occurredAt time.Time, kind string) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeNappy,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"kind": kind},
	}
}

func nappyEventWithSize(babyID uuid.UUID, occurredAt time.Time, kind, pooSize string) store.Event {
	ev := nappyEvent(babyID, occurredAt, kind)
	ev.Attributes["poo_size"] = pooSize
	return ev
}
