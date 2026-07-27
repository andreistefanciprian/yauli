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
	IsToday        bool                         `json:"is_today"`
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
	rangeStart, todayStart := sleepInsightsWindow(rangeDays, loc, now)

	events, err := h.Store.ListAllEvents(r.Context(), baby.FamilyID, baby.ID, rangeStart, now, reportEventsLimit*rangeDays)
	if err != nil {
		log.Printf("list sleep insights events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load sleep insights")
		return
	}

	writeJSON(w, http.StatusOK, buildSleepInsights(events, rangeDays, rangeStart, todayStart, now))
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

// sleepInsightsWindow resolves the local-calendar-day window for a range
// selection: rangeDays-1 full days before today, plus today itself (partial,
// up to now).
func sleepInsightsWindow(rangeDays int, loc *time.Location, now time.Time) (rangeStart, todayStart time.Time) {
	todayStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	rangeStart = todayStart.AddDate(0, 0, -(rangeDays - 1))
	return rangeStart, todayStart
}

func buildSleepInsights(events []store.Event, rangeDays int, rangeStart, todayStart, now time.Time) sleepInsightsResponse {
	sorted := sortedAnalyticsEvents(events)

	days := make([]sleepInsightDayResponse, 0, rangeDays)
	var totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum, daysWithData int

	for i := 0; i < rangeDays; i++ {
		dayStart := rangeStart.AddDate(0, 0, i)
		isToday := dayStart.Equal(todayStart)
		dayEnd := dayStart.AddDate(0, 0, 1)
		if isToday {
			dayEnd = now
		}

		day := buildSleepInsightDay(sorted, dayStart, dayEnd, i, rangeDays, isToday, now)
		if day.HasData {
			daysWithData++
			totalMinutesSum += day.TotalMinutes
			completedSum += day.CompletedCount
			napMinutesSum += day.NapMinutes
			nightMinutesSum += day.NightMinutes
		}
		days = append(days, day)
	}

	aggregate, observations := buildSleepInsightAggregate(sorted, daysWithData, totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum)

	return sleepInsightsResponse{
		RangeDays:    rangeDays,
		RangeLabel:   fmt.Sprintf("%s – %s", rangeStart.Format("Jan 2"), todayStart.Format("Jan 2")),
		Days:         days,
		Aggregate:    aggregate,
		Observations: observations,
	}
}

func buildSleepInsightDay(events []store.Event, dayStart, dayEnd time.Time, index, rangeDays int, isToday bool, now time.Time) sleepInsightDayResponse {
	day := sleepInsightDayResponse{
		LocalDate: dayStart.Format(time.DateOnly),
		Label:     sleepInsightDayLabel(dayStart, rangeDays),
		ShowLabel: sleepInsightShowLabel(index, rangeDays),
		FullLabel: sleepInsightFullLabel(dayStart, isToday),
		IsToday:   isToday,
	}

	var periods []sleepInsightPeriodResponse
	for _, ev := range events {
		if ev.EventType != eventTypeSleep {
			continue
		}
		if ev.OccurredAt.Before(dayStart) || !ev.OccurredAt.Before(dayEnd) {
			continue
		}
		periods = append(periods, sleepInsightPeriodFromEvent(ev, dayStart, now))
	}

	day.HasData = len(periods) > 0
	for _, period := range periods {
		day.TotalMinutes += period.DurationMinutes
		if !period.Ongoing {
			day.CompletedCount++
		}
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
		if isToday {
			day.TotalLabel += " so far"
		}
		day.LongestLabel = formatCompactDurationMinutes(day.LongestMinutes)
		day.NapNightLabel = fmt.Sprintf("%s · %s", formatCompactDurationMinutes(day.NapMinutes), formatCompactDurationMinutes(day.NightMinutes))
	}

	return day
}

func sleepInsightPeriodFromEvent(ev store.Event, dayStart time.Time, now time.Time) sleepInsightPeriodResponse {
	sleepType, _ := ev.Attributes["type"].(string)
	if sleepType != string(SleepTypeNap) && sleepType != string(SleepTypeNight) {
		sleepType = string(SleepTypeNight)
	}

	startMinutes := int(ev.OccurredAt.Sub(dayStart).Minutes())
	durationMinutes, ok := attributeOptionalInt(ev.Attributes, "duration_minutes")
	ongoing := !ok
	if ongoing {
		durationMinutes = int(now.Sub(ev.OccurredAt).Minutes())
		if durationMinutes < 0 {
			durationMinutes = 0
		}
	}
	endMinutes := startMinutes + durationMinutes

	return sleepInsightPeriodResponse{
		Type:            sleepType,
		StartMinutes:    startMinutes,
		EndMinutes:      endMinutes,
		DurationMinutes: durationMinutes,
		Ongoing:         ongoing,
		TimeRangeLabel:  sleepPeriodTimeRangeLabel(startMinutes, endMinutes, ongoing),
		DurationLabel:   sleepPeriodDurationLabel(durationMinutes, ongoing),
	}
}

func buildSleepInsightAggregate(sortedEvents []store.Event, daysWithData, totalMinutesSum, completedSum, napMinutesSum, nightMinutesSum int) (sleepInsightAggregateResponse, []string) {
	aggregate := sleepInsightAggregateResponse{HasAnyData: daysWithData > 0}
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

	avgTotal := int(math.Round(float64(totalMinutesSum) / float64(daysWithData)))
	aggregate.AverageTotalLabel = formatCompactDurationMinutes(avgTotal)
	aggregate.AverageCompletedLabel = strconv.FormatFloat(float64(completedSum)/float64(daysWithData), 'f', 1, 64)

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

func sleepInsightFullLabel(dayStart time.Time, isToday bool) string {
	label := dayStart.Format("Monday, January 2")
	if isToday {
		label += " · today"
	}
	return label
}

func sleepClockLabel(minutes int) string {
	minutes = ((minutes % 1440) + 1440) % 1440
	hour := minutes / 60
	minute := minutes % 60
	meridiem := "AM"
	if hour >= 12 {
		meridiem = "PM"
	}
	hour12 := hour % 12
	if hour12 == 0 {
		hour12 = 12
	}
	return fmt.Sprintf("%d:%02d %s", hour12, minute, meridiem)
}

func sleepPeriodTimeRangeLabel(startMinutes, endMinutes int, ongoing bool) string {
	end := "ongoing"
	if !ongoing {
		end = sleepClockLabel(endMinutes)
	}
	return fmt.Sprintf("%s – %s", sleepClockLabel(startMinutes), end)
}

func sleepPeriodDurationLabel(durationMinutes int, ongoing bool) string {
	label := formatCompactDurationMinutes(durationMinutes)
	if ongoing {
		return label + " so far"
	}
	return label
}
