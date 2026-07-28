package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	CarryoverNote  string
	CloseHref      string
}

type InsightsViewData struct {
	Category   string
	Categories []InsightsCategoryOption

	Ranges                []InsightsRangeOption
	RangeDays             int
	RangeLabel            string
	HasAnyData            bool
	HeroLabel             string
	AverageBasisLabel     string
	RecordsBeginLabel     string
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

	Metrics                    []InsightsMetricOption
	GrowthHeroValue            string
	GrowthHeroCaption          string
	GrowthRangeLabel           string
	HasGrowthData              bool
	GrowthChartPoints          []InsightsChartPoint
	GrowthLinePoints           string
	SelectedGrowthPoint        *InsightsSelectedGrowthPoint
	ShowGrowthSupportingRow    bool
	GrowthCountLabel           string
	GrowthIntervalLabel        string
	GrowthIntervalCaption      string
	GrowthChangeOverallLabel   string
	GrowthChangeOverallCaption string

	Observations     []string
	ShowObservations bool
}

type InsightsCategoryOption struct {
	Label  string
	Href   string
	Active bool
}

type InsightsMetricOption struct {
	Label  string
	Href   string
	Active bool
}

// InsightsChartPoint is one plotted measurement on the Growth line chart —
// coordinates are pre-computed here (in the growthChartWidth x
// growthChartHeight SVG viewBox) so the template only has to place a
// <circle>, matching how sleep's ChartDays carry a pre-computed BarPercent.
type InsightsChartPoint struct {
	Key         string
	Label       string
	ShowLabel   bool
	FullLabel   string
	ValueLabel  string
	ChangeLabel string
	CX          string
	CY          string
	LeftPercent string
	TopPercent  string
	Radius      string
	Selected    bool
	Href        string
}

