package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

// insightsRangeChoices are the only range selections the Insights page
// exposes — kept in sync with backend-api's own allow-list.
var insightsRangeChoices = []struct {
	Days  int
	Label string
}{
	{Days: 7, Label: "7 days"},
	{Days: 30, Label: "30 days"},
	{Days: 90, Label: "3 months"},
}

const insightsDefaultRangeDays = 30

// insightsChartFloorMinutes matches the design's chart baseline: even a
// range with nothing but very short sleeps gets a sensible bar scale instead
// of every bar reading as ~100% tall.
const insightsChartFloorMinutes = 60

type InsightsRangeOption struct {
	Label  string
	Href   string
	Active bool
}

type InsightsChartDay struct {
	Key          string
	Label        string
	ShowLabel    bool
	FullLabel    string
	IsToday      bool
	HasData      bool
	BarPercent   int
	NightPercent int
	NapPercent   int
	Selected     bool
	Href         string
}

type InsightsPeriodRow struct {
	Tag            string
	TagClass       string
	TimeRangeLabel string
	DurationLabel  string
	Ongoing        bool
}

type InsightsSelectedDay struct {
	FullLabel      string
	TotalLabel     string
	CompletedCount int
	LongestLabel   string
	NapNightLabel  string
	Periods        []InsightsPeriodRow
	HasPeriods     bool
	CloseHref      string
}

type InsightsViewData struct {
	Ranges                []InsightsRangeOption
	RangeDays             int
	RangeLabel            string
	HasAnyData            bool
	HeroLabel             string
	ChartClass            string
	ChartDays             []InsightsChartDay
	SelectedDay           *InsightsSelectedDay
	ShowSupportingRow     bool
	AverageCompletedLabel string
	LongestOverallLabel   string
	AverageWakeLabel      string
	AverageWakeCaption    string
	ShowNapNight          bool
	NapPercent            int
	NightPercent          int
	Observations          []string
	ShowObservations      bool
}

type insightsPageData struct {
	Baby     backendclient.Baby
	Account  accountViewData
	Insights InsightsViewData
}

