package handlers

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

// feedInsightsDefaultRangeDays matches the Insights page's default
// selection — kept identical to Sleep's and Nappy's so the shared range pill
// row means the same thing in every category.
const feedInsightsDefaultRangeDays = 30

var feedInsightsAllowedRangeDays = map[int]bool{7: true, 30: true, 90: true}

type feedInsightsResponse struct {
	RangeDays          int                          `json:"range_days"`
	RangeLabel         string                       `json:"range_label"`
	RangeStartsAtBirth bool                         `json:"range_starts_at_birth"`
	Days               []feedInsightDayResponse     `json:"days"`
	Aggregate          feedInsightAggregateResponse `json:"aggregate"`
	Observations       []string                     `json:"observations"`
}

type feedInsightDayResponse struct {
	LocalDate      string                     `json:"local_date"`
	Label          string                     `json:"label"`
	ShowLabel      bool                       `json:"show_label"`
	FullLabel      string                     `json:"full_label"`
	HasData        bool                       `json:"has_data"`
	TotalCount     int                        `json:"total_count"`
	BreastCount    int                        `json:"breast_count"`
	FormulaCount   int                        `json:"formula_count"`
	ExpressedCount int                        `json:"expressed_count"`
	BreastMinutes  int                        `json:"breast_minutes"`
	FormulaMl      int                        `json:"formula_ml"`
	ExpressedMl    int                        `json:"expressed_ml"`
	BottleMl       int                        `json:"bottle_ml"`
	Events         []feedInsightEventResponse `json:"events"`

	breastDurationCount int
}

type feedInsightEventResponse struct {
	Kind        string `json:"kind"` // "breast", "formula", or "expressed"
	TimeLabel   string `json:"time_label"`
	DetailLabel string `json:"detail_label"`
}

type feedInsightAggregateResponse struct {
	HasAnyData                   bool   `json:"has_any_data"`
	RecordedDays                 int    `json:"recorded_days"`
	TotalCount                   int    `json:"total_count"`
	BreastCount                  int    `json:"breast_count"`
	FormulaCount                 int    `json:"formula_count"`
	ExpressedCount               int    `json:"expressed_count"`
	AveragePerDayLabel           string `json:"average_per_day_label,omitempty"`
	HasAverageGap                bool   `json:"has_average_gap"`
	AverageGapLabel              string `json:"average_gap_label,omitempty"`
	AverageGapCaption            string `json:"average_gap_caption,omitempty"`
	BreastTotalMinutes           int    `json:"breast_total_minutes"`
	BreastTotalLabel             string `json:"breast_total_label,omitempty"`
	BreastFeedsWithDurationCount int    `json:"breast_feeds_with_duration_count"`
	BreastDurationBasisLabel     string `json:"breast_duration_basis_label,omitempty"`
	BottleTotalMl                int    `json:"bottle_total_ml"`
	BottleTotalLabel             string `json:"bottle_total_label,omitempty"`
	BreastPercent                *int   `json:"breast_percent,omitempty"`
	FormulaPercent               *int   `json:"formula_percent,omitempty"`
	ExpressedPercent             *int   `json:"expressed_percent,omitempty"`
}

// feedInsightTotals accumulates range-level sums while the per-day loop in
// buildFeedInsights runs, so buildFeedInsightAggregate takes one value.
type feedInsightTotals struct {
	recordedDays        int
	totalCount          int
	breastMinutes       int
	breastDurationCount int
	formulaMl           int
	expressedMl         int
	breastCount         int
	formulaCount        int
	expressedCount      int
}

// GetFeedInsights returns a calendar view of recorded feeds — one entry per
// day over the requested range (7/30/90 completed days before today,
// mirroring Sleep Insights' own window so the shared range pills mean the
// same thing), plus range-level aggregates and factual observations. All
// display strings are pre-formatted here so the frontend only has to lay the
// data out, not calculate it.
func (h *Handlers) GetFeedInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	rangeDays, ok := parseFeedInsightsRangeDays(r.URL.Query().Get("range"))
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

	events, err := h.Store.ListEventsByType(r.Context(), baby.FamilyID, baby.ID, eventTypeFeed, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("list feed insights events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load feed insights")
		return
	}

	response, _ := buildFeedInsights(events, rangeDays, rangeStart, rangeEnd)
	response.RangeStartsAtBirth = rangeStartsAtBirth
	writeJSON(w, http.StatusOK, response)
}

func parseFeedInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return feedInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !feedInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

