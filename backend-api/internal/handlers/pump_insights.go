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

// pumpInsightsDefaultRangeDays matches the Insights page's default
// selection — kept identical to Sleep's, Nappy's, and Feed's so the shared
// range pill row means the same thing in every category.
const pumpInsightsDefaultRangeDays = 30

var pumpInsightsAllowedRangeDays = map[int]bool{7: true, 30: true, 90: true}

type pumpInsightsResponse struct {
	RangeDays          int                          `json:"range_days"`
	RangeLabel         string                       `json:"range_label"`
	RangeStartsAtBirth bool                         `json:"range_starts_at_birth"`
	Days               []pumpInsightDayResponse     `json:"days"`
	Aggregate          pumpInsightAggregateResponse `json:"aggregate"`
	Observations       []string                     `json:"observations"`
}

type pumpInsightDayResponse struct {
	LocalDate     string                     `json:"local_date"`
	Label         string                     `json:"label"`
	ShowLabel     bool                       `json:"show_label"`
	FullLabel     string                     `json:"full_label"`
	HasData       bool                       `json:"has_data"`
	SessionCount  int                        `json:"session_count"`
	TotalMl       int                        `json:"total_ml"`
	TotalMinutes  int                        `json:"total_minutes"`
	DurationLabel string                     `json:"duration_label,omitempty"`
	Events        []pumpInsightEventResponse `json:"events"`

	durationCount int
}

type pumpInsightEventResponse struct {
	TimeLabel     string `json:"time_label"`
	VolumeLabel   string `json:"volume_label"`
	DurationLabel string `json:"duration_label,omitempty"`
}

type pumpInsightAggregateResponse struct {
	HasAnyData                  bool   `json:"has_any_data"`
	RecordedDays                int    `json:"recorded_days"`
	SessionCount                int    `json:"session_count"`
	SessionsWithDurationCount   int    `json:"sessions_with_duration_count"`
	TotalMl                     int    `json:"total_ml"`
	TotalMlLabel                string `json:"total_ml_label,omitempty"`
	TotalMinutes                int    `json:"total_minutes"`
	TotalDurationLabel          string `json:"total_duration_label,omitempty"`
	AveragePerDayLabel          string `json:"average_per_day_label,omitempty"`
	HasAverageGap               bool   `json:"has_average_gap"`
	AverageGapLabel             string `json:"average_gap_label,omitempty"`
	AverageGapCaption           string `json:"average_gap_caption,omitempty"`
	AverageSessionMlLabel       string `json:"average_session_ml_label,omitempty"`
	AverageSessionDurationLabel string `json:"average_session_duration_label,omitempty"`
}

// pumpInsightTotals accumulates range-level sums while the per-day loop in
// buildPumpInsights runs, so buildPumpInsightAggregate takes one value.
type pumpInsightTotals struct {
	recordedDays  int
	sessionCount  int
	totalMl       int
	totalMinutes  int
	durationCount int
}

// GetPumpInsights returns a calendar view of recorded pumping sessions — one
// entry per day over the requested range (7/30/90 completed days before
// today, mirroring Sleep Insights' own window so the shared range pills mean
// the same thing), plus range-level aggregates and factual observations. All
// display strings are pre-formatted here so the frontend only has to lay the
// data out, not calculate it.
func (h *Handlers) GetPumpInsights(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}

	rangeDays, ok := parsePumpInsightsRangeDays(r.URL.Query().Get("range"))
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

	events, err := h.Store.ListEventsByType(r.Context(), baby.FamilyID, baby.ID, eventTypePump, rangeStart, rangeEnd, reportEventsLimit*(rangeDays+1))
	if err != nil {
		log.Printf("list pump insights events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load pump insights")
		return
	}

	response := buildPumpInsights(events, rangeDays, rangeStart, rangeEnd)
	response.RangeStartsAtBirth = rangeStartsAtBirth
	writeJSON(w, http.StatusOK, response)
}

func parsePumpInsightsRangeDays(raw string) (int, bool) {
	if raw == "" {
		return pumpInsightsDefaultRangeDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || !pumpInsightsAllowedRangeDays[days] {
		return 0, false
	}
	return days, true
}

func buildPumpInsights(events []store.Event, rangeDays int, rangeStart, rangeEnd time.Time) pumpInsightsResponse {
	sorted := sortedAnalyticsEvents(events)

	visibleDays := sleepInsightsDayCount(rangeStart, rangeEnd)
	days := make([]pumpInsightDayResponse, 0, visibleDays)
	var totals pumpInsightTotals

	for i := 0; i < visibleDays; i++ {
		dayStart := rangeStart.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)

		day := buildPumpInsightDay(sorted, dayStart, dayEnd, i, visibleDays)
		if day.HasData {
			totals.recordedDays++
			totals.sessionCount += day.SessionCount
			totals.totalMl += day.TotalMl
			totals.totalMinutes += day.TotalMinutes
			totals.durationCount += day.durationCount
		}
		days = append(days, day)
	}

	aggregate, observations := buildPumpInsightAggregate(sorted, totals)
	return pumpInsightsResponse{
		RangeDays:    rangeDays,
		RangeLabel:   sleepInsightsRangeLabel(rangeStart, rangeEnd),
		Days:         days,
		Aggregate:    aggregate,
		Observations: observations,
	}
}

