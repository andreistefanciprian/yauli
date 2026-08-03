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

const insightsSleepBoundaryFootnote = "Started the day before or continues into the next. Sleep periods only counts sleeps that started this day, and the duration and chart bar shown here reflect only the portion that fell on this day."

// insightsChartFloorMinutes matches the design's chart baseline: even a
// range with nothing but very short sleeps gets a sensible bar scale instead
// of every bar reading as ~100% tall.
const insightsChartFloorMinutes = 60

// insightsChartAxisStepMinutes/insightsChartAxisMaxMinutes give the sleep
// chart's y-axis labels (6h/12h/18h/24h) and the ceiling bars are scaled
// against — the busiest day's total rounds up to the next 6h mark instead of
// always stretching to fill the chart.
const insightsChartAxisStepMinutes = 360
const insightsChartAxisMaxMinutes = 1440

type InsightsRangeOption struct {
	Label  string
	Href   string
	Active bool
}

type InsightsChartAxisGuide struct {
	Label   string
	Percent string
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
	Boundary       bool
}

type InsightsSelectedDay struct {
	FullLabel      string
	TotalLabel     string
	CompletedCount int
	LongestLabel   string
	NapNightLabel  string
	Periods        []InsightsPeriodRow
	HasPeriods     bool
	BoundaryNote   string
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
	ChartAxisGuides       []InsightsChartAxisGuide
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
	GrowthAxisGuides           []InsightsGrowthAxisGuide
	SelectedGrowthPoint        *InsightsSelectedGrowthPoint
	ShowGrowthSupportingRow    bool
	GrowthCountLabel           string
	GrowthIntervalLabel        string
	GrowthIntervalCaption      string
	GrowthChangeOverallLabel   string
	GrowthChangeOverallCaption string

	NappyChartClass        string
	NappyChartDays         []InsightsNappyChartDay
	NappyChartAxisGuides   []InsightsChartAxisGuide
	NappyHeroValue         string
	NappyRangeLabel        string
	HasNappyData           bool
	SelectedNappyDay       *InsightsNappySelectedDay
	ShowNappySupportingRow bool
	NappyAverageLabel      string
	NappyAverageGapLabel   string
	NappyAverageGapCaption string
	ShowNappyBreakdown     bool
	NappyWeePercent        int
	NappyPooPercent        int
	NappyMixedPercent      int

	IsBreastMetric        bool
	FeedChartClass        string
	FeedChartDays         []InsightsFeedChartDay
	FeedChartAxisGuides   []InsightsChartAxisGuide
	FeedHeroValue         string
	FeedHeroCaption       string
	FeedCountLabel        string
	FeedRangeLabel        string
	HasFeedData           bool
	SelectedFeedDay       *InsightsSelectedFeedDay
	ShowFeedSupportingRow bool
	FeedAverageLabel      string
	FeedAverageGapLabel   string
	FeedAverageGapCaption string
	ShowFeedBreakdown     bool
	FeedBreastPercent     int
	FeedFormulaPercent    int
	FeedExpressedPercent  int

	Observations     []string
	ShowObservations bool
}

type InsightsFeedChartDay struct {
	Key                   string
	Label                 string
	ShowLabel             bool
	FullLabel             string
	HasData               bool
	BarPercent            int
	IsBreastMetric        bool
	IsBottleMetric        bool
	FormulaSharePercent   int
	ExpressedSharePercent int
	Selected              bool
	Href                  string
}

type InsightsFeedEventRow struct {
	Tag         string
	TagClass    string
	TimeLabel   string
	DetailLabel string
}

type InsightsSelectedFeedDay struct {
	FullLabel      string
	TotalLabel     string
	BreastLabel    string
	FormulaLabel   string
	ExpressedLabel string
	Events         []InsightsFeedEventRow
	HasEvents      bool
	CloseHref      string
}

type InsightsNappyChartDay struct {
	Key          string
	Label        string
	ShowLabel    bool
	FullLabel    string
	HasData      bool
	BarPercent   int
	WeePercent   int
	PooPercent   int
	MixedPercent int
	Selected     bool
	Href         string
}

type InsightsNappyEventRow struct {
	Tag       string
	TagClass  string
	TimeLabel string
}

