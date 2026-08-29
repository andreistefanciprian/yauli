package handlers

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParseFeedInsightsRangeDays(t *testing.T) {
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
			got, ok := parseFeedInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestBuildFeedInsightsDayAggregation(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	events := []store.Event{
		breastFeedEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 15),
		formulaFeedEvent(babyID, time.Date(2026, 7, 20, 11, 0, 0, 0, loc), 120),
		expressedFeedEvent(babyID, time.Date(2026, 7, 20, 14, 0, 0, 0, loc), 90),
		breastFeedEvent(babyID, time.Date(2026, 7, 21, 9, 0, 0, 0, loc), 20),
	}

	resp := buildFeedInsights(events, 7, rangeStart, rangeEnd)

	if len(resp.Days) != 7 {
		t.Fatalf("len(Days) = %d, want 7", len(resp.Days))
	}
	first := resp.Days[0]
	if first.LocalDate != "2026-07-20" {
		t.Fatalf("Days[0].LocalDate = %q, want 2026-07-20", first.LocalDate)
	}
	if !first.HasData || first.TotalCount != 3 || first.BreastCount != 1 || first.FormulaCount != 1 || first.ExpressedCount != 1 {
		t.Fatalf("Days[0] = %#v, want total 3 (1 breast, 1 formula, 1 expressed)", first)
	}
	if first.BreastMinutes != 15 || first.FormulaMl != 120 || first.ExpressedMl != 90 || first.BottleMl != 210 {
		t.Fatalf("Days[0] amounts = %#v, want breast 15min, formula 120ml, expressed 90ml, bottle 210ml", first)
	}
	if len(first.Events) != 3 {
		t.Fatalf("len(Days[0].Events) = %d, want 3", len(first.Events))
	}
	if first.Events[0].Kind != "breast" || first.Events[0].TimeLabel != "8:00 AM" || first.Events[0].DetailLabel != "15m" {
		t.Fatalf("Days[0].Events[0] = %#v, want breast at 8:00 AM, 15 min", first.Events[0])
	}
	if first.Events[1].DetailLabel != "120 ml" {
		t.Fatalf("Days[0].Events[1].DetailLabel = %q, want 120 ml", first.Events[1].DetailLabel)
	}

	second := resp.Days[1]
	if !second.HasData || second.TotalCount != 1 || second.BreastCount != 1 {
		t.Fatalf("Days[1] = %#v, want a single breast feed", second)
	}

	third := resp.Days[2]
	if third.HasData || third.TotalCount != 0 {
		t.Fatalf("Days[2] = %#v, want an empty day", third)
	}

	agg := resp.Aggregate
	if !agg.HasAnyData || agg.TotalCount != 4 || agg.RecordedDays != 2 {
		t.Fatalf("Aggregate = %#v, want 4 total across 2 recorded days", agg)
	}
	if agg.BreastCount != 2 || agg.FormulaCount != 1 || agg.ExpressedCount != 1 {
		t.Fatalf("aggregate feed-type counts = %d/%d/%d, want 2/1/1", agg.BreastCount, agg.FormulaCount, agg.ExpressedCount)
	}
	if agg.AveragePerDayLabel != "2.0" {
		t.Fatalf("AveragePerDayLabel = %q, want 2.0", agg.AveragePerDayLabel)
	}
	if agg.BreastTotalMinutes != 35 || agg.BreastTotalLabel != "35m" {
		t.Fatalf("breast total = %d/%q, want 35/35m", agg.BreastTotalMinutes, agg.BreastTotalLabel)
	}
	if agg.BottleTotalMl != 210 || agg.BottleTotalLabel != "210 ml" {
		t.Fatalf("bottle total = %d/%q, want 210/210 ml", agg.BottleTotalMl, agg.BottleTotalLabel)
	}
	if agg.BreastPercent == nil || *agg.BreastPercent != 50 {
		t.Fatalf("BreastPercent = %v, want 50", agg.BreastPercent)
	}
	if agg.FormulaPercent == nil || *agg.FormulaPercent != 25 {
		t.Fatalf("FormulaPercent = %v, want 25", agg.FormulaPercent)
	}
	if agg.ExpressedPercent == nil || *agg.ExpressedPercent != 25 {
		t.Fatalf("ExpressedPercent = %v, want 25", agg.ExpressedPercent)
	}
	if agg.BottleFormulaPercent == nil || *agg.BottleFormulaPercent != 57 {
		t.Fatalf("BottleFormulaPercent = %v, want 57 (120ml of 210ml)", agg.BottleFormulaPercent)
	}
	if agg.BottleExpressedPercent == nil || *agg.BottleExpressedPercent != 43 {
		t.Fatalf("BottleExpressedPercent = %v, want 43 (90ml of 210ml)", agg.BottleExpressedPercent)
	}
	if !agg.HasAverageGap {
		t.Fatalf("HasAverageGap = false, want true with 4 recorded feeds")
	}
}