// buildFeedInsights returns the response plus the range-level totals it was
// built from. The totals aren't part of the serialized response — they're a
// second return value purely so overviewFeedStats can derive Overview-only
// figures (the formula/expressed volume split) without those figures
// growing this endpoint's public JSON surface for a consumer that isn't
// this endpoint. GetFeedInsights itself ignores the second value.
func buildFeedInsights(events []store.Event, rangeDays int, rangeStart, rangeEnd time.Time) (feedInsightsResponse, feedInsightTotals) {
	// Feeds are discrete activities: counts, volume, and duration all belong to
	// the local calendar day on which the feed started. Unlike sleep, a feed
	// that crosses midnight is neither split nor carried into the next day.
	sorted := eventsStartingInWindow(sortedAnalyticsEvents(events), rangeStart, rangeEnd)

	visibleDays := sleepInsightsDayCount(rangeStart, rangeEnd)
	days := make([]feedInsightDayResponse, 0, visibleDays)
	var totals feedInsightTotals

	for i := 0; i < visibleDays; i++ {
		dayStart := rangeStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		day := buildFeedInsightDay(sorted, dayStart, dayEnd, i, visibleDays)
		if day.HasData {
			totals.recordedDays++
			totals.totalCount += day.TotalCount
			totals.breastMinutes += day.BreastMinutes
			totals.breastDurationCount += day.breastDurationCount
			totals.formulaMl += day.FormulaMl
			totals.expressedMl += day.ExpressedMl
			totals.breastCount += day.BreastCount
			totals.formulaCount += day.FormulaCount
			totals.expressedCount += day.ExpressedCount
		}
		days = append(days, day)
	}

	aggregate, observations := buildFeedInsightAggregate(sorted, totals)
	resp := feedInsightsResponse{
		RangeDays:    rangeDays,
		RangeLabel:   sleepInsightsRangeLabel(rangeStart, rangeEnd),
		Days:         days,
		Aggregate:    aggregate,
		Observations: observations,
	}
	return resp, totals
}

func buildFeedInsightDay(events []store.Event, dayStart, dayEnd time.Time, index, rangeDays int) feedInsightDayResponse {
	day := feedInsightDayResponse{
		LocalDate: dayStart.Format(time.DateOnly),
		Label:     sleepInsightDayLabel(dayStart, rangeDays),
		ShowLabel: sleepInsightShowLabel(index, rangeDays),
		FullLabel: sleepInsightFullLabel(dayStart),
	}

	var feedEvents []feedInsightEventResponse
	for _, ev := range events {
		if ev.EventType != eventTypeFeed {
			continue
		}
		occurredAt := ev.OccurredAt.In(dayStart.Location())
		if occurredAt.Before(dayStart) || !occurredAt.Before(dayEnd) {
			continue
		}

		event, ok := buildFeedInsightEvent(ev, occurredAt, &day)
		if !ok {
			continue
		}
		feedEvents = append(feedEvents, event)
		day.TotalCount++
	}
	day.BottleMl = day.FormulaMl + day.ExpressedMl
	day.HasData = day.TotalCount > 0
	day.Events = feedEvents

	return day
}

// buildFeedInsightEvent classifies one feed event, folding its amount into
// the running day totals and returning its display row. ok is false for a
// feed recorded with an unrecognized type, which is excluded from the day
// entirely rather than shown with blank details.
func buildFeedInsightEvent(ev store.Event, occurredAt time.Time, day *feedInsightDayResponse) (feedInsightEventResponse, bool) {
	kind, _ := ev.Attributes["type"].(string)
	timeLabel := occurredAt.Format("3:04 PM")

	switch FeedType(kind) {
	case FeedTypeBreast:
		day.BreastCount++
		minutes, ok := attributeInt(ev.Attributes, "duration_minutes")
		if !ok {
			return feedInsightEventResponse{Kind: string(FeedTypeBreast), TimeLabel: timeLabel, DetailLabel: "Ongoing"}, true
		}
		day.breastDurationCount++
		day.BreastMinutes += minutes
		return feedInsightEventResponse{Kind: string(FeedTypeBreast), TimeLabel: timeLabel, DetailLabel: formatCompactFeedDurationMinutes(minutes)}, true
	case FeedTypeFormula:
		ml, _ := attributeInt(ev.Attributes, "amount_ml")
		day.FormulaCount++
		day.FormulaMl += ml
		return feedInsightEventResponse{Kind: string(FeedTypeFormula), TimeLabel: timeLabel, DetailLabel: fmt.Sprintf("%d ml", ml)}, true
	case FeedTypeExpressed:
		ml, _ := attributeInt(ev.Attributes, "amount_ml")
		day.ExpressedCount++
		day.ExpressedMl += ml
		return feedInsightEventResponse{Kind: string(FeedTypeExpressed), TimeLabel: timeLabel, DetailLabel: fmt.Sprintf("%d ml", ml)}, true
	default:
		return feedInsightEventResponse{}, false
	}
}

