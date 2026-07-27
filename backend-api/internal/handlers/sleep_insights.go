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

// sleepInsightsDefaultRangeDays matches the Insights page's default
// selection (the middle of the three range pills).
const sleepInsightsDefaultRangeDays = 30

var sleepInsightsAllowedRangeDays = map[int]bool{7: true, 30: true, 90: true}

type sleepInsightsResponse struct {
	RangeDays    int                           `json:"range_days"`
	RangeLabel   string                        `json:"range_label"`
	Days         []sleepInsightDayResponse     `json:"days"`
	Aggregate    sleepInsightAggregateResponse `json:"aggregate"`
	Observations []string                      `json:"observations"`
}

type sleepInsightDayResponse struct {
	LocalDate      string                       `json:"local_date"`
	Label          string                       `json:"label"`
	ShowLabel      bool                         `json:"show_label"`
	FullLabel      string                       `json:"full_label"`
	HasData        bool                         `json:"has_data"`
	TotalMinutes   int                          `json:"total_minutes"`
	TotalLabel     string                       `json:"total_label,omitempty"`
	CompletedCount int                          `json:"completed_count"`
	LongestMinutes int                          `json:"longest_minutes"`
	LongestLabel   string                       `json:"longest_label,omitempty"`
	NapMinutes     int                          `json:"nap_minutes"`
	NightMinutes   int                          `json:"night_minutes"`
	NapNightLabel  string                       `json:"nap_night_label,omitempty"`
	Periods        []sleepInsightPeriodResponse `json:"periods"`
}

type sleepInsightPeriodResponse struct {
	Type            string `json:"type"` // "nap" or "night"
	StartMinutes    int    `json:"start_minutes"`
	EndMinutes      int    `json:"end_minutes"`
	DurationMinutes int    `json:"duration_minutes"`
	Ongoing         bool   `json:"ongoing"`
	TimeRangeLabel  string `json:"time_range_label"`
	DurationLabel   string `json:"duration_label"`
}

type sleepInsightAggregateResponse struct {
	HasAnyData               bool   `json:"has_any_data"`
	AverageTotalLabel        string `json:"average_total_label,omitempty"`
	AverageCompletedLabel    string `json:"average_completed_label,omitempty"`
	LongestOverallLabel      string `json:"longest_overall_label,omitempty"`
	HasWakeWindow            bool   `json:"has_wake_window"`
	AverageWakeWindowLabel   string `json:"average_wake_window_label,omitempty"`
	AverageWakeWindowCaption string `json:"average_wake_window_caption,omitempty"`
	NapPercent               *int   `json:"nap_percent,omitempty"`
	NightPercent             *int   `json:"night_percent,omitempty"`
}

// GetSleepInsights returns a calendar view of recorded sleep — one entry per
// day over the requested range (7/30/90 days ending today), plus range-level
// aggregates and factual observations. All display strings are pre-formatted
// here so the frontend only has to lay the data out, not calculate it.
func (h *Handlers) GetSleepInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	rangeDays, ok := parseSleepInsightsRangeDays(r.URL.Query().Get("range"))
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

	// Include the preceding local day so a completed overnight sleep can
	// contribute its post-midnight portion to the first requested day. Sleeps
	// are ordinary baby intervals rather than multi-day records; stale ongoing
	// events are displayed but never contribute duration.
	queryStart := rangeStart.AddDate(0, 0, -1)
	events, err := h.Store.ListEventsByType(r.Context(), baby.FamilyID, baby.ID, eventTypeSleep, queryStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("list sleep insights events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load sleep insights")
		return
	}

	writeJSON(w, http.StatusOK, buildSleepInsights(events, rangeDays, rangeStart, rangeEnd))
}

func parseSleepInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return sleepInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !sleepInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

// sleepInsightsWindow resolves a range of completed local calendar days.
// Today is deliberately excluded because it is still partial.
func sleepInsightsWindow(rangeDays int, loc *time.Location, now time.Time) (rangeStart, rangeEnd time.Time) {
	rangeEnd = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	rangeStart = rangeEnd.AddDate(0, 0, -rangeDays)
	return rangeStart, rangeEnd
}