type InsightsSelectedGrowthPoint struct {
	FullLabel   string
	ValueLabel  string
	ChangeLabel string
	CloseHref   string
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

	category := insightsCategoryFromQuery(r)

	var view InsightsViewData
	if category == insightsCategoryGrowth {
		metric := insightsMetricFromQuery(r)
		rangeDays := insightsGrowthRangeFromQuery(r)
		selectedPoint := r.URL.Query().Get("point")

		insights, err := h.Backend.GetGrowthInsights(r.Context(), metric, rangeDays)
		if err != nil {
			log.Printf("get growth insights: %v", err)
			http.Error(w, "failed to load growth insights", http.StatusBadGateway)
			return
		}

		view = buildGrowthInsightsView(insights, rangeDays, metric, selectedPoint)
	} else {
		rangeDays := insightsRangeFromQuery(r)
		selectedDate := r.URL.Query().Get("day")

		insights, err := h.Backend.GetSleepInsights(r.Context(), rangeDays)
		if err != nil {
			log.Printf("get sleep insights: %v", err)
			http.Error(w, "failed to load sleep insights", http.StatusBadGateway)
			return
		}

		view = buildInsightsView(insights, rangeDays, selectedDate)
	}

	view.Category = category
	view.Categories = insightsCategoryOptions(category)

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

	chartSourceDays := insights.Days
	trimmedLeadingDays := false
	if rangeDays > 7 && !insights.RangeStartsAtBirth {
		for i, day := range insights.Days {
			if day.HasData {
				chartSourceDays = insights.Days[i:]
				trimmedLeadingDays = i > 0
				break
			}
		}
	}

	maxTotalMinutes := insightsChartFloorMinutes
	for _, day := range chartSourceDays {
		if day.TotalMinutes > maxTotalMinutes {
			maxTotalMinutes = day.TotalMinutes
		}
	}

	var selectedRaw *backendclient.SleepInsightDay
	for _, day := range insights.Days {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate
		if isSelected {
			d := day
			selectedRaw = &d
		}
	}

	chartDays := make([]InsightsChartDay, len(chartSourceDays))
	for i, day := range chartSourceDays {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate

		toggleDate := day.LocalDate
		if isSelected {
			toggleDate = ""
		}

		showLabel := day.ShowLabel
		if rangeDays > 7 && (i == 0 || i == len(chartSourceDays)-1) {
			showLabel = true
		}

		chartDays[i] = InsightsChartDay{
			Key:        day.LocalDate,
			Label:      day.Label,
			ShowLabel:  showLabel,
			FullLabel:  day.FullLabel,
			HasData:    day.HasData,
			BarPercent: barPercent(day, maxTotalMinutes),
			Selected:   isSelected,
			Href:       insightsHref(rangeDays, toggleDate),
		}
		chartDays[i].NightPercent, chartDays[i].NapPercent = splitPercents(day.NightMinutes, day.TotalMinutes)
	}

	partialRecordedRange := rangeDays > 7 &&
		insights.Aggregate.HasAnyData &&
		len(chartSourceDays) > 0 &&
		len(chartSourceDays) < rangeDays

	chartClass := fmt.Sprintf("insights-chart-%d", rangeDays)
	if partialRecordedRange {
		chartClass += " insights-chart-adaptive"
	}

	view := InsightsViewData{
		Ranges:     ranges,
		RangeDays:  rangeDays,
		RangeLabel: insights.RangeLabel,
		HasAnyData: insights.Aggregate.HasAnyData,
		HeroLabel:  emptyDash(insights.Aggregate.AverageTotalLabel),
		ChartClass: chartClass,
		ChartDays:  chartDays,
	}

	startsAtFirstRecordedDay := !insights.RangeStartsAtBirth &&
		partialRecordedRange &&
		len(chartSourceDays) > 0 &&
		chartSourceDays[0].HasData

	if trimmedLeadingDays || startsAtFirstRecordedDay {
		view.RangeLabel = insightsVisibleRangeLabel(chartSourceDays)
		view.RecordsBeginLabel = "Records begin " + chartSourceDays[0].Label
	} else if insights.RangeStartsAtBirth && partialRecordedRange {
		view.RecordsBeginLabel = "Since birth"
	}

	if selectedRaw != nil {
		view.SelectedDay = buildInsightsSelectedDay(*selectedRaw, rangeDays)
	}

	if insights.Aggregate.HasAnyData {
		view.ShowSupportingRow = true
		view.AverageBasisLabel = recordedDaysBasisLabel(insights.Aggregate.RecordedDays)
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

func insightsVisibleRangeLabel(days []backendclient.SleepInsightDay) string {
	if len(days) == 0 {
		return ""
	}
	first, last := days[0].Label, days[len(days)-1].Label
	if first == last {
		return first
	}
	return first + " – " + last
}

func recordedDaysBasisLabel(days int) string {
	if days <= 0 {
		return ""
	}
	if days == 1 {
		return "Based on 1 recorded day"
	}
	return fmt.Sprintf("Based on %d recorded days", days)
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
		NapNightLabel:  emptyDash(day.NapNightLabel),
		Periods:        rows,
		HasPeriods:     len(rows) > 0,
		CarryoverNote:  day.CarryoverNote,
		CloseHref:      insightsHref(rangeDays, ""),
	}
}

// barPercent turns a day's completed recorded minutes into a chart bar
// height, giving any positive duration a visibly solid minimum bar so it
// never looks like the hatched "no data" placeholder.
func barPercent(day backendclient.SleepInsightDay, maxTotalMinutes int) int {
	if !day.HasData || day.TotalMinutes <= 0 {
		return 0
	}
	pct := int(math.Round(float64(day.TotalMinutes) / float64(maxTotalMinutes) * 100))
	if pct < 4 {
		pct = 4
	}
	return pct
}

func splitPercents(night, total int) (nightPercent, napPercent int) {
	if total <= 0 {
		return 0, 0
	}
	nightPercent = int(math.Round(float64(night) / float64(total) * 100))
	return nightPercent, 100 - nightPercent
}

func emptyDash(label string) string {
	if label == "" {
		return "—"
	}
	return label
}

// insightsCategorySleep/insightsCategoryGrowth are the only two Insights
// categories today — the category pill row is built so more can be added
// later without changing this switch.
const (
	insightsCategorySleep  = "sleep"
	insightsCategoryGrowth = "growth"
)

func insightsCategoryFromQuery(r *http.Request) string {
	if r.URL.Query().Get("category") == insightsCategoryGrowth {
		return insightsCategoryGrowth
	}
	return insightsCategorySleep
}

// insightsCategoryOptions links plainly (no htmx) to a fresh /insights load:
// unlike a range or metric switch, a category switch changes the page's
// title and its range pills' own options, not just the card body, so a full
// re-render is simplest.
func insightsCategoryOptions(active string) []InsightsCategoryOption {
	return []InsightsCategoryOption{
		{Label: "Sleep", Href: "/insights?category=" + insightsCategorySleep, Active: active == insightsCategorySleep},
		{Label: "Growth", Href: "/insights?category=" + insightsCategoryGrowth, Active: active == insightsCategoryGrowth},
	}
}

// insightsGrowthRangeChoices are the only Growth range selections the
// Insights page exposes — kept in sync with backend-api's own allow-list.
// Days: 0 is the "All time" sentinel (no cutoff).
var insightsGrowthRangeChoices = []struct {
	Days  int
	Label string
}{
	{Days: 90, Label: "3 months"},
	{Days: 180, Label: "6 months"},
	{Days: 365, Label: "1 year"},
	{Days: 0, Label: "All time"},
}

const insightsGrowthDefaultRangeDays = 180

var insightsGrowthMetricChoices = []struct {
	Key   string
	Label string
}{
	{Key: "weight", Label: "Weight"},
	{Key: "length", Label: "Length"},
	{Key: "head_circumference", Label: "Head circumference"},
}

const insightsDefaultGrowthMetric = "weight"

// growthChartWidth/growthChartHeight fix the SVG viewBox for the Growth line
// chart. The <svg> itself stretches to the card's actual width/height via
// preserveAspectRatio="none" (the same technique already used by the intro
// page's growth-chart teaser), so these are just the coordinate space
// buildGrowthChartPoints computes positions in, not a literal pixel size.
const growthChartWidth = 600
const growthChartHeight = 160

func insightsGrowthRangeFromQuery(r *http.Request) int {
	raw := r.URL.Query().Get("range")
	for _, choice := range insightsGrowthRangeChoices {
		if fmt.Sprintf("%d", choice.Days) == raw {
			return choice.Days
		}
	}
	return insightsGrowthDefaultRangeDays
}

func insightsMetricFromQuery(r *http.Request) string {
	raw := r.URL.Query().Get("metric")
	for _, choice := range insightsGrowthMetricChoices {
		if choice.Key == raw {
			return raw
		}
	}
	return insightsDefaultGrowthMetric
}

func insightsGrowthHref(rangeDays int, metric, selectedPoint string) string {
	href := fmt.Sprintf("/insights?category=%s&range=%d&metric=%s", insightsCategoryGrowth, rangeDays, url.QueryEscape(metric))
	if selectedPoint != "" {
		href += "&point=" + url.QueryEscape(selectedPoint)
	}
	return href
}

// buildGrowthInsightsView turns backend-api's fully-computed Growth Insights
// payload into template-ready view state, the Growth counterpart of
// buildInsightsView: which range/metric/point is active, the line chart's
// plotted coordinates (layout math over the values backend-api already
// supplies), and per-interaction hrefs. No growth calculations happen here.
func buildGrowthInsightsView(insights backendclient.GrowthInsights, rangeDays int, metric, selectedPoint string) InsightsViewData {
	ranges := make([]InsightsRangeOption, len(insightsGrowthRangeChoices))
	for i, choice := range insightsGrowthRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   insightsGrowthHref(choice.Days, metric, ""),
			Active: choice.Days == rangeDays,
		}
	}

	metrics := make([]InsightsMetricOption, len(insightsGrowthMetricChoices))
	for i, choice := range insightsGrowthMetricChoices {
		metrics[i] = InsightsMetricOption{
			Label:  choice.Label,
			Href:   insightsGrowthHref(rangeDays, choice.Key, ""),
			Active: choice.Key == metric,
		}
	}

	chartPoints, linePoints := buildGrowthChartPoints(insights.Points, insights.RangeStart, insights.RangeEnd, selectedPoint, rangeDays, metric)

	view := InsightsViewData{
		Ranges:            ranges,
		Metrics:           metrics,
		GrowthRangeLabel:  insights.RangeLabel,
		HasGrowthData:     insights.HasAnyData,
		GrowthChartPoints: chartPoints,
		GrowthLinePoints:  linePoints,
		GrowthHeroValue:   "—",
		GrowthHeroCaption: fmt.Sprintf("Latest recorded %s", strings.ToLower(insights.MetricLabel)),
	}

	if insights.HasAnyData {
		view.GrowthHeroValue = insights.Aggregate.LatestValueLabel
	}

	if selectedPoint != "" {
		for _, p := range insights.Points {
			if p.ID != selectedPoint {
				continue
			}
			view.SelectedGrowthPoint = &InsightsSelectedGrowthPoint{
				FullLabel:   p.FullLabel,
				ValueLabel:  p.ValueLabel,
				ChangeLabel: p.ChangeLabel,
				CloseHref:   insightsGrowthHref(rangeDays, metric, ""),
			}
			break
		}
	}

	if insights.HasAnyData {
		view.ShowGrowthSupportingRow = true
		view.GrowthCountLabel = strconv.Itoa(insights.Aggregate.Count)
		view.GrowthIntervalLabel = emptyDash(insights.Aggregate.AverageIntervalDaysLabel)
		view.GrowthIntervalCaption = insights.Aggregate.AverageIntervalCaption
		view.GrowthChangeOverallLabel = emptyDash(insights.Aggregate.ChangeOverallLabel)
		view.GrowthChangeOverallCaption = insights.Aggregate.ChangeOverallCaption
	}

	view.Observations = insights.Observations
	view.ShowObservations = len(insights.Observations) > 0

	return view
}