func buildFeedInsightAggregate(sortedEvents []store.Event, totals feedInsightTotals) (feedInsightAggregateResponse, []string) {
	aggregate := feedInsightAggregateResponse{
		HasAnyData:                   totals.totalCount > 0,
		RecordedDays:                 totals.recordedDays,
		TotalCount:                   totals.totalCount,
		BreastCount:                  totals.breastCount,
		FormulaCount:                 totals.formulaCount,
		ExpressedCount:               totals.expressedCount,
		BreastTotalMinutes:           totals.breastMinutes,
		BreastFeedsWithDurationCount: totals.breastDurationCount,
		BottleTotalMl:                totals.formulaMl + totals.expressedMl,
	}
	if !aggregate.HasAnyData {
		return aggregate, nil
	}

	var observations []string

	if totals.breastCount > 0 && totals.breastDurationCount == 0 {
		aggregate.BreastTotalLabel = "Not yet available"
	} else {
		aggregate.BreastTotalLabel = formatCompactFeedDurationMinutes(aggregate.BreastTotalMinutes)
	}
	if totals.breastCount == 0 {
		aggregate.BreastDurationBasisLabel = "No breast feeds recorded"
	} else {
		aggregate.BreastDurationBasisLabel = insightsDurationBasisLabel(totals.breastDurationCount, totals.breastCount, "breast feed", "breast feeds")
	}
	aggregate.BottleTotalLabel = fmt.Sprintf("%d ml", aggregate.BottleTotalMl)
	aggregate.AveragePerDayLabel = strconv.FormatFloat(float64(totals.totalCount)/float64(totals.recordedDays), 'f', 1, 64)

	var feedEvents []store.Event
	for _, ev := range sortedEvents {
		if ev.EventType == eventTypeFeed {
			feedEvents = append(feedEvents, ev)
		}
	}
	if len(feedEvents) >= 2 {
		var gapMinutesSum float64
		for i := 1; i < len(feedEvents); i++ {
			gapMinutesSum += feedEvents[i].OccurredAt.Sub(feedEvents[i-1].OccurredAt).Minutes()
		}
		avgGapMinutes := int(math.Round(gapMinutesSum / float64(len(feedEvents)-1)))
		aggregate.HasAverageGap = true
		aggregate.AverageGapLabel = formatCompactFeedDurationMinutes(avgGapMinutes)
		aggregate.AverageGapCaption = "Avg. time between feeds"
		observations = append(observations, fmt.Sprintf("Average recorded time between feeds: %s.", aggregate.AverageGapLabel))
	} else {
		aggregate.AverageGapLabel = "Not yet available"
		aggregate.AverageGapCaption = "Needs more recorded feeds"
	}

	breastPercent, formulaPercent, expressedPercent := feedInsightPercents(totals.breastCount, totals.formulaCount, totals.expressedCount)
	aggregate.BreastPercent = &breastPercent
	aggregate.FormulaPercent = &formulaPercent
	aggregate.ExpressedPercent = &expressedPercent

	return aggregate, observations
}

// formatCompactFeedDurationMinutes matches formatCompactDurationMinutes but
// with single-letter units (3h 20m) — the Insights feed card's own style,
// same as Sleep's, distinct from the "hr"/"min" units used in daily report
// summaries and Nappy insights.
func formatCompactFeedDurationMinutes(minutes int) string {
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", remainingMinutes)
	}
	if remainingMinutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
}

// feedVolumeLabel formats a volume in ml, switching to liters (one decimal
// place) once it reaches 1000 ml. Only used for range totals — a single
// feed's volume never approaches a liter, but a 30/90-day total routinely
// does.
func feedVolumeLabel(ml int) string {
	if ml >= 1000 {
		return fmt.Sprintf("%.1f L", float64(ml)/1000)
	}
	return fmt.Sprintf("%d ml", ml)
}

// feedInsightPercents allocates whole-percent shares of feed count across
// breast/formula/expressed using largest-remainder rounding, mirroring
// nappyInsightPercents so the two breakdown bars round the same way.
func feedInsightPercents(breast, formula, expressed int) (breastPercent, formulaPercent, expressedPercent int) {
	counts := [3]int{breast, formula, expressed}
	total := breast + formula + expressed
	if total <= 0 {
		return 0, 0, 0
	}

	var percents, remainders [3]int
	allocated := 0
	for i, count := range counts {
		scaled := count * 100
		percents[i] = scaled / total
		remainders[i] = scaled % total
		allocated += percents[i]
	}

	for allocated < 100 {
		largestRemainder := 0
		for i := 1; i < len(remainders); i++ {
			if remainders[i] > remainders[largestRemainder] {
				largestRemainder = i
			}
		}
		percents[largestRemainder]++
		remainders[largestRemainder] = -1
		allocated++
	}

	return percents[0], percents[1], percents[2]
}
