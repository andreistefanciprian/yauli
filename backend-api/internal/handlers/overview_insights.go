package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

// overviewInsightsDefaultRangeDays matches every other Insights category's
// default range pill.
const overviewInsightsDefaultRangeDays = 30

var overviewInsightsAllowedRangeDays = map[int]bool{7: true, 30: true, 90: true}

// overviewInsightsResponse is the Insights Overview tab's "recorded stats"
// card payload: one aggregate figure per category rather than the
// day-by-day detail /insights/sleep etc. return. Deliberately lean —
// display strings are still pre-formatted here, but only the specific
// fields the card renders are included, not the full per-category
// Aggregate shape.
type overviewInsightsResponse struct {
	RangeDays int `json:"range_days"`
	// AgeLabel is how old the baby is as of today, in the baby's own
	// timezone — e.g. "6 weeks, 3 days old". Computed here rather than in
	// the frontend because it depends on "today" in that timezone, the same
	// calendar-boundary concern Growth/Health already handle for
	// age-at-event. Omitted when the baby's profile has no birth date.
	AgeLabel string `json:"age_label,omitempty"`
	// BirthDateLabel ("12 June 2026") travels alongside AgeLabel so the
	// frontend never needs the baby's profile just to render the Overview
	// tab's age card or ChatGPT summary — see ShowInsights in the frontend,
	// which skips GetCurrentBaby entirely for htmx partial re-renders now
	// that neither label depends on it.
	BirthDateLabel string              `json:"birth_date_label,omitempty"`
	Sleep          overviewSleepStats  `json:"sleep"`
	Feed           overviewFeedStats   `json:"feed"`
	Nappy          overviewNappyStats  `json:"nappy"`
	Pump           overviewPumpStats   `json:"pump"`
	Growth         overviewGrowthStats `json:"growth"`
	Health         overviewHealthStats `json:"health"`
}

type overviewSleepStats struct {
	Available  bool `json:"available"`
	HasAnyData bool `json:"has_any_data"`
	// AverageTotalLabel/NightPercent/AverageWakeWindowLabel mirror
	// sleepInsightAggregateResponse's identically-named fields.
	AverageTotalLabel      string `json:"average_total_label,omitempty"`
	NightPercent           *int   `json:"night_percent,omitempty"`
	HasWakeWindow          bool   `json:"has_wake_window"`
	AverageWakeWindowLabel string `json:"average_wake_window_label,omitempty"`
}

type overviewFeedStats struct {
	Available          bool   `json:"available"`
	HasAnyData         bool   `json:"has_any_data"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	BreastTotalLabel   string `json:"breast_total_label,omitempty"`
	BottleTotalLabel   string `json:"bottle_total_label,omitempty"`
}

type overviewNappyStats struct {
	Available          bool   `json:"available"`
	HasAnyData         bool   `json:"has_any_data"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	HasAverageGap      bool   `json:"has_average_gap"`
	AverageGapLabel    string `json:"average_gap_label,omitempty"`
}

type overviewPumpStats struct {
	Available    bool   `json:"available"`
	HasAnyData   bool   `json:"has_any_data"`
	SessionCount int    `json:"session_count"`
	TotalMlLabel string `json:"total_ml_label,omitempty"`
}

type overviewGrowthStats struct {
	Available        bool   `json:"available"`
	HasAnyData       bool   `json:"has_any_data"`
	LatestValueLabel string `json:"latest_value_label,omitempty"`
	// HasBirthWeight is false when the baby's profile has no recorded birth
	// weight to diff against — distinct from HasAnyData, which is about
	// whether any growth measurement exists at all.
	HasBirthWeight        bool   `json:"has_birth_weight"`
	ChangeSinceBirthLabel string `json:"change_since_birth_label,omitempty"`

	// Length mirrors the weight fields above exactly — same reasoning, same
	// "has data" / "has a birth baseline to diff against" split — computed
	// from the same already-fetched growth events, just a different metric.
	HasLengthData               bool   `json:"has_length_data"`
	LatestLengthLabel           string `json:"latest_length_label,omitempty"`
	HasBirthLength              bool   `json:"has_birth_length"`
	LengthChangeSinceBirthLabel string `json:"length_change_since_birth_label,omitempty"`
}

