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

// nappyInsightsDefaultRangeDays matches the Insights page's default
// selection — kept identical to Sleep's so the shared range pill row means
// the same thing in either category.
const nappyInsightsDefaultRangeDays = 30

var nappyInsightsAllowedRangeDays = map[int]bool{7: true, 30: true, 90: true}

type nappyInsightsResponse struct {
	RangeDays          int                           `json:"range_days"`
	RangeLabel         string                        `json:"range_label"`
	RangeStartsAtBirth bool                          `json:"range_starts_at_birth"`
	Days               []nappyInsightDayResponse     `json:"days"`
	Aggregate          nappyInsightAggregateResponse `json:"aggregate"`
	Observations       []string                      `json:"observations"`
}

type nappyInsightDayResponse struct {
	LocalDate  string                      `json:"local_date"`
	Label      string                      `json:"label"`
	ShowLabel  bool                        `json:"show_label"`
	FullLabel  string                      `json:"full_label"`
	HasData    bool                        `json:"has_data"`
	TotalCount int                         `json:"total_count"`
	WeeCount   int                         `json:"wee_count"`
	PooCount   int                         `json:"poo_count"`
	MixedCount int                         `json:"mixed_count"`
	Events     []nappyInsightEventResponse `json:"events"`

	blowoutCount int
	largeCount   int
}

type nappyInsightEventResponse struct {
	Kind      string `json:"kind"`           // "wee", "poo", or "mixed"
	Size      string `json:"size,omitempty"` // "smear", "small", "medium", "large", or "blowout"; empty for wee-only events
	TimeLabel string `json:"time_label"`
}