type InsightsNappySelectedDay struct {
	FullLabel  string
	TotalLabel string
	WeeLabel   string
	PooLabel   string
	MixedLabel string
	Events     []InsightsNappyEventRow
	HasEvents  bool
	CloseHref  string
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
	Key          string
	Label        string
	ShowLabel    bool
	FullLabel    string
	ValueLabel   string
	ChangeLabel  string
	CX           string
	CY           string
	LeftPercent  string
	TopPercent   string
	Radius       string
	CalloutClass string
	Selected     bool
	Href         string
}

type InsightsGrowthAxisGuide struct {
	Label      string
	Y          string
	TopPercent string
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
	switch category {
	case insightsCategoryGrowth:
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
	case insightsCategoryNappies:
		rangeDays := insightsRangeFromQuery(r)
		selectedDate := r.URL.Query().Get("day")

		insights, err := h.Backend.GetNappyInsights(r.Context(), rangeDays)
		if err != nil {
			log.Printf("get nappy insights: %v", err)
			http.Error(w, "failed to load nappy insights", http.StatusBadGateway)
			return
		}

		view = buildNappyInsightsView(insights, rangeDays, selectedDate)
	case insightsCategoryFeeds:
		rangeDays := insightsRangeFromQuery(r)
		metric := insightsFeedMetricFromQuery(r)
		selectedDate := r.URL.Query().Get("day")

		insights, err := h.Backend.GetFeedInsights(r.Context(), rangeDays)
		if err != nil {
			log.Printf("get feed insights: %v", err)
			http.Error(w, "failed to load feed insights", http.StatusBadGateway)
			return
		}

		view = buildFeedInsightsView(insights, rangeDays, metric, selectedDate)
	default:
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
	for i, day := range insights.Days {
		if day.HasData {
			chartSourceDays = insights.Days[i:]
			trimmedLeadingDays = i > 0
			break
		}
	}

	maxTotalMinutes := insightsChartFloorMinutes
	for _, day := range chartSourceDays {
		if day.TotalMinutes > maxTotalMinutes {
			maxTotalMinutes = day.TotalMinutes
		}
	}
	axisCeilingMinutes := sleepChartAxisCeilingMinutes(maxTotalMinutes)

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
			BarPercent: barPercent(day, axisCeilingMinutes),
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
		Ranges:          ranges,
		RangeDays:       rangeDays,
		RangeLabel:      insights.RangeLabel,
		HasAnyData:      insights.Aggregate.HasAnyData,
		HeroLabel:       emptyDash(insights.Aggregate.AverageTotalLabel),
		ChartClass:      chartClass,
		ChartDays:       chartDays,
		ChartAxisGuides: sleepChartAxisGuides(axisCeilingMinutes),
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
	hasBoundaryPeriod := false
	for i, period := range day.Periods {
		tag, tagClass := "Night", "night"
		if period.Type == "nap" {
			tag, tagClass = "Nap", "nap"
		}
		boundary := period.StartedPreviousDay || period.ContinuesNextDay
		if boundary {
			hasBoundaryPeriod = true
		}
		rows[i] = InsightsPeriodRow{
			Tag:            tag,
			TagClass:       tagClass,
			TimeRangeLabel: period.TimeRangeLabel,
			DurationLabel:  period.DurationLabel,
			Ongoing:        period.Ongoing,
			Boundary:       boundary,
		}
	}

	totalLabel := day.TotalLabel
	if totalLabel == "" {
		totalLabel = "—"
	}
	boundaryNote := ""
	if hasBoundaryPeriod {
		boundaryNote = insightsSleepBoundaryFootnote
	}

	return &InsightsSelectedDay{
		FullLabel:      day.FullLabel,
		TotalLabel:     totalLabel,
		CompletedCount: day.CompletedCount,
		LongestLabel:   emptyDash(day.LongestLabel),
		NapNightLabel:  emptyDash(day.NapNightLabel),
		Periods:        rows,
		HasPeriods:     len(rows) > 0,
		BoundaryNote:   boundaryNote,
		CloseHref:      insightsHref(rangeDays, ""),
	}
}

// sleepChartAxisCeilingMinutes rounds the busiest day's total up to the next
// 6h mark (capped at 24h) so the chart's bars scale against a y-axis with
// sensible round-number labels instead of an arbitrary maximum.
func sleepChartAxisCeilingMinutes(maxTotalMinutes int) int {
	ceiling := axisCeiling(maxTotalMinutes, insightsChartAxisStepMinutes)
	if ceiling > insightsChartAxisMaxMinutes {
		ceiling = insightsChartAxisMaxMinutes
	}
	return ceiling
}

// sleepChartAxisGuides returns the 6h-step labels (6h/12h/.../ceilingMinutes)
// positioned as a bottom-offset percentage matching how bars are scaled.
func sleepChartAxisGuides(ceilingMinutes int) []InsightsChartAxisGuide {
	return axisGuides(ceilingMinutes, insightsChartAxisStepMinutes, func(mark int) string {
		return fmt.Sprintf("%dh", mark/60)
	})
}

// axisCeiling rounds a bar chart's tallest value up to the next multiple of
// step (at least one step), so bars scale against a y-axis with round-number
// labels instead of an arbitrary maximum.
func axisCeiling(maxValue, step int) int {
	ceiling := ((maxValue + step - 1) / step) * step
	if ceiling < step {
		ceiling = step
	}
	return ceiling
}

// axisGuides returns evenly spaced tick labels from step up to ceiling
// (inclusive), each positioned as a bottom-offset percentage matching how
// barPercent scales bars against the same ceiling.
func axisGuides(ceiling, step int, labelFor func(mark int) string) []InsightsChartAxisGuide {
	guides := make([]InsightsChartAxisGuide, 0, ceiling/step)
	for mark := step; mark <= ceiling; mark += step {
		guides = append(guides, InsightsChartAxisGuide{
			Label:   labelFor(mark),
			Percent: strconv.FormatFloat(float64(mark)/float64(ceiling)*100, 'f', 2, 64),
		})
	}
	return guides
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

// insightsCategorySleep/insightsCategoryFeeds/insightsCategoryNappies/
// insightsCategoryGrowth are the Insights categories — the category pill row
// is built so more can be added later without changing this switch.
const (
	insightsCategorySleep   = "sleep"
	insightsCategoryFeeds   = "feeds"
	insightsCategoryNappies = "nappies"
	insightsCategoryGrowth  = "growth"
)

func insightsCategoryFromQuery(r *http.Request) string {
	switch r.URL.Query().Get("category") {
	case insightsCategoryGrowth:
		return insightsCategoryGrowth
	case insightsCategoryNappies:
		return insightsCategoryNappies
	case insightsCategoryFeeds:
		return insightsCategoryFeeds
	default:
		return insightsCategorySleep
	}
}

// insightsCategoryOptions links plainly (no htmx) to a fresh /insights load:
// unlike a range or metric switch, a category switch changes the page's
// title and its range pills' own options, not just the card body, so a full
// re-render is simplest.
func insightsCategoryOptions(active string) []InsightsCategoryOption {
	return []InsightsCategoryOption{
		{Label: "Sleep", Href: "/insights?category=" + insightsCategorySleep, Active: active == insightsCategorySleep},
		{Label: "Feeds", Href: "/insights?category=" + insightsCategoryFeeds, Active: active == insightsCategoryFeeds},
		{Label: "Nappies", Href: "/insights?category=" + insightsCategoryNappies, Active: active == insightsCategoryNappies},
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
const (
	growthChartWidth        = 600
	growthChartHeight       = 160
	growthChartTopMargin    = 20.0
	growthChartBottomMargin = 20.0
)

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
	axisGuides := buildGrowthAxisGuides(insights.Points)

	view := InsightsViewData{
		Ranges:            ranges,
		Metrics:           metrics,
		GrowthRangeLabel:  insights.RangeLabel,
		HasGrowthData:     insights.HasAnyData,
		GrowthChartPoints: chartPoints,
		GrowthLinePoints:  linePoints,
		GrowthAxisGuides:  axisGuides,
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
	plotWidth := float64(growthChartWidth) - leftMargin - rightMargin
	plotHeight := float64(growthChartHeight) - growthChartTopMargin - growthChartBottomMargin

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
		y := float64(growthChartHeight) - growthChartBottomMargin - (p.Value-minValue)/span*plotHeight

		isSelected := selectedPoint != "" && p.ID == selectedPoint
		toggled := p.ID
		if isSelected {
			toggled = ""
		}
		radius := "4"
		if isSelected {
			radius = "6"
		}
		calloutClass := ""
		if i == 0 {
			calloutClass = "first"
		} else if i == n-1 {
			calloutClass = "last"
		}
		if n > 1 && (p.Value == minValue || p.Value == maxValue) {
			calloutClass = ""
		}

		cx := strconv.FormatFloat(x, 'f', 1, 64)
		cy := strconv.FormatFloat(y, 'f', 1, 64)
		leftPercent := strconv.FormatFloat(x/float64(growthChartWidth)*100, 'f', 2, 64)
		topPercent := strconv.FormatFloat(y/float64(growthChartHeight)*100, 'f', 2, 64)

		chartPoints[i] = InsightsChartPoint{
			Key:          p.ID,
			Label:        p.Label,
			ShowLabel:    p.ShowLabel,
			FullLabel:    p.FullLabel,
			ValueLabel:   p.ValueLabel,
			ChangeLabel:  p.ChangeLabel,
			CX:           cx,
			CY:           cy,
			LeftPercent:  leftPercent,
			TopPercent:   topPercent,
			Radius:       radius,
			CalloutClass: calloutClass,
			Selected:     isSelected,
			Href:         insightsGrowthHref(rangeDays, metric, toggled),
		}
		lineParts[i] = cx + "," + cy
	}

	return chartPoints, strings.Join(lineParts, " ")
}

func buildGrowthAxisGuides(points []backendclient.GrowthInsightPoint) []InsightsGrowthAxisGuide {
	if len(points) < 2 {
		return nil
	}

	minPoint, maxPoint := points[0], points[0]
	for _, point := range points[1:] {
		if point.Value < minPoint.Value {
			minPoint = point
		}
		if point.Value > maxPoint.Value {
			maxPoint = point
		}
	}
	if minPoint.Value == maxPoint.Value {
		return []InsightsGrowthAxisGuide{
			growthAxisGuide(minPoint.ValueLabel, float64(growthChartHeight)-growthChartBottomMargin),
		}
	}

	return []InsightsGrowthAxisGuide{
		growthAxisGuide(maxPoint.ValueLabel, growthChartTopMargin),
		growthAxisGuide(minPoint.ValueLabel, float64(growthChartHeight)-growthChartBottomMargin),
	}
}

func growthAxisGuide(label string, y float64) InsightsGrowthAxisGuide {
	return InsightsGrowthAxisGuide{
		Label:      label,
		Y:          strconv.FormatFloat(y, 'f', 1, 64),
		TopPercent: strconv.FormatFloat(y/float64(growthChartHeight)*100, 'f', 2, 64),
	}
}

// insightsNappyChartFloor matches the design's chart baseline for the
// Nappies bar chart — the counterpart of insightsChartFloorMinutes for
// Sleep, just in change-count units instead of minutes. Its axis labels
// (3/6/9/...) step by the same amount, the Nappies counterpart of
// insightsChartAxisStepMinutes.
const insightsNappyChartFloor = 3
const insightsNappyChartAxisStep = insightsNappyChartFloor

func nappyInsightsHref(rangeDays int, selectedDate string) string {
	href := fmt.Sprintf("/insights?category=%s&range=%d", insightsCategoryNappies, rangeDays)
	if selectedDate != "" {
		href += "&day=" + url.QueryEscape(selectedDate)
	}
	return href
}

// buildNappyInsightsView turns backend-api's fully-computed Nappy Insights
// payload into template-ready view state, the Nappies counterpart of
// buildInsightsView: which range/day is active, the chart's bar heights
// (layout math over the counts backend-api already supplies), and
// per-interaction hrefs. No nappy calculations happen here.
func buildNappyInsightsView(insights backendclient.NappyInsights, rangeDays int, selectedDate string) InsightsViewData {
	ranges := make([]InsightsRangeOption, len(insightsRangeChoices))
	for i, choice := range insightsRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   nappyInsightsHref(choice.Days, ""),
			Active: choice.Days == rangeDays,
		}
	}

	chartSourceDays := insights.Days
	trimmedLeadingDays := false
	for i, day := range insights.Days {
		if day.HasData {
			chartSourceDays = insights.Days[i:]
			trimmedLeadingDays = i > 0
			break
		}
	}

	maxCount := insightsNappyChartFloor
	for _, day := range chartSourceDays {
		if day.TotalCount > maxCount {
			maxCount = day.TotalCount
		}
	}
	nappyAxisCeiling := axisCeiling(maxCount, insightsNappyChartAxisStep)

	var selectedRaw *backendclient.NappyInsightDay
	for _, day := range insights.Days {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate
		if isSelected {
			d := day
			selectedRaw = &d
		}
	}

	chartDays := make([]InsightsNappyChartDay, len(chartSourceDays))
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

		weePercent, pooPercent, mixedPercent := nappySplitPercents(day)
		chartDays[i] = InsightsNappyChartDay{
			Key:          day.LocalDate,
			Label:        day.Label,
			ShowLabel:    showLabel,
			FullLabel:    day.FullLabel,
			HasData:      day.HasData,
			BarPercent:   nappyBarPercent(day, nappyAxisCeiling),
			WeePercent:   weePercent,
			PooPercent:   pooPercent,
			MixedPercent: mixedPercent,
			Selected:     isSelected,
			Href:         nappyInsightsHref(rangeDays, toggleDate),
		}
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
		Ranges:               ranges,
		RangeDays:            rangeDays,
		NappyRangeLabel:      insights.RangeLabel,
		HasNappyData:         insights.Aggregate.HasAnyData,
		NappyHeroValue:       "0",
		NappyChartClass:      chartClass,
		NappyChartDays:       chartDays,
		NappyChartAxisGuides: axisGuides(nappyAxisCeiling, insightsNappyChartAxisStep, func(mark int) string { return strconv.Itoa(mark) }),
	}
	if insights.Aggregate.HasAnyData {
		view.NappyHeroValue = strconv.Itoa(insights.Aggregate.TotalCount)
	}

	startsAtFirstRecordedDay := !insights.RangeStartsAtBirth &&
		partialRecordedRange &&
		len(chartSourceDays) > 0 &&
		chartSourceDays[0].HasData

	if trimmedLeadingDays || startsAtFirstRecordedDay {
		view.NappyRangeLabel = nappyInsightsVisibleRangeLabel(chartSourceDays)
		view.RecordsBeginLabel = "Records begin " + chartSourceDays[0].Label
	} else if insights.RangeStartsAtBirth && partialRecordedRange {
		view.RecordsBeginLabel = "Since birth"
	}

	if selectedRaw != nil {
		view.SelectedNappyDay = buildNappyInsightsSelectedDay(*selectedRaw, rangeDays)
	}

	if insights.Aggregate.HasAnyData {
		view.ShowNappySupportingRow = true
		view.NappyAverageLabel = insights.Aggregate.AveragePerDayLabel
		view.NappyAverageGapLabel = insights.Aggregate.AverageGapLabel
		view.NappyAverageGapCaption = insights.Aggregate.AverageGapCaption
		if insights.Aggregate.WeePercent != nil && insights.Aggregate.PooPercent != nil && insights.Aggregate.MixedPercent != nil {
			view.ShowNappyBreakdown = true
			view.NappyWeePercent = *insights.Aggregate.WeePercent
			view.NappyPooPercent = *insights.Aggregate.PooPercent
			view.NappyMixedPercent = *insights.Aggregate.MixedPercent
		}
	}

	view.Observations = insights.Observations
	view.ShowObservations = len(insights.Observations) > 0

	return view
}