func TestBuildFeedInsightsEmptyRange(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 27, 0, 0, 0, 0, loc)

	resp := buildFeedInsights(nil, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAnyData {
		t.Fatalf("HasAnyData = true, want false for an empty range")
	}
	if resp.Aggregate.BreastPercent != nil {
		t.Fatalf("BreastPercent = %v, want nil with no data", resp.Aggregate.BreastPercent)
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

func TestBuildFeedInsightsSingleFeedHasNoAverageGap(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	events := []store.Event{
		breastFeedEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 15),
	}

	resp := buildFeedInsights(events, 7, rangeStart, rangeEnd)

	if resp.Aggregate.HasAverageGap {
		t.Fatalf("HasAverageGap = true, want false with only one recorded feed")
	}
	if resp.Aggregate.AverageGapLabel != "Not yet available" {
		t.Fatalf("AverageGapLabel = %q, want fallback text", resp.Aggregate.AverageGapLabel)
	}
	if resp.Aggregate.BottleFormulaPercent != nil || resp.Aggregate.BottleExpressedPercent != nil {
		t.Fatalf("Bottle*Percent = %v/%v, want nil with no bottle feeds recorded", resp.Aggregate.BottleFormulaPercent, resp.Aggregate.BottleExpressedPercent)
	}
}

func TestBuildFeedInsightsPreservesMissingBreastDuration(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	resp := buildFeedInsights([]store.Event{
		ongoingBreastFeedEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc)),
	}, 7, rangeStart, rangeEnd)

	day := resp.Days[0]
	if day.TotalCount != 1 || day.BreastCount != 1 {
		t.Fatalf("day counts = total:%d breast:%d, want 1/1", day.TotalCount, day.BreastCount)
	}
	if day.BreastMinutes != 0 {
		t.Fatalf("BreastMinutes = %d, want no recorded duration", day.BreastMinutes)
	}
	if len(day.Events) != 1 || day.Events[0].DetailLabel != "Ongoing" {
		t.Fatalf("Events = %#v, want one ongoing breast feed", day.Events)
	}
	if resp.Aggregate.BreastTotalLabel != "Not yet available" {
		t.Fatalf("BreastTotalLabel = %q, want missing-duration fallback", resp.Aggregate.BreastTotalLabel)
	}
	if resp.Aggregate.BreastFeedsWithDurationCount != 0 || resp.Aggregate.BreastDurationBasisLabel != "Duration recorded for 0 of 1 breast feed" {
		t.Fatalf("breast duration basis = %d/%q, want ongoing-only disclosure", resp.Aggregate.BreastFeedsWithDurationCount, resp.Aggregate.BreastDurationBasisLabel)
	}
}