func (h *Handlers) ShowInsights(w http.ResponseWriter, r *http.Request) {
	baby, err := h.Backend.GetCurrentBaby(r.Context())
	if err != nil {
		if errors.Is(err, backendclient.ErrNotFound) {
			http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
			return
		}
		log.Printf("%v", err)
		http.Error(w, "failed to load baby", http.StatusBadGateway)
		return
	}

	rangeDays := insightsRangeFromQuery(r)
	selectedDate := r.URL.Query().Get("day")

	insights, err := h.Backend.GetSleepInsights(r.Context(), rangeDays)
	if err != nil {
		log.Printf("get sleep insights: %v", err)
		http.Error(w, "failed to load sleep insights", http.StatusBadGateway)
		return
	}

	view := buildInsightsView(insights, rangeDays, selectedDate)

	if r.Header.Get("HX-Request") == "true" {
		h.renderInsightsWorkspace(w, view)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := insightsPageData{
		Baby:     baby,
		Account:  h.loadAccount(r.Context()),
		Insights: view,
	}
	if err := h.Templates.ExecuteTemplate(w, "insights", data); err != nil {
		log.Printf("render insights template: %v", err)
	}
}

func (h *Handlers) renderInsightsWorkspace(w http.ResponseWriter, view InsightsViewData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.Templates.ExecuteTemplate(w, "insights-workspace", view); err != nil {
		log.Printf("render insights workspace template: %v", err)
	}
}

func insightsRangeFromQuery(r *http.Request) int {
	raw := r.URL.Query().Get("range")
	for _, choice := range insightsRangeChoices {
		if fmt.Sprintf("%d", choice.Days) == raw {
			return choice.Days
		}
	}
	return insightsDefaultRangeDays
}

func insightsHref(rangeDays int, selectedDate string) string {
	href := fmt.Sprintf("/insights?range=%d", rangeDays)
	if selectedDate != "" {
		href += "&day=" + url.QueryEscape(selectedDate)
	}
	return href
}

// buildInsightsView turns backend-api's fully-computed Sleep Insights payload
// into template-ready view state: which range/day is active, the chart's bar
// heights (simple layout math over the totals backend-api already supplies),
// and per-interaction hrefs. No sleep calculations happen here.
func buildInsightsView(insights backendclient.SleepInsights, rangeDays int, selectedDate string) InsightsViewData {
	ranges := make([]InsightsRangeOption, len(insightsRangeChoices))
	for i, choice := range insightsRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   insightsHref(choice.Days, ""),
			Active: choice.Days == rangeDays,
		}
	}

	maxTotalMinutes := insightsChartFloorMinutes
	for _, day := range insights.Days {
		if day.TotalMinutes > maxTotalMinutes {
			maxTotalMinutes = day.TotalMinutes
		}
	}

	var selectedRaw *backendclient.SleepInsightDay
	chartDays := make([]InsightsChartDay, len(insights.Days))
	for i, day := range insights.Days {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate
		if isSelected {
			d := day
			selectedRaw = &d
		}

		toggleDate := day.LocalDate
		if isSelected {
			toggleDate = ""
		}

		chartDays[i] = InsightsChartDay{
			Key:          day.LocalDate,
			Label:        day.Label,
			ShowLabel:    day.ShowLabel,
			FullLabel:    day.FullLabel,
			IsToday:      day.IsToday,
			HasData:      day.HasData,
			BarPercent:   barPercent(day, maxTotalMinutes),
			NightPercent: splitPercent(day.NightMinutes, day.TotalMinutes),
			NapPercent:   splitPercent(day.NapMinutes, day.TotalMinutes),
			Selected:     isSelected,
			Href:         insightsHref(rangeDays, toggleDate),
		}
	}

	view := InsightsViewData{
		Ranges:     ranges,
		RangeDays:  rangeDays,
		RangeLabel: insights.RangeLabel,
		HasAnyData: insights.Aggregate.HasAnyData,
		HeroLabel:  emptyDash(insights.Aggregate.AverageTotalLabel),
		ChartClass: fmt.Sprintf("insights-chart-%d", rangeDays),
		ChartDays:  chartDays,
	}

	if selectedRaw != nil {
		view.SelectedDay = buildInsightsSelectedDay(*selectedRaw, rangeDays)
	}

	if insights.Aggregate.HasAnyData {
		view.ShowSupportingRow = true
		view.AverageCompletedLabel = insights.Aggregate.AverageCompletedLabel
		view.LongestOverallLabel = emptyDash(insights.Aggregate.LongestOverallLabel)
		view.AverageWakeLabel = insights.Aggregate.AverageWakeWindowLabel
		view.AverageWakeCaption = insights.Aggregate.AverageWakeWindowCaption
		if insights.Aggregate.NapPercent != nil && insights.Aggregate.NightPercent != nil {
			view.ShowNapNight = true
			view.NapPercent = *insights.Aggregate.NapPercent
			view.NightPercent = *insights.Aggregate.NightPercent
		}
	}

	view.Observations = insights.Observations
	view.ShowObservations = len(insights.Observations) > 0

	return view
}

func buildInsightsSelectedDay(day backendclient.SleepInsightDay, rangeDays int) *InsightsSelectedDay {
	rows := make([]InsightsPeriodRow, len(day.Periods))
	for i, period := range day.Periods {
		tag, tagClass := "Night", "night"
		if period.Type == "nap" {
			tag, tagClass = "Nap", "nap"
		}
		rows[i] = InsightsPeriodRow{
			Tag:            tag,
			TagClass:       tagClass,
			TimeRangeLabel: period.TimeRangeLabel,
			DurationLabel:  period.DurationLabel,
			Ongoing:        period.Ongoing,
		}
	}

	totalLabel := day.TotalLabel
	if totalLabel == "" {
		totalLabel = "—"
	}

	return &InsightsSelectedDay{
		FullLabel:      day.FullLabel,
		TotalLabel:     totalLabel,
		CompletedCount: day.CompletedCount,
		LongestLabel:   emptyDash(day.LongestLabel),
		NapNightLabel:  day.NapNightLabel,
		Periods:        rows,
		HasPeriods:     len(rows) > 0,
		CloseHref:      insightsHref(rangeDays, ""),
	}
}

// barPercent turns a day's recorded minutes into a chart bar height, giving
// any recorded sleep (however brief) a visibly solid minimum bar so it never
// looks like the hatched "no data" placeholder.
func barPercent(day backendclient.SleepInsightDay, maxTotalMinutes int) int {
	if !day.HasData {
		return 0
	}
	pct := int(math.Round(float64(day.TotalMinutes) / float64(maxTotalMinutes) * 100))
	if pct < 4 {
		pct = 4
	}
	return pct
}

func splitPercent(part, total int) int {
	if total <= 0 {
		return 50
	}
	return int(math.Round(float64(part) / float64(total) * 100))
}

func emptyDash(label string) string {
	if label == "" {
		return "—"
	}
	return label
}