func nappyInsightsVisibleRangeLabel(days []backendclient.NappyInsightDay) string {
	if len(days) == 0 {
		return ""
	}
	first, last := days[0].Label, days[len(days)-1].Label
	if first == last {
		return first
	}
	return first + " – " + last
}

func buildNappyInsightsSelectedDay(day backendclient.NappyInsightDay, rangeDays int) *InsightsNappySelectedDay {
	rows := make([]InsightsNappyEventRow, len(day.Events))
	for i, ev := range day.Events {
		tag, tagClass := nappyKindLabel(ev.Kind)
		rows[i] = InsightsNappyEventRow{Tag: tag, TagClass: tagClass, TimeLabel: ev.TimeLabel}
	}

	return &InsightsNappySelectedDay{
		FullLabel:  day.FullLabel,
		TotalLabel: strconv.Itoa(day.TotalCount),
		WeeLabel:   strconv.Itoa(day.WeeCount),
		PooLabel:   strconv.Itoa(day.PooCount),
		MixedLabel: strconv.Itoa(day.MixedCount),
		Events:     rows,
		HasEvents:  len(rows) > 0,
		CloseHref:  nappyInsightsHref(rangeDays, ""),
	}
}

func nappyKindLabel(kind string) (tag, tagClass string) {
	switch kind {
	case "poo":
		return "Poo", "poo"
	case "mixed":
		return "Wee & Poo", "mixed"
	default:
		return "Wee", "wee"
	}
}