func buildPumpInsightDay(events []store.Event, dayStart, dayEnd time.Time, index, rangeDays int) pumpInsightDayResponse {
	day := pumpInsightDayResponse{
		LocalDate: dayStart.Format(time.DateOnly),
		Label:     sleepInsightDayLabel(dayStart, rangeDays),
		ShowLabel: sleepInsightShowLabel(index, rangeDays),
		FullLabel: sleepInsightFullLabel(dayStart),
	}

	var pumpEvents []pumpInsightEventResponse
	for _, ev := range events {
		if ev.EventType != eventTypePump {
			continue
		}
		occurredAt := ev.OccurredAt.In(dayStart.Location())
		if occurredAt.Before(dayStart) || !occurredAt.Before(dayEnd) {
			continue
		}

		ml, _ := attributeInt(ev.Attributes, "amount_ml")
		event := pumpInsightEventResponse{
			TimeLabel:   occurredAt.Format("3:04 PM"),
			VolumeLabel: fmt.Sprintf("%d ml", ml),
		}
		day.SessionCount++
		day.TotalMl += ml
		if minutes, ok := attributeInt(ev.Attributes, "duration_minutes"); ok {
			day.durationCount++
			day.TotalMinutes += minutes
			event.DurationLabel = formatCompactPumpDurationMinutes(minutes)
		} else {
			// No recorded duration means the session is still ongoing —
			// mirrors isOngoingPump in handlers.go and buildFeedInsightEvent's
			// same fallback for a breast feed still in progress.
			event.DurationLabel = "Ongoing"
		}
		pumpEvents = append(pumpEvents, event)
	}
	day.HasData = day.SessionCount > 0
	if day.durationCount > 0 {
		day.DurationLabel = formatCompactPumpDurationMinutes(day.TotalMinutes)
	}
	day.Events = pumpEvents

	return day
}

func buildPumpInsightAggregate(sortedEvents []store.Event, totals pumpInsightTotals) (pumpInsightAggregateResponse, []string) {
	aggregate := pumpInsightAggregateResponse{
		HasAnyData:                totals.sessionCount > 0,
		RecordedDays:              totals.recordedDays,
		SessionCount:              totals.sessionCount,
		SessionsWithDurationCount: totals.durationCount,
		TotalMl:                   totals.totalMl,
		TotalMinutes:              totals.totalMinutes,
	}
	if !aggregate.HasAnyData {
		return aggregate, nil
	}

	var observations []string

	aggregate.TotalMlLabel = fmt.Sprintf("%d ml", totals.totalMl)
	if totals.durationCount == 0 {
		aggregate.TotalDurationLabel = "Not yet available"
	} else {
		aggregate.TotalDurationLabel = formatCompactPumpDurationMinutes(totals.totalMinutes)
	}
	aggregate.AveragePerDayLabel = strconv.FormatFloat(float64(totals.sessionCount)/float64(totals.recordedDays), 'f', 1, 64)
	aggregate.AverageSessionMlLabel = fmt.Sprintf("%d ml", int(math.Round(float64(totals.totalMl)/float64(totals.sessionCount))))
	if totals.durationCount == 0 {
		aggregate.AverageSessionDurationLabel = "Not yet available"
	} else {
		aggregate.AverageSessionDurationLabel = formatCompactPumpDurationMinutes(int(math.Round(float64(totals.totalMinutes) / float64(totals.durationCount))))
	}

	var pumpEvents []store.Event
	for _, ev := range sortedEvents {
		if ev.EventType == eventTypePump {
			pumpEvents = append(pumpEvents, ev)
		}
	}
	if len(pumpEvents) >= 2 {
		var gapMinutesSum float64
		for i := 1; i < len(pumpEvents); i++ {
			gapMinutesSum += pumpEvents[i].OccurredAt.Sub(pumpEvents[i-1].OccurredAt).Minutes()
		}
		avgGapMinutes := int(math.Round(gapMinutesSum / float64(len(pumpEvents)-1)))
		aggregate.HasAverageGap = true
		aggregate.AverageGapLabel = formatCompactPumpDurationMinutes(avgGapMinutes)
		aggregate.AverageGapCaption = "Avg. time between sessions"
		observations = append(observations, fmt.Sprintf("Average recorded time between pumping sessions: %s.", aggregate.AverageGapLabel))
	} else {
		aggregate.AverageGapLabel = "Not yet available"
		aggregate.AverageGapCaption = "Needs more recorded pumping sessions"
	}

	return aggregate, observations
}

// formatCompactPumpDurationMinutes matches formatCompactSleepDurationMinutes
// and formatCompactFeedDurationMinutes but for Pump — its own copy so each
// Insights category can independently adjust its duration formatting.
func formatCompactPumpDurationMinutes(minutes int) string {
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
