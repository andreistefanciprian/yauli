package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

// growthInsightsDefaultRangeDays matches the Insights page's default Growth
// range selection (the second of the four range pills).
const growthInsightsDefaultRangeDays = 180

// growthInsightsAllTimeRangeDays is the sentinel "range" value meaning no
// cutoff — every recorded measurement is included.
const growthInsightsAllTimeRangeDays = 0

var growthInsightsAllowedRangeDays = map[int]bool{90: true, 180: true, 365: true, growthInsightsAllTimeRangeDays: true}

// growthInsightsEventsLimit is generous on purpose: growth measurements are
// logged far less often than sleeps or feeds (typically at checkups), so the
// full history for a baby is small even over years.
const growthInsightsEventsLimit = 5000

const growthInsightsDefaultMetric = "weight"

type growthMetricConfig struct {
	Label string
	Unit  string
}

var growthInsightsMetrics = map[string]growthMetricConfig{
	"weight":             {Label: "Weight", Unit: "kg"},
	"length":             {Label: "Length", Unit: "cm"},
	"head_circumference": {Label: "Head circumference", Unit: "cm"},
}

type growthInsightsResponse struct {
	Metric       string                         `json:"metric"`
	MetricLabel  string                         `json:"metric_label"`
	Unit         string                         `json:"unit"`
	RangeDays    int                            `json:"range_days"`
	RangeLabel   string                         `json:"range_label"`
	RangeStart   *time.Time                     `json:"range_start,omitempty"`
	RangeEnd     time.Time                      `json:"range_end"`
	HasAnyData   bool                           `json:"has_any_data"`
	Points       []growthInsightPointResponse   `json:"points"`
	Aggregate    growthInsightAggregateResponse `json:"aggregate"`
	Observations []string                       `json:"observations"`
}

type growthInsightPointResponse struct {
	ID          string    `json:"id"`
	OccurredAt  time.Time `json:"occurred_at"`
	LocalDate   string    `json:"local_date"`
	Label       string    `json:"label"`
	ShowLabel   bool      `json:"show_label"`
	FullLabel   string    `json:"full_label"`
	Value       float64   `json:"value"`
	ValueLabel  string    `json:"value_label"`
	ChangeLabel string    `json:"change_label"`
}

type growthInsightAggregateResponse struct {
	Count                    int    `json:"count"`
	LatestValueLabel         string `json:"latest_value_label,omitempty"`
	AverageIntervalDaysLabel string `json:"average_interval_days_label,omitempty"`
	AverageIntervalCaption   string `json:"average_interval_caption,omitempty"`
	ChangeOverallLabel       string `json:"change_overall_label,omitempty"`
	ChangeOverallCaption     string `json:"change_overall_caption,omitempty"`
}

// GetGrowthInsights returns one recorded measurement per data point for the
// requested metric (weight/length/head circumference) over a range ending
// today, plus range-level aggregates and factual observations. As with Sleep
// Insights, every display string is pre-formatted here so the frontend only
// lays the data out, not calculate it.
func (h *Handlers) GetGrowthInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	metric, ok := parseGrowthInsightsMetric(r.URL.Query().Get("metric"))
	if !ok {
		writeError(w, http.StatusBadRequest, "metric must be one of: weight, length, head_circumference")
		return
	}

	rangeDays, ok := parseGrowthInsightsRangeDays(r.URL.Query().Get("range"))
	if !ok {
		writeError(w, http.StatusBadRequest, "range must be one of: 90, 180, 365, 0 (all time)")
		return
	}

	loc, err := time.LoadLocation(baby.Timezone)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve baby timezone")
		return
	}
	now := time.Now().In(loc)
	birthStart, err := growthInsightsBirthStart(baby.BirthDate, loc, now)
	if err != nil {
		log.Printf("parse baby birth date %q: %v", baby.BirthDate, err)
		writeError(w, http.StatusInternalServerError, "failed to resolve baby birth date")
		return
	}

	// The full history is fetched regardless of range so that a point right
	// at the edge of the selected range can still report its true change
	// since the previous measurement, even when that measurement falls
	// outside the window.
	events, err := h.Store.ListEventsByType(r.Context(), baby.FamilyID, baby.ID, eventTypeGrowthMeasurement, time.Time{}, now.AddDate(0, 0, 1), growthInsightsEventsLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load growth insights")
		return
	}

	writeJSON(w, http.StatusOK, buildGrowthInsights(events, metric, rangeDays, now, birthStart))
}