type nappyInsightAggregateResponse struct {
	HasAnyData         bool   `json:"has_any_data"`
	RecordedDays       int    `json:"recorded_days"`
	TotalCount         int    `json:"total_count"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	HasAverageGap      bool   `json:"has_average_gap"`
	AverageGapLabel    string `json:"average_gap_label,omitempty"`
	AverageGapCaption  string `json:"average_gap_caption,omitempty"`
	WeePercent         *int   `json:"wee_percent,omitempty"`
	PooPercent         *int   `json:"poo_percent,omitempty"`
	MixedPercent       *int   `json:"mixed_percent,omitempty"`
	BlowoutCount       int    `json:"blowout_count"`
	LargeCount         int    `json:"large_count"`
}

// GetNappyInsights returns a calendar view of recorded nappy changes — one
// entry per day over the requested range (7/30/90 completed days before
// today, mirroring Sleep Insights' own window so the shared range pills mean
// the same thing), plus range-level aggregates and factual observations. All
// display strings are pre-formatted here so the frontend only has to lay the
// data out, not calculate it.
func (h *Handlers) GetNappyInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	rangeDays, ok := parseNappyInsightsRangeDays(r.URL.Query().Get("range"))
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

	events, err := h.Store.ListEventsByType(r.Context(), baby.FamilyID, baby.ID, eventTypeNappy, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("list nappy insights events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load nappy insights")
		return
	}

	response := buildNappyInsights(events, rangeDays, rangeStart, rangeEnd)
	response.RangeStartsAtBirth = rangeStartsAtBirth
	writeJSON(w, http.StatusOK, response)
}

func parseNappyInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return nappyInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !nappyInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

func buildNappyInsights(events []store.Event, rangeDays int, rangeStart, rangeEnd time.Time) nappyInsightsResponse {
	sorted := sortedAnalyticsEvents(events)

	visibleDays := sleepInsightsDayCount(rangeStart, rangeEnd)
	days := make([]nappyInsightDayResponse, 0, visibleDays)
	var recordedDays, totalCountSum, weeSum, pooSum, mixedSum, blowoutSum, largeSum int

	for i := 0; i < visibleDays; i++ {
		dayStart := rangeStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		day := buildNappyInsightDay(sorted, dayStart, dayEnd, i, visibleDays)
		if day.HasData {
			recordedDays++
			totalCountSum += day.TotalCount
			weeSum += day.WeeCount
			pooSum += day.PooCount
			mixedSum += day.MixedCount
			blowoutSum += day.blowoutCount
			largeSum += day.largeCount
		}
		days = append(days, day)
	}

	aggregate, observations := buildNappyInsightAggregate(sorted, recordedDays, totalCountSum, weeSum, pooSum, mixedSum, blowoutSum, largeSum)
	return nappyInsightsResponse{
		RangeDays:    rangeDays,
		RangeLabel:   sleepInsightsRangeLabel(rangeStart, rangeEnd),
		Days:         days,
		Aggregate:    aggregate,
		Observations: observations,
	}
}

func buildNappyInsightDay(events []store.Event, dayStart, dayEnd time.Time, index, rangeDays int) nappyInsightDayResponse {
	day := nappyInsightDayResponse{
		LocalDate: dayStart.Format(time.DateOnly),
		Label:     sleepInsightDayLabel(dayStart, rangeDays),
		ShowLabel: sleepInsightShowLabel(index, rangeDays),
		FullLabel: sleepInsightFullLabel(dayStart),
	}

	var nappyEvents []nappyInsightEventResponse
	for _, ev := range events {
		if ev.EventType != eventTypeNappy {
			continue
		}
		occurredAt := ev.OccurredAt.In(dayStart.Location())
		if occurredAt.Before(dayStart) || !occurredAt.Before(dayEnd) {
			continue
		}

		kind := nappyInsightKind(ev.Attributes)
		size := nappyInsightSize(ev.Attributes)
		nappyEvents = append(nappyEvents, nappyInsightEventResponse{
			Kind:      kind,
			Size:      size,
			TimeLabel: occurredAt.Format("3:04 PM"),
		})
		day.TotalCount++
		switch kind {
		case "poo":
			day.PooCount++
		case "mixed":
			day.MixedCount++
		default:
			day.WeeCount++
		}
		switch size {
		case string(PooSizeBlowout):
			day.blowoutCount++
		case string(PooSizeLarge):
			day.largeCount++
		}
	}
	day.HasData = day.TotalCount > 0
	day.Events = nappyEvents

	return day
}

// nappyInsightSize maps the app's stored poo_size attribute through to the
// Insights response, validating it against the known PooSize enum so a
// missing or corrupt attribute (e.g. a wee-only event, which never sets
// poo_size) surfaces as "" rather than an arbitrary string.
func nappyInsightSize(attributes map[string]any) string {
	size, _ := attributes["poo_size"].(string)
	if !PooSize(size).Valid() {
		return ""
	}
	return size
}

// nappyInsightKind maps the app's stored nappy kind (wet/poo/both) to the
// Insights display vocabulary (wee/poo/mixed), matching the Claude Design
// handoff's naming exactly while keeping backend-api's own event storage
// unchanged.
func nappyInsightKind(attributes map[string]any) string {
	kind, _ := attributes["kind"].(string)
	switch NappyKind(kind) {
	case NappyKindPoo:
		return "poo"
	case NappyKindBoth:
		return "mixed"
	default:
		return "wee"
	}
}

func buildNappyInsightAggregate(sortedEvents []store.Event, recordedDays, totalCountSum, weeSum, pooSum, mixedSum, blowoutSum, largeSum int) (nappyInsightAggregateResponse, []string) {
	aggregate := nappyInsightAggregateResponse{
		HasAnyData:   totalCountSum > 0,
		RecordedDays: recordedDays,
		TotalCount:   totalCountSum,
		BlowoutCount: blowoutSum,
		LargeCount:   largeSum,
	}
	if !aggregate.HasAnyData {
		return aggregate, nil
	}

	var observations []string

	aggregate.AveragePerDayLabel = strconv.FormatFloat(float64(totalCountSum)/float64(recordedDays), 'f', 1, 64)

	var nappyEvents []store.Event
	for _, ev := range sortedEvents {
		if ev.EventType == eventTypeNappy {
			nappyEvents = append(nappyEvents, ev)
		}
	}
	if len(nappyEvents) >= 2 {
		var gapMinutesSum float64
		for i := 1; i < len(nappyEvents); i++ {
			gapMinutesSum += nappyEvents[i].OccurredAt.Sub(nappyEvents[i-1].OccurredAt).Minutes()
		}
		avgGapMinutes := int(math.Round(gapMinutesSum / float64(len(nappyEvents)-1)))
		aggregate.HasAverageGap = true
		aggregate.AverageGapLabel = formatCompactDurationMinutes(avgGapMinutes)
		aggregate.AverageGapCaption = "Avg. time between changes"
		observations = append(observations, fmt.Sprintf("Average recorded time between changes: %s.", aggregate.AverageGapLabel))
	} else {
		aggregate.AverageGapLabel = "Not yet available"
		aggregate.AverageGapCaption = "Needs more recorded nappy changes"
	}

	weePercent, pooPercent, mixedPercent := nappyInsightPercents(weeSum, pooSum, mixedSum)
	aggregate.WeePercent = &weePercent
	aggregate.PooPercent = &pooPercent
	aggregate.MixedPercent = &mixedPercent

	return aggregate, observations
}

func nappyInsightPercents(wee, poo, mixed int) (weePercent, pooPercent, mixedPercent int) {
	counts := [3]int{wee, poo, mixed}
	total := wee + poo + mixed
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