func buildSleepInsights(events []store.Event, rangeDays int, rangeStart, rangeEnd time.Time) sleepInsightsResponse {
	sorted := sortedAnalyticsEvents(events)

	days := make([]sleepInsightDayResponse, 0, rangeDays)
	var totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum int

	for i := 0; i < rangeDays; i++ {
		dayStart := rangeStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		day := buildSleepInsightDay(sorted, dayStart, dayEnd, i, rangeDays)
		if day.HasData {
			totalMinutesSum += day.TotalMinutes
			completedSum += day.CompletedCount
			napMinutesSum += day.NapMinutes
			nightMinutesSum += day.NightMinutes
		}
		days = append(days, day)
	}

	analyticsEvents := eventsStartingInWindow(sorted, rangeStart, rangeEnd)
	aggregate, observations := buildSleepInsightAggregate(analyticsEvents, rangeDays, totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum)
	lastDay := rangeEnd.AddDate(0, 0, -1)

	return sleepInsightsResponse{
		RangeDays:    rangeDays,
		RangeLabel:   fmt.Sprintf("%s – %s", rangeStart.Format("Jan 2"), lastDay.Format("Jan 2")),
		Days:         days,
		Aggregate:    aggregate,
		Observations: observations,
	}
}

func eventsStartingInWindow(events []store.Event, rangeStart, rangeEnd time.Time) []store.Event {
	filtered := make([]store.Event, 0, len(events))
	for _, event := range events {
		if event.OccurredAt.Before(rangeStart) || !event.OccurredAt.Before(rangeEnd) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func buildSleepInsightDay(events []store.Event, dayStart, dayEnd time.Time, index, rangeDays int) sleepInsightDayResponse {
	day := sleepInsightDayResponse{
		LocalDate: dayStart.Format(time.DateOnly),
		Label:     sleepInsightDayLabel(dayStart, rangeDays),
		ShowLabel: sleepInsightShowLabel(index, rangeDays),
		FullLabel: sleepInsightFullLabel(dayStart),
	}

	var periods []sleepInsightPeriodResponse
	for _, ev := range events {
		if ev.EventType != eventTypeSleep {
			continue
		}
		period, overlaps := sleepInsightPeriodForDay(ev, dayStart, dayEnd)
		if !overlaps {
			continue
		}
		periods = append(periods, period)
	}

	day.HasData = len(periods) > 0
	for _, period := range periods {
		if period.Ongoing {
			continue
		}
		day.TotalMinutes += period.DurationMinutes
		day.CompletedCount++
		if period.DurationMinutes > day.LongestMinutes {
			day.LongestMinutes = period.DurationMinutes
		}
		if period.Type == string(SleepTypeNap) {
			day.NapMinutes += period.DurationMinutes
		} else {
			day.NightMinutes += period.DurationMinutes
		}
	}
	day.Periods = periods

	if day.HasData {
		day.TotalLabel = formatCompactDurationMinutes(day.TotalMinutes)
		if day.CompletedCount > 0 {
			day.LongestLabel = formatCompactDurationMinutes(day.LongestMinutes)
			day.NapNightLabel = fmt.Sprintf("%s · %s", formatCompactDurationMinutes(day.NapMinutes), formatCompactDurationMinutes(day.NightMinutes))
		}
	}

	return day
}

func sleepInsightPeriodForDay(ev store.Event, dayStart, dayEnd time.Time) (sleepInsightPeriodResponse, bool) {
	sleepType, _ := ev.Attributes["type"].(string)
	if sleepType != string(SleepTypeNap) && sleepType != string(SleepTypeNight) {
		sleepType = string(SleepTypeNight)
	}

	durationMinutes, ok := attributeOptionalInt(ev.Attributes, "duration_minutes")
	if !ok {
		if ev.OccurredAt.Before(dayStart) || !ev.OccurredAt.Before(dayEnd) {
			return sleepInsightPeriodResponse{}, false
		}
		startMinutes := int(ev.OccurredAt.Sub(dayStart).Minutes())
		return sleepInsightPeriodResponse{
			Type:           sleepType,
			StartMinutes:   startMinutes,
			EndMinutes:     startMinutes,
			Ongoing:        true,
			TimeRangeLabel: sleepPeriodTimeRangeLabel(ev.OccurredAt, time.Time{}, true),
			DurationLabel:  "Ongoing",
		}, true
	}

	sleepStart := ev.OccurredAt
	sleepEnd := sleepStart.Add(time.Duration(durationMinutes) * time.Minute)
	overlapStart := sleepStart
	if overlapStart.Before(dayStart) {
		overlapStart = dayStart
	}
	overlapEnd := sleepEnd
	if overlapEnd.After(dayEnd) {
		overlapEnd = dayEnd
	}
	if !overlapEnd.After(overlapStart) {
		return sleepInsightPeriodResponse{}, false
	}

	startMinutes := int(overlapStart.Sub(dayStart).Minutes())
	endMinutes := int(overlapEnd.Sub(dayStart).Minutes())
	overlapMinutes := int(overlapEnd.Sub(overlapStart).Minutes())

	return sleepInsightPeriodResponse{
		Type:            sleepType,
		StartMinutes:    startMinutes,
		EndMinutes:      endMinutes,
		DurationMinutes: overlapMinutes,
		TimeRangeLabel:  sleepPeriodTimeRangeLabel(overlapStart, overlapEnd, false),
		DurationLabel:   sleepPeriodDurationLabel(overlapMinutes),
	}, true
}

func buildSleepInsightAggregate(sortedEvents []store.Event, rangeDays, totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum int) (sleepInsightAggregateResponse, []string) {
	aggregate := sleepInsightAggregateResponse{HasAnyData: completedSum > 0}
	if !aggregate.HasAnyData {
		return aggregate, nil
	}

	var observations []string

	intervals := buildSleepIntervalAnalytics(sortedEvents)
	if intervals.LongestDurationMinutes != nil {
		aggregate.LongestOverallLabel = formatCompactDurationMinutes(*intervals.LongestDurationMinutes)
		observations = append(observations, fmt.Sprintf("Longest recorded sleep in this range: %s.", aggregate.LongestOverallLabel))
	}

	if intervals.WakeWindows.AverageMinutes != nil {
		aggregate.HasWakeWindow = true
		aggregate.AverageWakeWindowLabel = formatCompactDurationMinutes(*intervals.WakeWindows.AverageMinutes)
		aggregate.AverageWakeWindowCaption = "Avg. recorded wake window"
		observations = append(observations, fmt.Sprintf("Average recorded wake window: %s.", aggregate.AverageWakeWindowLabel))
	} else {
		aggregate.AverageWakeWindowLabel = "Not yet available"
		aggregate.AverageWakeWindowCaption = "Needs more recorded sleep periods"
		observations = append(observations, "Not enough recorded sleep yet to calculate an average wake window.")
	}

	avgTotal := int(math.Round(float64(totalMinutesSum) / float64(rangeDays)))
	aggregate.AverageTotalLabel = formatCompactDurationMinutes(avgTotal)
	aggregate.AverageCompletedLabel = strconv.FormatFloat(float64(completedSum)/float64(rangeDays), 'f', 1, 64)

	if sleepTotal := napMinutesSum + nightMinutesSum; sleepTotal > 0 {
		napPercent := int(math.Round(float64(napMinutesSum) / float64(sleepTotal) * 100))
		nightPercent := 100 - napPercent
		aggregate.NapPercent = &napPercent
		aggregate.NightPercent = &nightPercent
		observations = append(observations, fmt.Sprintf("Night sleep accounted for %d%% of recorded sleep in this range.", nightPercent))
	}

	return aggregate, observations
}

func sleepInsightDayLabel(dayStart time.Time, rangeDays int) string {
	if rangeDays <= 7 {
		return dayStart.Format("Mon")
	}
	return dayStart.Format("Jan 2")
}

// sleepInsightShowLabel decides chart label density: every bar for a 7-day
// range, every 5th (+ the last) for 30 days, every 15th (+ the last) for 90 —
// dense enough to orient by, sparse enough that labels don't collide.
func sleepInsightShowLabel(index, rangeDays int) bool {
	switch {
	case rangeDays <= 7:
		return true
	case rangeDays <= 30:
		return index%5 == 0 || index == rangeDays-1
	default:
		return index%15 == 0 || index == rangeDays-1
	}
}

func sleepInsightFullLabel(dayStart time.Time) string {
	return dayStart.Format("Monday, January 2")
}

func sleepPeriodTimeRangeLabel(start, end time.Time, ongoing bool) string {
	endLabel := "ongoing"
	if !ongoing {
		endLabel = end.Format("3:04 PM")
	}
	return fmt.Sprintf("%s – %s", start.Format("3:04 PM"), endLabel)
}

func sleepPeriodDurationLabel(durationMinutes int) string {
	return formatCompactDurationMinutes(durationMinutes)
}