func parseGrowthInsightsMetric(raw string) (string, bool) {
	if raw == "" {
		return growthInsightsDefaultMetric, true
	}
	if _, ok := growthInsightsMetrics[raw]; !ok {
		return "", false
	}
	return raw, true
}

func parseGrowthInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return growthInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !growthInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

type growthMeasurement struct {
	id    string
	at    time.Time
	value float64
}

func growthInsightsBirthStart(birthDate string, loc *time.Location, now time.Time) (*time.Time, error) {
	if birthDate == "" {
		return nil, nil
	}

	birthStart, err := time.ParseInLocation(time.DateOnly, birthDate, loc)
	if err != nil {
		return nil, err
	}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if birthStart.After(todayStart) {
		birthStart = todayStart
	}
	return &birthStart, nil
}

func buildGrowthInsights(events []store.Event, metric string, rangeDays int, now time.Time, birthStart *time.Time) growthInsightsResponse {
	cfg := growthInsightsMetrics[metric]

	var all []growthMeasurement
	for _, ev := range events {
		if ev.EventType != eventTypeGrowthMeasurement {
			continue
		}
		value, ok := growthMeasurementValue(ev, metric)
		if !ok {
			continue
		}
		all = append(all, growthMeasurement{
			id:    ev.ID.String(),
			at:    ev.OccurredAt.In(now.Location()),
			value: value,
		})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })

	hasCutoff := rangeDays != growthInsightsAllTimeRangeDays
	var cutoff time.Time
	if hasCutoff {
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cutoff = todayStart.AddDate(0, 0, -rangeDays)
	}

	var filterStart *time.Time
	rangeStartsAtBirth := false
	if hasCutoff {
		filterStart = &cutoff
	}
	if birthStart != nil && (!hasCutoff || !birthStart.Before(cutoff)) {
		filterStart = birthStart
		rangeStartsAtBirth = true
	}

	resp := growthInsightsResponse{
		Metric:      metric,
		MetricLabel: cfg.Label,
		Unit:        cfg.Unit,
		RangeDays:   rangeDays,
		RangeEnd:    now,
	}

	type withChange struct {
		growthMeasurement
		changeLabel string
	}
	inRange := make([]withChange, 0, len(all))
	for i, m := range all {
		changeLabel := "First recorded measurement"
		if i > 0 {
			changeLabel = growthChangeLabel(metric, m.value-all[i-1].value)
		}
		if filterStart != nil && m.at.Before(*filterStart) {
			continue
		}
		inRange = append(inRange, withChange{growthMeasurement: m, changeLabel: changeLabel})
	}

	switch {
	case rangeStartsAtBirth:
		resp.RangeStart = filterStart
	case len(inRange) > 0:
		first := inRange[0].at
		firstDay := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, now.Location())
		resp.RangeStart = &firstDay
	case hasCutoff:
		resp.RangeStart = &cutoff
	}
	resp.RangeLabel = growthInsightsRangeLabel(resp.RangeStart, now)

	resp.HasAnyData = len(inRange) > 0
	if !resp.HasAnyData {
		return resp
	}

	points := make([]growthInsightPointResponse, len(inRange))
	for i, m := range inRange {
		points[i] = growthInsightPointResponse{
			ID:          m.id,
			OccurredAt:  m.at,
			LocalDate:   m.at.Format(time.DateOnly),
			Label:       growthInsightsShortLabel(m.at),
			ShowLabel:   growthInsightShowLabel(i, len(inRange)),
			FullLabel:   m.at.Format("Monday, January 2"),
			Value:       m.value,
			ValueLabel:  growthValueLabel(metric, m.value),
			ChangeLabel: m.changeLabel,
		}
	}
	resp.Points = points

	last := inRange[len(inRange)-1]
	agg := growthInsightAggregateResponse{
		Count:            len(inRange),
		LatestValueLabel: growthValueLabel(metric, last.value),
	}

	var observations []string
	if len(inRange) > 1 {
		first := inRange[0]
		gaps := make([]int, 0, len(inRange)-1)
		for i := 1; i < len(inRange); i++ {
			gaps = append(gaps, int(math.Round(inRange[i].at.Sub(inRange[i-1].at).Hours()/24)))
		}
		var gapSum int
		maxGap := gaps[0]
		for _, g := range gaps {
			gapSum += g
			if g > maxGap {
				maxGap = g
			}
		}
		avgGap := float64(gapSum) / float64(len(gaps))

		agg.AverageIntervalDaysLabel = fmt.Sprintf("%.0f days", avgGap)
		agg.AverageIntervalCaption = "Avg. days between measurements"

		changeOverall := last.value - first.value
		agg.ChangeOverallLabel = growthChangeLabel(metric, changeOverall)
		agg.ChangeOverallCaption = fmt.Sprintf("Change across %s", resp.RangeLabel)

		switch {
		case changeOverall > 0:
			observations = append(observations, fmt.Sprintf("%s increased by %s across the selected range.", cfg.Label, growthAbsChangeLabel(metric, changeOverall)))
		case changeOverall < 0:
			observations = append(observations, fmt.Sprintf("%s decreased by %s across the selected range.", cfg.Label, growthAbsChangeLabel(metric, changeOverall)))
		default:
			observations = append(observations, fmt.Sprintf("%s did not change across the selected range.", cfg.Label))
		}
		observations = append(observations, fmt.Sprintf("Longest gap between measurements: %d days.", maxGap))
	} else {
		agg.AverageIntervalDaysLabel = "—"
		agg.AverageIntervalCaption = "Needs more than one recorded measurement"
		agg.ChangeOverallLabel = "—"
		agg.ChangeOverallCaption = "Needs more than one recorded measurement"
	}
	observations = append(observations, fmt.Sprintf("Last recorded %s: %s on %s.", strings.ToLower(cfg.Label), growthValueLabel(metric, last.value), growthInsightsShortLabel(last.at)))

	resp.Aggregate = agg
	resp.Observations = observations
	return resp
}