// nappyBarPercent turns a day's nappy count into a chart bar height, giving
// any positive count a visibly solid minimum bar so it never looks like the
// hatched "no data" placeholder — the Nappies counterpart of barPercent.
func nappyBarPercent(day backendclient.NappyInsightDay, maxCount int) int {
	if !day.HasData || day.TotalCount <= 0 {
		return 0
	}
	pct := int(math.Round(float64(day.TotalCount) / float64(maxCount) * 100))
	if pct < 4 {
		pct = 4
	}
	return pct
}

func nappySplitPercents(day backendclient.NappyInsightDay) (weePercent, pooPercent, mixedPercent int) {
	counts := [3]int{day.WeeCount, day.PooCount, day.MixedCount}
	total := day.WeeCount + day.PooCount + day.MixedCount
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

// insightsFeedMetricBreast/insightsFeedMetricBottle select which of the two
// mutually-exclusive Feed Insights views is showing — breast feeds are
// duration-based, formula/expressed feeds are volume-based, so they can't
// share one hero stat or chart y-axis and are switched via a metric sub-tab
// the same way Growth switches between weight/length/head circumference.
const (
	insightsFeedMetricBreast = "breast"
	insightsFeedMetricBottle = "bottle"
)

var insightsFeedMetricChoices = []struct {
	Key   string
	Label string
}{
	{Key: insightsFeedMetricBreast, Label: "Breast"},
	{Key: insightsFeedMetricBottle, Label: "Formula & Expressed"},
}

// insightsFeedBreastChartFloorMinutes/insightsFeedBottleChartFloorMl match
// the design's chart baseline for the Feeds bar chart — the Feeds
// counterpart of insightsChartFloorMinutes/insightsNappyChartFloor, one
// floor per metric since breast is minutes and bottle is ml.
const (
	insightsFeedBreastChartFloorMinutes = 15
	insightsFeedBottleChartFloorMl      = 60
)

// insightsFeedBreastAxisStepMinutes/insightsFeedBottleAxisStepMl give the
// Feeds chart's axis labels (1h/2h/... or 60 ml/120 ml/...) — coarser than
// the bar-scale floors above so the narrow axis column doesn't get crowded.
const (
	insightsFeedBreastAxisStepMinutes = 60
	insightsFeedBottleAxisStepMl      = 60
)

func insightsFeedMetricFromQuery(r *http.Request) string {
	if r.URL.Query().Get("metric") == insightsFeedMetricBottle {
		return insightsFeedMetricBottle
	}
	return insightsFeedMetricBreast
}

func feedInsightsHref(rangeDays int, metric, selectedDate string) string {
	href := fmt.Sprintf("/insights?category=%s&range=%d&metric=%s", insightsCategoryFeeds, rangeDays, url.QueryEscape(metric))
	if selectedDate != "" {
		href += "&day=" + url.QueryEscape(selectedDate)
	}
	return href
}

// buildFeedInsightsView turns backend-api's fully-computed Feed Insights
// payload into template-ready view state, the Feeds counterpart of
// buildNappyInsightsView: which range/metric/day is active, the chart's bar
// heights (layout math over the totals backend-api already supplies), and
// per-interaction hrefs. No feed calculations happen here.
func buildFeedInsightsView(insights backendclient.FeedInsights, rangeDays int, metric, selectedDate string) InsightsViewData {
	isBreastMetric := metric != insightsFeedMetricBottle

	ranges := make([]InsightsRangeOption, len(insightsRangeChoices))
	for i, choice := range insightsRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   feedInsightsHref(choice.Days, metric, ""),
			Active: choice.Days == rangeDays,
		}
	}

	metrics := make([]InsightsMetricOption, len(insightsFeedMetricChoices))
	for i, choice := range insightsFeedMetricChoices {
		metrics[i] = InsightsMetricOption{
			Label:  choice.Label,
			Href:   feedInsightsHref(rangeDays, choice.Key, ""),
			Active: choice.Key == metric,
		}
	}

	chartSourceDays := insights.Days
	trimmedLeadingDays := false
	for i, day := range insights.Days {
		if day.HasData {
			chartSourceDays = insights.Days[i:]
			trimmedLeadingDays = i > 0
			break
		}
	}

	maxVal := insightsFeedBreastChartFloorMinutes
	axisStep := insightsFeedBreastAxisStepMinutes
	if !isBreastMetric {
		maxVal = insightsFeedBottleChartFloorMl
		axisStep = insightsFeedBottleAxisStepMl
	}
	for _, day := range chartSourceDays {
		value := feedChartValue(day, isBreastMetric)
		if value > maxVal {
			maxVal = value
		}
	}
	feedAxisCeiling := axisCeiling(maxVal, axisStep)

	var selectedRaw *backendclient.FeedInsightDay
	for _, day := range insights.Days {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate
		if isSelected {
			d := day
			selectedRaw = &d
		}
	}

	chartDays := make([]InsightsFeedChartDay, len(chartSourceDays))
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

		formulaShare, expressedShare := splitPercents(day.FormulaMl, day.BottleMl)
		metricValue := feedChartValue(day, isBreastMetric)

		chartDays[i] = InsightsFeedChartDay{
			Key:                   day.LocalDate,
			Label:                 day.Label,
			ShowLabel:             showLabel,
			FullLabel:             day.FullLabel,
			HasData:               day.HasData && metricValue > 0,
			BarPercent:            feedBarPercent(day, metricValue, feedAxisCeiling),
			IsBreastMetric:        isBreastMetric,
			IsBottleMetric:        !isBreastMetric,
			FormulaSharePercent:   formulaShare,
			ExpressedSharePercent: expressedShare,
			Selected:              isSelected,
			Href:                  feedInsightsHref(rangeDays, metric, toggleDate),
		}
	}

	partialRecordedRange := rangeDays > 7 &&
		insights.Aggregate.HasAnyData &&
		len(chartSourceDays) > 0 &&
		len(chartSourceDays) < rangeDays

	chartClass := fmt.Sprintf("insights-chart-%d", rangeDays)
	if partialRecordedRange {
		chartClass += " insights-chart-adaptive"
	}

	heroValue, heroCaption := "—", "Total recorded breast feeding time"
	if !isBreastMetric {
		heroCaption = "Total formula & expressed volume"
	}
	if insights.Aggregate.HasAnyData {
		if isBreastMetric {
			heroValue = insights.Aggregate.BreastTotalLabel
		} else {
			heroValue = insights.Aggregate.BottleTotalLabel
		}
	}

	feedAxisLabel := func(mark int) string { return fmt.Sprintf("%dh", mark/60) }
	if !isBreastMetric {
		feedAxisLabel = func(mark int) string { return fmt.Sprintf("%d ml", mark) }
	}

	view := InsightsViewData{
		Ranges:              ranges,
		RangeDays:           rangeDays,
		Metrics:             metrics,
		IsBreastMetric:      isBreastMetric,
		FeedRangeLabel:      insights.RangeLabel,
		HasFeedData:         insights.Aggregate.HasAnyData,
		FeedHeroValue:       heroValue,
		FeedHeroCaption:     heroCaption,
		FeedCountLabel:      strconv.Itoa(insights.Aggregate.TotalCount),
		FeedChartClass:      chartClass,
		FeedChartDays:       chartDays,
		FeedChartAxisGuides: axisGuides(feedAxisCeiling, axisStep, feedAxisLabel),
	}

	startsAtFirstRecordedDay := !insights.RangeStartsAtBirth &&
		partialRecordedRange &&
		len(chartSourceDays) > 0 &&
		chartSourceDays[0].HasData

	if trimmedLeadingDays || startsAtFirstRecordedDay {
		view.FeedRangeLabel = feedInsightsVisibleRangeLabel(chartSourceDays)
		view.RecordsBeginLabel = "Records begin " + chartSourceDays[0].Label
	} else if insights.RangeStartsAtBirth && partialRecordedRange {
		view.RecordsBeginLabel = "Since birth"
	}

	if selectedRaw != nil {
		view.SelectedFeedDay = buildFeedInsightsSelectedDay(*selectedRaw, rangeDays, metric)
	}

	if insights.Aggregate.HasAnyData {
		view.ShowFeedSupportingRow = true
		view.FeedAverageLabel = insights.Aggregate.AveragePerDayLabel
		view.FeedAverageGapLabel = insights.Aggregate.AverageGapLabel
		view.FeedAverageGapCaption = insights.Aggregate.AverageGapCaption
		if insights.Aggregate.BreastPercent != nil && insights.Aggregate.FormulaPercent != nil && insights.Aggregate.ExpressedPercent != nil {
			view.ShowFeedBreakdown = true
			view.FeedBreastPercent = *insights.Aggregate.BreastPercent
			view.FeedFormulaPercent = *insights.Aggregate.FormulaPercent
			view.FeedExpressedPercent = *insights.Aggregate.ExpressedPercent
		}
	}

	view.Observations = insights.Observations
	view.ShowObservations = len(insights.Observations) > 0

	return view
}