// buildGrowthChartPoints lays out one recorded measurement per plotted point
// along a growthChartWidth x growthChartHeight SVG viewBox — min/max-scaled
// vertically and by elapsed time horizontally, plus the polyline "points"
// attribute connecting them in order.
func buildGrowthChartPoints(points []backendclient.GrowthInsightPoint, rangeStart *time.Time, rangeEnd time.Time, selectedPoint string, rangeDays int, metric string) ([]InsightsChartPoint, string) {
	if len(points) == 0 {
		return nil, ""
	}

	minValue, maxValue := points[0].Value, points[0].Value
	for _, p := range points {
		if p.Value < minValue {
			minValue = p.Value
		}
		if p.Value > maxValue {
			maxValue = p.Value
		}
	}
	span := maxValue - minValue
	if span == 0 {
		span = 1
	}

	const leftMargin, rightMargin = 20.0, 20.0
	const topMargin, bottomMargin = 20.0, 20.0
	plotWidth := float64(growthChartWidth) - leftMargin - rightMargin
	plotHeight := float64(growthChartHeight) - topMargin - bottomMargin

	minTime, maxTime := points[0].OccurredAt, points[0].OccurredAt
	for _, p := range points[1:] {
		if p.OccurredAt.Before(minTime) {
			minTime = p.OccurredAt
		}
		if p.OccurredAt.After(maxTime) {
			maxTime = p.OccurredAt
		}
	}
	if rangeStart != nil && !rangeStart.After(minTime) {
		minTime = *rangeStart
	}
	if rangeEnd.After(maxTime) {
		maxTime = rangeEnd
	}
	timeSpan := maxTime.Sub(minTime)

	n := len(points)
	chartPoints := make([]InsightsChartPoint, n)
	lineParts := make([]string, n)

	for i, p := range points {
		x := float64(growthChartWidth) / 2
		if timeSpan > 0 {
			x = leftMargin + float64(p.OccurredAt.Sub(minTime))/float64(timeSpan)*plotWidth
		}
		y := float64(growthChartHeight) - bottomMargin - (p.Value-minValue)/span*plotHeight

		isSelected := selectedPoint != "" && p.ID == selectedPoint
		toggled := p.ID
		if isSelected {
			toggled = ""
		}
		radius := "4"
		if isSelected {
			radius = "6"
		}

		cx := strconv.FormatFloat(x, 'f', 1, 64)
		cy := strconv.FormatFloat(y, 'f', 1, 64)
		leftPercent := strconv.FormatFloat(x/float64(growthChartWidth)*100, 'f', 2, 64)
		topPercent := strconv.FormatFloat(y/float64(growthChartHeight)*100, 'f', 2, 64)

		chartPoints[i] = InsightsChartPoint{
			Key:         p.ID,
			Label:       p.Label,
			ShowLabel:   p.ShowLabel,
			FullLabel:   p.FullLabel,
			ValueLabel:  p.ValueLabel,
			ChangeLabel: p.ChangeLabel,
			CX:          cx,
			CY:          cy,
			LeftPercent: leftPercent,
			TopPercent:  topPercent,
			Radius:      radius,
			Selected:    isSelected,
			Href:        insightsGrowthHref(rangeDays, metric, toggled),
		}
		lineParts[i] = cx + "," + cy
	}

	return chartPoints, strings.Join(lineParts, " ")
}