func growthInsightsRangeLabel(rangeStart *time.Time, now time.Time) string {
	if rangeStart == nil {
		return "All time"
	}
	return fmt.Sprintf("%s – %s", growthInsightsShortLabel(*rangeStart), growthInsightsShortLabel(now))
}

func growthMeasurementValue(ev store.Event, metric string) (float64, bool) {
	switch metric {
	case "weight":
		grams, ok := attributeOptionalInt(ev.Attributes, "weight_grams")
		return float64(grams), ok
	case "length":
		return attributeFloat(ev.Attributes, "length_cm")
	case "head_circumference":
		return attributeFloat(ev.Attributes, "head_circumference_cm")
	default:
		return 0, false
	}
}

func growthValueLabel(metric string, value float64) string {
	if metric == "weight" {
		return fmt.Sprintf("%.3f kg", value/1000)
	}
	return fmt.Sprintf("%s cm", formatGrowthDecimal(roundToOneDecimal(value)))
}

// growthChangeLabel signs and formats a difference between two measurements.
// cm-based metrics are rounded to one decimal place first: their stored
// values arrive as float64 (from a "numeric" column), and subtracting two
// such values can leave binary floating-point noise (e.g. 2.1999999999999997
// instead of 2.2) that formatGrowthDecimal would otherwise print verbatim.
func growthChangeLabel(metric string, diff float64) string {
	sign := ""
	if diff > 0 {
		sign = "+"
	}
	if metric == "weight" {
		return fmt.Sprintf("%s%.3f kg", sign, diff/1000)
	}
	return fmt.Sprintf("%s%s cm", sign, formatGrowthDecimal(roundToOneDecimal(diff)))
}

func growthAbsChangeLabel(metric string, diff float64) string {
	if diff < 0 {
		diff = -diff
	}
	if metric == "weight" {
		return fmt.Sprintf("%.3f kg", diff/1000)
	}
	return fmt.Sprintf("%s cm", formatGrowthDecimal(roundToOneDecimal(diff)))
}

func roundToOneDecimal(value float64) float64 {
	return math.Round(value*10) / 10
}

// formatGrowthDecimal renders a cm value with its shortest round-tripping
// representation (e.g. "58.3", "2.2") rather than a fixed precision, so a
// whole-number head circumference doesn't grow a spurious ".0".
func formatGrowthDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func growthInsightsShortLabel(t time.Time) string {
	return t.Format("Jan 2")
}

// growthInsightShowLabel keeps roughly six labels visible under the chart
// regardless of how many measurements fall in the range — dense enough to
// orient by, sparse enough that labels don't collide — always including the
// most recent point.
func growthInsightShowLabel(index, total int) bool {
	if total <= 1 {
		return true
	}
	labelEvery := 1
	switch {
	case total <= 6:
		labelEvery = 1
	case total <= 14:
		labelEvery = 2
	default:
		labelEvery = int(math.Ceil(float64(total) / 6))
	}
	return index%labelEvery == 0 || index == total-1
}