func TestBuildFeedInsightsTotalsOnlyRecordedBreastDurations(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 21, 0, 0, 0, 0, loc)

	resp := buildFeedInsights([]store.Event{
		breastFeedEvent(babyID, time.Date(2026, 7, 20, 8, 0, 0, 0, loc), 15),
		ongoingBreastFeedEvent(babyID, time.Date(2026, 7, 20, 11, 0, 0, 0, loc)),
	}, 7, rangeStart, rangeEnd)

	if resp.Aggregate.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want both recorded feed starts", resp.Aggregate.TotalCount)
	}
	if resp.Aggregate.BreastTotalMinutes != 15 || resp.Aggregate.BreastTotalLabel != "15m" {
		t.Fatalf("breast total = %d/%q, want only the recorded 15 minute duration", resp.Aggregate.BreastTotalMinutes, resp.Aggregate.BreastTotalLabel)
	}
	if resp.Aggregate.BreastFeedsWithDurationCount != 1 || resp.Aggregate.BreastDurationBasisLabel != "Duration recorded for 1 of 2 breast feeds" {
		t.Fatalf("breast duration basis = %d/%q, want partial-duration disclosure", resp.Aggregate.BreastFeedsWithDurationCount, resp.Aggregate.BreastDurationBasisLabel)
	}
}

func TestBuildFeedInsightsAttributesFullDurationToStartDay(t *testing.T) {
	loc := mustLoadLocation(t, "Australia/Adelaide")
	babyID := uuid.New()
	rangeStart := time.Date(2026, 7, 20, 0, 0, 0, 0, loc)
	rangeEnd := time.Date(2026, 7, 22, 0, 0, 0, 0, loc)

	resp := buildFeedInsights([]store.Event{
		// This feed overlaps the range but started before it, so it is excluded.
		breastFeedEvent(babyID, time.Date(2026, 7, 19, 23, 55, 0, 0, loc), 20),
		// This feed crosses midnight but remains entirely on its start day.
		breastFeedEvent(babyID, time.Date(2026, 7, 20, 23, 55, 0, 0, loc), 20),
	}, 7, rangeStart, rangeEnd)

	if resp.Aggregate.TotalCount != 1 || resp.Aggregate.BreastTotalMinutes != 20 {
		t.Fatalf("aggregate = %#v, want only the in-range feed start and its full duration", resp.Aggregate)
	}
	if len(resp.Days) != 2 || resp.Days[0].TotalCount != 1 || resp.Days[0].BreastMinutes != 20 {
		t.Fatalf("start day = %#v, want the full crossing feed", resp.Days)
	}
	if resp.Days[1].HasData || resp.Days[1].TotalCount != 0 || resp.Days[1].BreastMinutes != 0 {
		t.Fatalf("next day = %#v, want no carried feed duration", resp.Days[1])
	}
}

func TestFeedInsightPercentsStayValidAfterRounding(t *testing.T) {
	tests := []struct {
		name                                   string
		breast, formula, expressed             int
		wantBreast, wantFormula, wantExpressed int
	}{
		{
			name:        "two categories do not make an absent category negative",
			breast:      1,
			formula:     7,
			wantBreast:  13,
			wantFormula: 87,
		},
		{
			name:          "equal thirds allocate the rounding remainder once",
			breast:        1,
			formula:       1,
			expressed:     1,
			wantBreast:    34,
			wantFormula:   33,
			wantExpressed: 33,
		},
		{
			name: "no data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			breast, formula, expressed := feedInsightPercents(tt.breast, tt.formula, tt.expressed)
			if breast != tt.wantBreast || formula != tt.wantFormula || expressed != tt.wantExpressed {
				t.Fatalf("feedInsightPercents(%d, %d, %d) = %d, %d, %d; want %d, %d, %d",
					tt.breast, tt.formula, tt.expressed,
					breast, formula, expressed,
					tt.wantBreast, tt.wantFormula, tt.wantExpressed,
				)
			}
		})
	}
}

func breastFeedEvent(babyID uuid.UUID, occurredAt time.Time, durationMinutes int) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeFeed,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"type": string(FeedTypeBreast), "duration_minutes": durationMinutes},
	}
}

func ongoingBreastFeedEvent(babyID uuid.UUID, occurredAt time.Time) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeFeed,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"type": string(FeedTypeBreast)},
	}
}

func formulaFeedEvent(babyID uuid.UUID, occurredAt time.Time, amountMl int) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeFeed,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"type": string(FeedTypeFormula), "amount_ml": amountMl},
	}
}

func expressedFeedEvent(babyID uuid.UUID, occurredAt time.Time, amountMl int) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeFeed,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"type": string(FeedTypeExpressed), "amount_ml": amountMl},
	}
}