// GetOverviewInsights returns the small set of range-level aggregates the
// Insights Overview tab's "recorded stats" card needs. Sleep/Feeds/Nappies/
// Pump are computed over the requested range, reusing the exact same
// builder functions their own /insights/{category} endpoints use. Growth and
// Health always report against the baby's whole recorded history because
// "since birth" and recorded health history are not range-scoped concepts.
//
// The six sources are fetched concurrently and degrade independently: if
// one lookup fails, its stats report available=false (and the error is
// logged) while the other five still render. A successful lookup with no
// matching records reports available=true and empty category-specific data,
// so the frontend never misrepresents an outage as an empty history.
func (h *Handlers) GetOverviewInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	rangeDays, ok := parseOverviewInsightsRangeDays(r.URL.Query().Get("range"))
	if !ok {
		writeError(w, http.StatusBadRequest, "range must be one of: 7, 30, 90")
		return
	}

	loc, err := time.LoadLocation(baby.Timezone)
	if err != nil {
		log.Printf("load baby timezone %q: %v", baby.Timezone, err)
		writeError(w, http.StatusInternalServerError, "failed to resolve baby timezone")
		return
	}

	now := time.Now().In(loc)
	rangeStart, rangeEnd := sleepInsightsWindow(rangeDays, loc, now)
	rangeStart, rangeStartsAtBirth, err := clampSleepInsightsStartToBirthDate(rangeStart, rangeEnd, baby.BirthDate, loc)
	if err != nil {
		log.Printf("parse baby birth date %q: %v", baby.BirthDate, err)
		writeError(w, http.StatusInternalServerError, "failed to resolve baby birth date")
		return
	}

	// Mirrors GetSleepInsights: include the preceding local day so a
	// completed overnight sleep can contribute its post-midnight portion to
	// the first requested day.
	sleepQueryStart := rangeStart.AddDate(0, 0, -1)
	if rangeStartsAtBirth {
		sleepQueryStart = rangeStart
	}

	ctx := r.Context()
	response := overviewInsightsResponse{RangeDays: rangeDays}

	if birthStart, err := growthInsightsBirthStart(baby.BirthDate, loc, now); err != nil {
		log.Printf("overview insights: parse baby birth date %q for age: %v", baby.BirthDate, err)
	} else if birthStart != nil {
		response.AgeLabel = overviewBabyAgeLabel(*birthStart, now)
		response.BirthDateLabel = birthStart.Format("2 January 2006")
	}

	// Each goroutine writes only to its own field of response — disjoint
	// memory, so this needs no locking; wg.Wait() is the happens-before edge
	// before writeJSON reads the assembled struct.
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		response.Sleep = h.overviewSleepStats(ctx, baby, sleepQueryStart, rangeStart, rangeEnd, rangeDays)
	}()
	go func() {
		defer wg.Done()
		response.Feed = h.overviewFeedStats(ctx, baby, rangeStart, rangeEnd, rangeDays)
	}()
	go func() {
		defer wg.Done()
		response.Nappy = h.overviewNappyStats(ctx, baby, rangeStart, rangeEnd, rangeDays)
	}()
	go func() {
		defer wg.Done()
		response.Pump = h.overviewPumpStats(ctx, baby, rangeStart, rangeEnd, rangeDays)
	}()
	go func() {
		defer wg.Done()
		response.Growth = h.overviewGrowthStats(ctx, baby, loc, now)
	}()
	go func() {
		defer wg.Done()
		response.Health = h.overviewHealthStats(ctx, baby, loc, now)
	}()
	wg.Wait()

	writeJSON(w, http.StatusOK, response)
}

func parseOverviewInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return overviewInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !overviewInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

func (h *Handlers) overviewSleepStats(ctx context.Context, baby store.Baby, queryStart, rangeStart, rangeEnd time.Time, rangeDays int) overviewSleepStats {
	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypeSleep, queryStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("overview insights: list sleep events: %v", err)
		return overviewSleepStats{}
	}
	agg := buildSleepInsights(events, rangeDays, rangeStart, rangeEnd).Aggregate
	return overviewSleepStats{
		Available:              true,
		HasAnyData:             agg.HasAnyData,
		AverageTotalLabel:      agg.AverageTotalLabel,
		NightPercent:           agg.NightPercent,
		HasWakeWindow:          agg.HasWakeWindow,
		AverageWakeWindowLabel: agg.AverageWakeWindowLabel,
	}
}

func (h *Handlers) overviewFeedStats(ctx context.Context, baby store.Baby, rangeStart, rangeEnd time.Time, rangeDays int) overviewFeedStats {
	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypeFeed, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("overview insights: list feed events: %v", err)
		return overviewFeedStats{}
	}
	agg := buildFeedInsights(events, rangeDays, rangeStart, rangeEnd).Aggregate
	return overviewFeedStats{
		Available:          true,
		HasAnyData:         agg.HasAnyData,
		AveragePerDayLabel: agg.AveragePerDayLabel,
		BreastTotalLabel:   agg.BreastTotalLabel,
		BottleTotalLabel:   agg.BottleTotalLabel,
	}
}