func feedInsightsVisibleRangeLabel(days []backendclient.FeedInsightDay) string {
	if len(days) == 0 {
		return ""
	}
	first, last := days[0].Label, days[len(days)-1].Label
	if first == last {
		return first
	}
	return first + " – " + last
}

// feedChartValue picks which of a day's two metric totals the chart bar
// represents — breast minutes or combined formula/expressed ml — matching
// whichever metric sub-tab is active.
func feedChartValue(day backendclient.FeedInsightDay, isBreastMetric bool) int {
	if isBreastMetric {
		return day.BreastMinutes
	}
	return day.BottleMl
}

// feedBarPercent turns a day's feed total into a chart bar height, giving
// any positive value a visibly solid minimum bar so it never looks like the
// hatched "no data" placeholder — the Feeds counterpart of barPercent.
func feedBarPercent(day backendclient.FeedInsightDay, value, maxVal int) int {
	if !day.HasData || value <= 0 {
		return 0
	}
	pct := int(math.Round(float64(value) / float64(maxVal) * 100))
	if pct < 4 {
		pct = 4
	}
	return pct
}

func buildFeedInsightsSelectedDay(day backendclient.FeedInsightDay, rangeDays int, metric string) *InsightsSelectedFeedDay {
	rows := make([]InsightsFeedEventRow, len(day.Events))
	for i, ev := range day.Events {
		tag, tagClass := feedKindLabel(ev.Kind)
		rows[i] = InsightsFeedEventRow{Tag: tag, TagClass: tagClass, TimeLabel: ev.TimeLabel, DetailLabel: ev.DetailLabel}
	}

	return &InsightsSelectedFeedDay{
		FullLabel:      day.FullLabel,
		TotalLabel:     strconv.Itoa(day.TotalCount),
		BreastLabel:    strconv.Itoa(day.BreastCount),
		FormulaLabel:   strconv.Itoa(day.FormulaCount),
		ExpressedLabel: strconv.Itoa(day.ExpressedCount),
		Events:         rows,
		HasEvents:      len(rows) > 0,
		CloseHref:      feedInsightsHref(rangeDays, metric, ""),
	}
}

func feedKindLabel(kind string) (tag, tagClass string) {
	switch kind {
	case "formula":
		return "Formula", "formula"
	case "expressed":
		return "Expressed", "expressed"
	default:
		return "Breast", "breast"
	}
}