func (h *Handlers) overviewNappyStats(ctx context.Context, baby store.Baby, rangeStart, rangeEnd time.Time, rangeDays int) overviewNappyStats {
	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypeNappy, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("overview insights: list nappy events: %v", err)
		return overviewNappyStats{}
	}
	agg := buildNappyInsights(events, rangeDays, rangeStart, rangeEnd).Aggregate
	return overviewNappyStats{
		Available:          true,
		HasAnyData:         agg.HasAnyData,
		AveragePerDayLabel: agg.AveragePerDayLabel,
		HasAverageGap:      agg.HasAverageGap,
		AverageGapLabel:    agg.AverageGapLabel,
	}
}

func (h *Handlers) overviewPumpStats(ctx context.Context, baby store.Baby, rangeStart, rangeEnd time.Time, rangeDays int) overviewPumpStats {
	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypePump, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("overview insights: list pump events: %v", err)
		return overviewPumpStats{}
	}
	agg := buildPumpInsights(events, rangeDays, rangeStart, rangeEnd).Aggregate
	return overviewPumpStats{
		Available:    true,
		HasAnyData:   agg.HasAnyData,
		SessionCount: agg.SessionCount,
		TotalMlLabel: agg.TotalMlLabel,
	}
}

func (h *Handlers) overviewGrowthStats(ctx context.Context, baby store.Baby, loc *time.Location, now time.Time) overviewGrowthStats {
	birthStart, err := growthInsightsBirthStart(baby.BirthDate, loc, now)
	if err != nil {
		log.Printf("overview insights: parse baby birth date %q: %v", baby.BirthDate, err)
		return overviewGrowthStats{}
	}

	// Full history, same as GetGrowthInsights — Overview always reports
	// against the whole recorded history, not the top pill's range.
	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypeGrowthMeasurement, time.Time{}, now.AddDate(0, 0, 1), growthInsightsEventsLimit)
	if err != nil {
		log.Printf("overview insights: list growth events: %v", err)
		return overviewGrowthStats{}
	}

	resp := buildGrowthInsights(events, growthInsightsDefaultMetric, growthInsightsAllTimeRangeDays, now, birthStart)
	stats := overviewGrowthStats{Available: true}
	if resp.HasAnyData {
		stats.HasAnyData = true
		stats.LatestValueLabel = resp.Aggregate.LatestValueLabel
		if birthWeight, err := strconv.ParseFloat(baby.BirthWeightKg, 64); err == nil {
			latest := resp.Points[len(resp.Points)-1].Value
			stats.HasBirthWeight = true
			stats.ChangeSinceBirthLabel = growthChangeLabel(growthInsightsDefaultMetric, latest-birthWeight*1000)
		}
	}

	// Reuses the same already-fetched events — buildGrowthInsights is
	// generic over metric, so no second store round-trip is needed to add
	// length alongside weight.
	lengthResp := buildGrowthInsights(events, "length", growthInsightsAllTimeRangeDays, now, birthStart)
	if lengthResp.HasAnyData {
		stats.HasLengthData = true
		stats.LatestLengthLabel = lengthResp.Aggregate.LatestValueLabel
		if birthLength, err := strconv.ParseFloat(baby.BirthLengthCm, 64); err == nil {
			latest := lengthResp.Points[len(lengthResp.Points)-1].Value
			stats.HasBirthLength = true
			stats.LengthChangeSinceBirthLabel = growthChangeLabel("length", latest-birthLength)
		}
	}

	return stats
}

// overviewBabyAgeLabel formats how old the baby is as of `now`, matching
// the Insights Overview age card's four tiers: under a week in days, under
// 13 weeks in weeks (+ leftover days), under 24 months in months, then years
// and leftover months. birthStart and now must already be in the baby's own
// timezone (see growthInsightsBirthStart / GetOverviewInsights).
func overviewBabyAgeLabel(birthStart, now time.Time) string {
	// Compare local calendar dates, not elapsed hours — see
	// healthInsightsAgeLabel for why (DST transitions).
	birthDate := time.Date(birthStart.Year(), birthStart.Month(), birthStart.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := int(today.Sub(birthDate).Hours() / 24)
	if days < 0 {
		days = 0
	}

	if days < 7 {
		return fmt.Sprintf("%d %s old", days, pluralWord(days, "day", "days"))
	}

	weeks := days / 7
	if weeks < 13 {
		if remDays := days % 7; remDays != 0 {
			return fmt.Sprintf("%d %s, %d %s old", weeks, pluralWord(weeks, "week", "weeks"), remDays, pluralWord(remDays, "day", "days"))
		}
		return fmt.Sprintf("%d %s old", weeks, pluralWord(weeks, "week", "weeks"))
	}

	months := monthsBetween(birthStart, now)
	if months < 24 {
		return fmt.Sprintf("%d %s old", months, pluralWord(months, "month", "months"))
	}

	years, remMonths := months/12, months%12
	if remMonths == 0 {
		return fmt.Sprintf("%d %s old", years, pluralWord(years, "year", "years"))
	}
	return fmt.Sprintf("%dy %dm old", years, remMonths)
}
