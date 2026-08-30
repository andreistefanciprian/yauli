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
const insightsChartMaxAxisGuides = 4

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
	NappyBlowoutCount      int
	NappyLargeCount        int
	NappyBlowoutLabel      string
	NappyLargeLabel        string

	IsBreastMetric        bool
	FeedChartClass        string
	FeedChartDays         []InsightsFeedChartDay
	FeedChartAxisGuides   []InsightsChartAxisGuide
	FeedHeroValue         string
	FeedHeroCaption       string
	FeedCountBasisLabel   string
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

	IsPumpVolumeMetric        bool
	PumpChartClass            string
	PumpChartDays             []InsightsPumpChartDay
	PumpChartAxisGuides       []InsightsChartAxisGuide
	PumpHeroValue             string
	PumpHeroCaption           string
	PumpCountBasisLabel       string
	PumpRangeLabel            string
	HasPumpData               bool
	SelectedPumpDay           *InsightsSelectedPumpDay
	ShowPumpSupportingRow     bool
	PumpAverageLabel          string
	PumpAverageSessionLabel   string
	PumpAverageSessionCaption string
	PumpAverageMlLabel        string
	PumpAverageGapLabel       string
	PumpAverageGapCaption     string

	// Overview — the Insights Overview tab's "recorded stats" card. See
	// buildOverviewStatsView. Each block's *EmptyLabel/*Label fields follow
	// the file's existing empty-string-means-omit convention (see
	// emptyDash/RecordsBeginLabel): a field is left blank when it has
	// nothing to say, and the template only renders it with {{if}}.
	OverviewRangeContextLabel string

	// Age card — OverviewAgeLabel is blank (card omitted) when the baby's
	// profile has no birth date, same convention as everything below.
	OverviewAgeLabel       string
	OverviewBirthDateLabel string

	OverviewSleepValueLabel string
	OverviewSleepNightLabel string
	OverviewSleepWakeLabel  string
	OverviewSleepEmptyLabel string
	OverviewSleepHref       string

	OverviewFeedValueLabel  string
	OverviewFeedBreastLabel string
	OverviewFeedBottleLabel string
	OverviewFeedEmptyLabel  string
	OverviewFeedHref        string

	OverviewNappyValueLabel string
	OverviewNappyGapLabel   string
	OverviewNappyEmptyLabel string
	OverviewNappyHref       string

	OverviewGrowthValueLabel  string
	OverviewGrowthChangeLabel string
	// OverviewGrowthLengthLabel is a single combined line ("58.3 cm length
	// (+7.8 cm since birth)") rather than mirroring Weight's separate
	// value/change fields — the stats grid cell is narrow, and Length is a
	// secondary figure alongside the card's primary Weight value.
	OverviewGrowthLengthLabel string
	OverviewGrowthHref        string

	OverviewPumpSummaryLabel string
	OverviewPumpHref         string

	// Health & medicine — see applyOverviewHealthView. Unlike Sleep/Feed/
	// Nappy/Pump, this block is independent of the range pill (same
	// reasoning as Growth) and carries its own history-panel toggle state.
	OverviewHealthVaxCountLabel    string
	OverviewHealthVaxEmptyLabel    string
	OverviewHealthVaxShowEmptyNote bool
	OverviewHealthVaxRecentLabel   string
	OverviewHealthVaxMetaLabel     string

	OverviewHealthMedRows       []InsightsHealthMedRow
	OverviewHealthMedEmptyLabel string

	OverviewHealthHistoryOpen  bool
	OverviewHealthHistoryLabel string
	OverviewHealthHistoryHref  string
	OverviewHealthVaxHistory   []InsightsHealthHistoryRow
	OverviewHealthMedHistory   []InsightsHealthHistoryRow

	// OverviewChatGptSummary is the plain-text prompt the "Discuss with
	// ChatGPT" button hands to chatgpt.com's `?q=` prefilled-prompt URL. See
	// buildOverviewChatGptSummary — it only restates the *Label fields above,
	// so this stays in sync with whatever the card actually shows.
	OverviewChatGptSummary string
}

// InsightsHealthMedRow is one of up to three rows in the Health & medicine
// card's collapsed medicine block: name/dose left, short date+time right.
type InsightsHealthMedRow struct {
	NameLabel string
	WhenLabel string
}

// InsightsHealthHistoryRow is one row in the expanded vaccination or
// medicine history.
type InsightsHealthHistoryRow struct {
	NameLabel        string
	HasDescription   bool
	DescriptionLabel string
	WhenLabel        string
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

type InsightsPumpChartDay struct {
	Key        string
	Label      string
	ShowLabel  bool
	FullLabel  string
	AriaLabel  string
	HasData    bool
	BarPercent int
	Selected   bool
	Href       string
}

type InsightsPumpEventRow struct {
	TimeLabel     string
	DurationLabel string
	VolumeLabel   string
}

type InsightsSelectedPumpDay struct {
	FullLabel     string
	SessionsLabel string
	DurationLabel string
	VolumeLabel   string
	Events        []InsightsPumpEventRow
	HasEvents     bool
	CloseHref     string
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
	Markers      []InsightsNappyMarker
	Selected     bool
	Href         string
}

// InsightsNappyMarker overlays a thin line on a day's stacked bar for one
// large or blowout poo — its class picks the color/thickness in CSS, and
// BottomPercent positions it by where the event fell in that day's
// sequence, so a marker's height reads as roughly when in the day it
// happened.
type InsightsNappyMarker struct {
	BottomPercent string
	SizeClass     string // "large" or "blowout"
}

type InsightsNappyEventRow struct {
	Tag       string
	TagClass  string
	TimeLabel string
	HasSize   bool
	SizeLabel string
	SizeClass string
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
	case insightsCategoryOverview:
		rangeDays := insightsRangeFromQuery(r)
		historyOpen := r.URL.Query().Get("history") == "1"

		insights, err := h.Backend.GetOverviewInsights(r.Context(), rangeDays)
		if err != nil {
			log.Printf("get overview insights: %v", err)
			http.Error(w, "failed to load overview insights", http.StatusBadGateway)
			return
		}

		view = buildOverviewStatsView(baby, insights, rangeDays, historyOpen)
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
	case insightsCategoryPump:
		rangeDays := insightsRangeFromQuery(r)
		metric := insightsPumpMetricFromQuery(r)
		selectedDate := r.URL.Query().Get("day")

		insights, err := h.Backend.GetPumpInsights(r.Context(), rangeDays)
		if err != nil {
			log.Printf("get pump insights: %v", err)
			http.Error(w, "failed to load pump insights", http.StatusBadGateway)
			return
		}

		view = buildPumpInsightsView(insights, rangeDays, metric, selectedDate)
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
	href := fmt.Sprintf("/insights?category=%s&range=%d", insightsCategorySleep, rangeDays)
	if selectedDate != "" {
		href += "&day=" + url.QueryEscape(selectedDate)
	}
	return href
}

func insightsOverviewHref(rangeDays int, historyOpen bool) string {
	href := fmt.Sprintf("/insights?category=%s&range=%d", insightsCategoryOverview, rangeDays)
	if historyOpen {
		href += "&history=1"
	}
	return href
}

// overviewFacts is the Insights Overview tab's single source of truth: the
// baby's identity/age plus backend-api's already-aggregated stats for the
// selected range, bundled together as plain values (no HTML, arrows, or
// hrefs — see backendclient.OverviewInsights, which is already exactly that
// shape). Built once per request by buildOverviewFacts; buildOverviewStatsView
// renders it into template copy/hrefs/colors, and buildOverviewChatGptSummary
// renders the same facts into the ChatGPT prompt, so the two can never
// disagree about the underlying numbers — only about how each audience wants
// them phrased.
type overviewFacts struct {
	RangeLabel     string
	BabyName       string
	BirthDateLabel string
	AgeLabel       string
	Insights       backendclient.OverviewInsights
}

func buildOverviewFacts(baby backendclient.Baby, insights backendclient.OverviewInsights, rangeLabel string) overviewFacts {
	facts := overviewFacts{
		RangeLabel: rangeLabel,
		BabyName:   baby.Name,
		AgeLabel:   insights.AgeLabel,
		Insights:   insights,
	}
	if birth, err := time.Parse(time.DateOnly, baby.BirthDate); err == nil {
		facts.BirthDateLabel = birth.Format("2 January 2006")
	}
	return facts
}

// buildOverviewStatsView turns overviewFacts into template-ready view state
// for the Overview tab: the age card, the "recorded stats" card (one
// value/support-line pair per category plus the pump footer row), and hrefs.
// Sleep/Feeds/Nappies follow the requested range (reflected in
// OverviewRangeContextLabel); Growth always reports against the whole
// recorded history regardless of range, so its copy says "since birth", not
// "in the last N days". No aggregation happens here — every number is
// already computed by backend-api; this only picks copy and hrefs.
func buildOverviewStatsView(baby backendclient.Baby, insights backendclient.OverviewInsights, rangeDays int, historyOpen bool) InsightsViewData {
	ranges := make([]InsightsRangeOption, len(insightsRangeChoices))
	rangeLabel := ""
	for i, choice := range insightsRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   insightsOverviewHref(choice.Days, historyOpen),
			Active: choice.Days == rangeDays,
		}
		if choice.Days == rangeDays {
			rangeLabel = choice.Label
		}
	}

	facts := buildOverviewFacts(baby, insights, rangeLabel)

	view := InsightsViewData{
		Ranges:                    ranges,
		RangeDays:                 rangeDays,
		OverviewRangeContextLabel: "Recorded over the last " + rangeLabel,
		OverviewAgeLabel:          facts.AgeLabel,
		OverviewBirthDateLabel:    facts.BirthDateLabel,
		OverviewSleepHref:         "/insights?category=" + insightsCategorySleep,
		OverviewFeedHref:          "/insights?category=" + insightsCategoryFeeds,
		OverviewNappyHref:         "/insights?category=" + insightsCategoryNappies,
		OverviewGrowthHref:        "/insights?category=" + insightsCategoryGrowth,
		OverviewPumpHref:          "/insights?category=" + insightsCategoryPump,
	}

	sleep := facts.Insights.Sleep
	view.OverviewSleepValueLabel = emptyDash(sleep.AverageTotalLabel)
	if !sleep.Available {
		view.OverviewSleepEmptyLabel = "Temporarily unavailable"
	} else if sleep.HasAnyData {
		if sleep.NightPercent != nil {
			view.OverviewSleepNightLabel = fmt.Sprintf("%d%% recorded overnight", *sleep.NightPercent)
		}
		if sleep.HasWakeWindow {
			view.OverviewSleepWakeLabel = sleep.AverageWakeWindowLabel + " average awake window"
		} else {
			view.OverviewSleepWakeLabel = "Not enough recorded sleep yet for an average awake window"
		}
	} else {
		view.OverviewSleepEmptyLabel = "Not enough recorded sleep yet"
	}

	feed := facts.Insights.Feed
	view.OverviewFeedValueLabel = emptyDash(feed.AveragePerDayLabel)
	if !feed.Available {
		view.OverviewFeedEmptyLabel = "Temporarily unavailable"
	} else if feed.HasAnyData {
		view.OverviewFeedBreastLabel = feed.BreastTotalLabel + " breast"
		view.OverviewFeedBottleLabel = feed.BottleTotalLabel + " bottle"
	} else {
		view.OverviewFeedEmptyLabel = "Not enough recorded feeds yet"
	}

	nappy := facts.Insights.Nappy
	view.OverviewNappyValueLabel = emptyDash(nappy.AveragePerDayLabel)
	if !nappy.Available {
		view.OverviewNappyEmptyLabel = "Temporarily unavailable"
	} else if nappy.HasAverageGap {
		view.OverviewNappyGapLabel = nappy.AverageGapLabel + " average spacing"
	} else {
		view.OverviewNappyEmptyLabel = "Not enough recorded changes yet"
	}

	growth := facts.Insights.Growth
	view.OverviewGrowthValueLabel = emptyDash(growth.LatestValueLabel)
	switch {
	case !growth.Available:
		view.OverviewGrowthChangeLabel = "Temporarily unavailable"
	case growth.HasBirthWeight:
		view.OverviewGrowthChangeLabel = growth.ChangeSinceBirthLabel + " since birth"
	case growth.HasAnyData:
		view.OverviewGrowthChangeLabel = "Birth weight not recorded"
	default:
		view.OverviewGrowthChangeLabel = "No recorded weight yet"
	}
	if growth.HasLengthData {
		view.OverviewGrowthLengthLabel = growth.LatestLengthLabel + " length"
		if growth.HasBirthLength {
			view.OverviewGrowthLengthLabel += " (" + growth.LengthChangeSinceBirthLabel + " since birth)"
		}
	}

	pump := facts.Insights.Pump
	if !pump.Available {
		view.OverviewPumpSummaryLabel = "Temporarily unavailable"
	} else if pump.HasAnyData {
		sessions := nappyMarkerCountLabel(pump.SessionCount, "pumping session", "pumping sessions")
		view.OverviewPumpSummaryLabel = sessions + " · " + pump.TotalMlLabel + " expressed"
	} else {
		view.OverviewPumpSummaryLabel = "No pumping sessions recorded"
	}

	applyOverviewHealthView(&view, facts.Insights.Health, rangeDays, historyOpen)
	view.OverviewChatGptSummary = buildOverviewChatGptSummary(facts)

	return view
}

// buildOverviewChatGptSummary composes the plain-text digest the "Discuss
// with ChatGPT" button sends via chatgpt.com's `?q=` prefilled-prompt URL
// (the same mechanism ChatGPT's own share links use — it fills the composer,
// it does not auto-send, so the parent still reviews it before anything
// leaves the device). It reads directly from overviewFacts rather than the
// rendered InsightsViewData, so a copy change to the stats card can't
// silently change what the prompt says — both are independent phrasings of
// the same underlying numbers.
func buildOverviewChatGptSummary(facts overviewFacts) string {
	var lines []string
	add := func(s string) {
		if s != "" {
			lines = append(lines, "- "+s)
		}
	}

	switch {
	case facts.AgeLabel != "" && facts.BirthDateLabel != "":
		add("Age: " + facts.AgeLabel + " (born " + facts.BirthDateLabel + ")")
	case facts.AgeLabel != "":
		add("Age: " + facts.AgeLabel)
	case facts.BirthDateLabel != "":
		add("Born: " + facts.BirthDateLabel)
	}

	sleep := facts.Insights.Sleep
	switch {
	case !sleep.Available:
		add("Sleep: Temporarily unavailable")
	case sleep.HasAnyData:
		line := "Sleep: " + sleep.AverageTotalLabel + " per day"
		if sleep.NightPercent != nil {
			line += fmt.Sprintf(", %d%% recorded overnight", *sleep.NightPercent)
		}
		add(line)
	default:
		add("Sleep: Not enough recorded sleep yet")
	}

	feed := facts.Insights.Feed
	switch {
	case !feed.Available:
		add("Feeds: Temporarily unavailable")
	case feed.HasAnyData:
		add("Feeds: " + feed.AveragePerDayLabel + " per day")
	default:
		add("Feeds: Not enough recorded feeds yet")
	}

	nappy := facts.Insights.Nappy
	switch {
	case !nappy.Available:
		add("Nappies: Temporarily unavailable")
	case nappy.HasAverageGap:
		add("Nappies: " + nappy.AveragePerDayLabel + " per day, " + nappy.AverageGapLabel + " average spacing")
	case nappy.HasAnyData:
		add("Nappies: " + nappy.AveragePerDayLabel + " per day")
	default:
		add("Nappies: Not enough recorded changes yet")
	}

	growth := facts.Insights.Growth
	switch {
	case !growth.Available:
		add("Growth: Temporarily unavailable")
	case growth.HasAnyData:
		line := "Growth: " + growth.LatestValueLabel
		if growth.HasBirthWeight {
			line += " (" + growth.ChangeSinceBirthLabel + " since birth)"
		}
		if growth.HasLengthData {
			line += "; " + growth.LatestLengthLabel + " length"
			if growth.HasBirthLength {
				line += " (" + growth.LengthChangeSinceBirthLabel + " since birth)"
			}
		}
		add(line)
	default:
		add("Growth: No recorded weight yet")
	}

	pump := facts.Insights.Pump
	if !pump.Available {
		add("Feeding support: Temporarily unavailable")
	} else if pump.HasAnyData {
		sessions := nappyMarkerCountLabel(pump.SessionCount, "pumping session", "pumping sessions")
		add("Feeding support: " + sessions + " · " + pump.TotalMlLabel + " expressed")
	} else {
		add("Feeding support: No pumping sessions recorded")
	}

	health := facts.Insights.Health
	switch {
	case !health.Available:
		add("Vaccinations: Temporarily unavailable")
		add("Medicine: Temporarily unavailable")
	default:
		if health.HasVaccinations {
			line := strconv.Itoa(health.VaccinationCount) + " recorded — Most recent: " + health.RecentGroupLabel + " (" + health.RecentDateLabel
			if health.RecentAgeLabel != "" {
				line += " · " + health.RecentAgeLabel
			}
			add("Vaccinations: " + line + ")")
		} else {
			add("Vaccinations: None recorded")
		}

		if health.HasMedicine {
			medLines := make([]string, len(health.MedicineRecent))
			for i, ev := range health.MedicineRecent {
				medLines[i] = ev.NameLabel + " (" + ev.ShortWhenLabel + ")"
			}
			add("Medicine: " + strings.Join(medLines, "; "))
		} else {
			add("Medicine: None recorded")
		}
	}

	intro := "Here is a summary of my baby's recorded data from Yauli (" + facts.RangeLabel + " for feeds/sleep/nappies; growth and health cover the whole recorded history):"
	return intro + "\n" + strings.Join(lines, "\n") + "\n\nWhat patterns or questions should I consider?"
}

// applyOverviewHealthView fills in the Health & medicine card's fields on an
// in-progress InsightsViewData. Split out from buildOverviewStatsView only
// because the health block has enough of its own branching (populated vs.
// empty vaccinations, populated vs. empty medicine, the history toggle) to
// otherwise crowd the rest of the stats card's straight-line assembly.
func applyOverviewHealthView(view *InsightsViewData, health backendclient.OverviewHealthStats, rangeDays int, historyOpen bool) {
	if !health.Available {
		view.OverviewHealthVaxEmptyLabel = "Temporarily unavailable"
		view.OverviewHealthMedEmptyLabel = "Temporarily unavailable"
		return
	}

	view.OverviewHealthHistoryOpen = historyOpen
	view.OverviewHealthHistoryHref = insightsOverviewHref(rangeDays, !historyOpen)
	view.OverviewHealthHistoryLabel = "View health history →"
	if historyOpen {
		view.OverviewHealthHistoryLabel = "Hide health history"
	}

	if health.HasVaccinations {
		view.OverviewHealthVaxCountLabel = strconv.Itoa(health.VaccinationCount) + " recorded"
		view.OverviewHealthVaxRecentLabel = "Most recent: " + health.RecentGroupLabel
		view.OverviewHealthVaxMetaLabel = health.RecentDateLabel
		if health.RecentAgeLabel != "" {
			view.OverviewHealthVaxMetaLabel += " · " + health.RecentAgeLabel
		}
	} else {
		view.OverviewHealthVaxEmptyLabel = "None recorded"
		view.OverviewHealthVaxShowEmptyNote = true
	}

	if health.HasMedicine {
		view.OverviewHealthMedRows = make([]InsightsHealthMedRow, len(health.MedicineRecent))
		for i, ev := range health.MedicineRecent {
			view.OverviewHealthMedRows[i] = InsightsHealthMedRow{NameLabel: ev.NameLabel, WhenLabel: ev.ShortWhenLabel}
		}
	} else {
		view.OverviewHealthMedEmptyLabel = "None recorded"
	}

	if !historyOpen {
		return
	}

	view.OverviewHealthVaxHistory = make([]InsightsHealthHistoryRow, len(health.VaccineHistory))
	for i, ev := range health.VaccineHistory {
		view.OverviewHealthVaxHistory[i] = InsightsHealthHistoryRow{
			NameLabel: ev.NameLabel, HasDescription: ev.DescriptionLabel != "", DescriptionLabel: ev.DescriptionLabel, WhenLabel: ev.WhenLabel,
		}
	}

	view.OverviewHealthMedHistory = make([]InsightsHealthHistoryRow, len(health.MedicineHistory))
	for i, ev := range health.MedicineHistory {
		view.OverviewHealthMedHistory[i] = InsightsHealthHistoryRow{NameLabel: ev.NameLabel, WhenLabel: ev.WhenLabel}
	}
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
			HasData:    day.TotalMinutes > 0,
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
	view.AverageBasisLabel = insights.Aggregate.AverageTotalBasisLabel
	if view.AverageBasisLabel == "" {
		view.AverageBasisLabel = recordedDaysBasisLabel(insights.Aggregate.RecordedDays)
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

// boundedAxisStep increases a chart's minimum tick step just enough to keep
// the number of axis labels within maxGuides. The result remains a multiple
// of the chart's existing unit-specific step, so labels stay round.
func boundedAxisStep(maxValue, minimumStep, maxGuides int) int {
	minimumStepCount := (maxValue + minimumStep - 1) / minimumStep
	stepMultiplier := (minimumStepCount + maxGuides - 1) / maxGuides
	if stepMultiplier < 1 {
		stepMultiplier = 1
	}
	return minimumStep * stepMultiplier
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

// These are the Insights categories. Overview is the default when the query
// omits or supplies an unknown category.
const (
	insightsCategorySleep    = "sleep"
	insightsCategoryFeeds    = "feeds"
	insightsCategoryPump     = "pump"
	insightsCategoryNappies  = "nappies"
	insightsCategoryGrowth   = "growth"
	insightsCategoryOverview = "overview"
)

func insightsCategoryFromQuery(r *http.Request) string {
	switch r.URL.Query().Get("category") {
	case insightsCategoryOverview:
		return insightsCategoryOverview
	case insightsCategorySleep:
		return insightsCategorySleep
	case insightsCategoryGrowth:
		return insightsCategoryGrowth
	case insightsCategoryNappies:
		return insightsCategoryNappies
	case insightsCategoryFeeds:
		return insightsCategoryFeeds
	case insightsCategoryPump:
		return insightsCategoryPump
	default:
		return insightsCategoryOverview
	}
}

// insightsCategoryOptions links plainly (no htmx) to a fresh /insights load:
// unlike a range or metric switch, a category switch changes the page's
// title and its range pills' own options, not just the card body, so a full
// re-render is simplest.
func insightsCategoryOptions(active string) []InsightsCategoryOption {
	return []InsightsCategoryOption{
		{Label: "Overview", Href: "/insights?category=" + insightsCategoryOverview, Active: active == insightsCategoryOverview},
		{Label: "Sleep", Href: "/insights?category=" + insightsCategorySleep, Active: active == insightsCategorySleep},
		{Label: "Feeds", Href: "/insights?category=" + insightsCategoryFeeds, Active: active == insightsCategoryFeeds},
		{Label: "Pump", Href: "/insights?category=" + insightsCategoryPump, Active: active == insightsCategoryPump},
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
	lastShownDate := ""

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

		showLabel := p.ShowLabel && p.LocalDate != lastShownDate
		if showLabel {
			lastShownDate = p.LocalDate
		}

		chartPoints[i] = InsightsChartPoint{
			Key:          p.ID,
			Label:        p.Label,
			ShowLabel:    showLabel,
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
// Sleep, just in change-count units instead of minutes. Axis labels start
// with the same three-change step and increase it for busy ranges so the
// chart never shows more than insightsChartMaxAxisGuides labels.
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
	nappyAxisStep := boundedAxisStep(maxCount, insightsNappyChartAxisStep, insightsChartMaxAxisGuides)
	nappyAxisCeiling := axisCeiling(maxCount, nappyAxisStep)

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
			Markers:      nappyDayMarkers(day),
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
		NappyChartAxisGuides: axisGuides(nappyAxisCeiling, nappyAxisStep, func(mark int) string { return strconv.Itoa(mark) }),
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
			view.NappyBlowoutCount = insights.Aggregate.BlowoutCount
			view.NappyLargeCount = insights.Aggregate.LargeCount
			view.NappyBlowoutLabel = nappyMarkerCountLabel(insights.Aggregate.BlowoutCount, "blowout", "blowouts")
			view.NappyLargeLabel = nappyMarkerCountLabel(insights.Aggregate.LargeCount, "large poo", "large poos")
		}
	}

	return view
}

func nappyMarkerCountLabel(count int, singular, plural string) string {
	label := plural
	if count == 1 {
		label = singular
	}
	return strconv.Itoa(count) + " " + label
}

// nappyDayMarkers overlays a thin line on a day's stacked bar for each large
// or blowout event, positioned by where it fell in that day's sequence
// (bottom: ((eventIndex + 0.5) / totalEventsThatDay) * 100%) so a marker's
// height reads as roughly when in the day it happened. Smear/small/medium
// poos and wee-only events draw nothing — the chart flags only what a
// parent would want to spot at a glance.
func nappyDayMarkers(day backendclient.NappyInsightDay) []InsightsNappyMarker {
	total := len(day.Events)
	if total == 0 {
		return nil
	}

	var markers []InsightsNappyMarker
	for i, ev := range day.Events {
		if ev.Size != "large" && ev.Size != "blowout" {
			continue
		}
		bottomPercent := (float64(i) + 0.5) / float64(total) * 100
		markers = append(markers, InsightsNappyMarker{
			BottomPercent: strconv.FormatFloat(bottomPercent, 'f', 1, 64),
			SizeClass:     ev.Size,
		})
	}
	return markers
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
		sizeLabel, sizeClass := nappySizeLabel(ev.Size)
		rows[i] = InsightsNappyEventRow{
			Tag: tag, TagClass: tagClass, TimeLabel: ev.TimeLabel,
			HasSize: sizeLabel != "", SizeLabel: sizeLabel, SizeClass: sizeClass,
		}
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

// nappySizeLabel maps a stored poo_size to its Insights display label and
// badge color tier — Smear/Small/Medium share a neutral tier, Large gets its
// own amber tier, and Blowout gets its own red tier, matching every sized
// event's badge in the selected-day list. Returns ("", "") for a wee-only
// event, which has no size at all.
func nappySizeLabel(size string) (label, class string) {
	switch size {
	case "smear":
		return "Smear", "neutral"
	case "small":
		return "Small", "neutral"
	case "medium":
		return "Medium", "neutral"
	case "large":
		return "Large", "large"
	case "blowout":
		return "Blowout", "blowout"
	default:
		return "", ""
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
// Feeds chart's minimum axis steps. Busy ranges increase these steps so the
// narrow axis column never shows more than insightsChartMaxAxisGuides labels.
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
	axisStep = boundedAxisStep(maxVal, axisStep, insightsChartMaxAxisGuides)
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
	metricFeedCount := insights.Aggregate.BreastCount
	countBasisLabel := insights.Aggregate.BreastDurationBasisLabel
	if !isBreastMetric {
		heroCaption = "Total formula & expressed volume"
		metricFeedCount = insights.Aggregate.FormulaCount + insights.Aggregate.ExpressedCount
		countBasisLabel = fmt.Sprintf("%d out of %d feeds recorded", metricFeedCount, insights.Aggregate.TotalCount)
	} else if countBasisLabel == "" {
		countBasisLabel = fmt.Sprintf("%d out of %d feeds recorded", metricFeedCount, insights.Aggregate.TotalCount)
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
		FeedCountBasisLabel: countBasisLabel,
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

// insightsPumpMetricVolume/insightsPumpMetricDuration select which of the
// two mutually-exclusive Pump Insights views is showing — pumping is both a
// measured output and a timed activity, so it switches via a metric sub-tab
// the same way Feeds switches between Breast and Formula & Expressed.
const (
	insightsPumpMetricVolume   = "volume"
	insightsPumpMetricDuration = "duration"
)

var insightsPumpMetricChoices = []struct {
	Key   string
	Label string
}{
	{Key: insightsPumpMetricVolume, Label: "Volume"},
	{Key: insightsPumpMetricDuration, Label: "Duration"},
}

// insightsPumpVolumeChartFloorMl/insightsPumpDurationChartFloorMinutes match
// the design's chart baseline for the Pump bar chart — the Pump counterpart
// of insightsFeedBreastChartFloorMinutes/insightsFeedBottleChartFloorMl.
const (
	insightsPumpVolumeChartFloorMl        = 60
	insightsPumpDurationChartFloorMinutes = 15
)

// insightsPumpVolumeAxisStepMl/insightsPumpDurationAxisStepMinutes give the
// Pump chart's minimum axis steps. Busy ranges increase these steps so the
// narrow axis column never shows more than insightsChartMaxAxisGuides labels.
const (
	insightsPumpVolumeAxisStepMl        = 60
	insightsPumpDurationAxisStepMinutes = 60
)

func insightsPumpMetricFromQuery(r *http.Request) string {
	if r.URL.Query().Get("metric") == insightsPumpMetricDuration {
		return insightsPumpMetricDuration
	}
	return insightsPumpMetricVolume
}

func pumpInsightsHref(rangeDays int, metric, selectedDate string) string {
	href := fmt.Sprintf("/insights?category=%s&range=%d&metric=%s", insightsCategoryPump, rangeDays, url.QueryEscape(metric))
	if selectedDate != "" {
		href += "&day=" + url.QueryEscape(selectedDate)
	}
	return href
}

// buildPumpInsightsView turns backend-api's fully-computed Pump Insights
// payload into template-ready view state, the Pump counterpart of
// buildFeedInsightsView: which range/metric/day is active, the chart's bar
// heights (layout math over the totals backend-api already supplies), and
// per-interaction hrefs. No pump calculations happen here.
func buildPumpInsightsView(insights backendclient.PumpInsights, rangeDays int, metric, selectedDate string) InsightsViewData {
	isVolumeMetric := metric != insightsPumpMetricDuration

	ranges := make([]InsightsRangeOption, len(insightsRangeChoices))
	for i, choice := range insightsRangeChoices {
		ranges[i] = InsightsRangeOption{
			Label:  choice.Label,
			Href:   pumpInsightsHref(choice.Days, metric, ""),
			Active: choice.Days == rangeDays,
		}
	}

	metrics := make([]InsightsMetricOption, len(insightsPumpMetricChoices))
	for i, choice := range insightsPumpMetricChoices {
		metrics[i] = InsightsMetricOption{
			Label:  choice.Label,
			Href:   pumpInsightsHref(rangeDays, choice.Key, ""),
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

	maxVal := insightsPumpVolumeChartFloorMl
	axisStep := insightsPumpVolumeAxisStepMl
	if !isVolumeMetric {
		maxVal = insightsPumpDurationChartFloorMinutes
		axisStep = insightsPumpDurationAxisStepMinutes
	}
	for _, day := range chartSourceDays {
		value := pumpChartValue(day, isVolumeMetric)
		if value > maxVal {
			maxVal = value
		}
	}
	axisStep = boundedAxisStep(maxVal, axisStep, insightsChartMaxAxisGuides)
	pumpAxisCeiling := axisCeiling(maxVal, axisStep)

	var selectedRaw *backendclient.PumpInsightDay
	for _, day := range insights.Days {
		isSelected := selectedDate != "" && day.LocalDate == selectedDate
		if isSelected {
			d := day
			selectedRaw = &d
		}
	}

	chartDays := make([]InsightsPumpChartDay, len(chartSourceDays))
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

		value := pumpChartValue(day, isVolumeMetric)

		chartDays[i] = InsightsPumpChartDay{
			Key:        day.LocalDate,
			Label:      day.Label,
			ShowLabel:  showLabel,
			FullLabel:  day.FullLabel,
			AriaLabel:  pumpChartAriaLabel(day, isVolumeMetric),
			HasData:    day.HasData && value > 0,
			BarPercent: pumpBarPercent(day, value, pumpAxisCeiling),
			Selected:   isSelected,
			Href:       pumpInsightsHref(rangeDays, metric, toggleDate),
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

	heroValue, heroCaption := "—", "Total expressed volume"
	countBasisLabel := fmt.Sprintf("%d sessions recorded", insights.Aggregate.SessionCount)
	if !isVolumeMetric {
		heroCaption = "Total recorded pumping time"
		countBasisLabel = insights.Aggregate.DurationBasisLabel
		if countBasisLabel == "" {
			countBasisLabel = fmt.Sprintf("%d out of %d sessions have duration recorded", insights.Aggregate.SessionsWithDurationCount, insights.Aggregate.SessionCount)
		}
	}
	if insights.Aggregate.HasAnyData {
		if isVolumeMetric {
			heroValue = insights.Aggregate.TotalMlLabel
		} else {
			heroValue = insights.Aggregate.TotalDurationLabel
		}
	}

	pumpAxisLabel := func(mark int) string { return fmt.Sprintf("%d ml", mark) }
	if !isVolumeMetric {
		pumpAxisLabel = func(mark int) string { return fmt.Sprintf("%dh", mark/60) }
	}

	view := InsightsViewData{
		Ranges:              ranges,
		RangeDays:           rangeDays,
		Metrics:             metrics,
		IsPumpVolumeMetric:  isVolumeMetric,
		PumpRangeLabel:      insights.RangeLabel,
		HasPumpData:         insights.Aggregate.HasAnyData,
		PumpHeroValue:       heroValue,
		PumpHeroCaption:     heroCaption,
		PumpCountBasisLabel: countBasisLabel,
		PumpChartClass:      chartClass,
		PumpChartDays:       chartDays,
		PumpChartAxisGuides: axisGuides(pumpAxisCeiling, axisStep, pumpAxisLabel),
	}

	startsAtFirstRecordedDay := !insights.RangeStartsAtBirth &&
		partialRecordedRange &&
		len(chartSourceDays) > 0 &&
		chartSourceDays[0].HasData

	if trimmedLeadingDays || startsAtFirstRecordedDay {
		view.PumpRangeLabel = pumpInsightsVisibleRangeLabel(chartSourceDays)
		view.RecordsBeginLabel = "Records begin " + chartSourceDays[0].Label
	} else if insights.RangeStartsAtBirth && partialRecordedRange {
		view.RecordsBeginLabel = "Since birth"
	}

	if selectedRaw != nil {
		view.SelectedPumpDay = buildPumpInsightsSelectedDay(*selectedRaw, rangeDays, metric)
	}

	if insights.Aggregate.HasAnyData {
		view.ShowPumpSupportingRow = true
		view.PumpAverageLabel = insights.Aggregate.AveragePerDayLabel
		view.PumpAverageSessionLabel = insights.Aggregate.AverageSessionDurationLabel
		view.PumpAverageSessionCaption = insights.Aggregate.AverageSessionDurationCaption
		view.PumpAverageMlLabel = insights.Aggregate.AverageSessionMlLabel
		view.PumpAverageGapLabel = insights.Aggregate.AverageGapLabel
		view.PumpAverageGapCaption = insights.Aggregate.AverageGapCaption
	}

	return view
}

func pumpChartAriaLabel(day backendclient.PumpInsightDay, isVolumeMetric bool) string {
	if !day.HasData {
		return day.FullLabel + ": no pumping sessions recorded"
	}
	if isVolumeMetric {
		return fmt.Sprintf("%s: %d ml pumped", day.FullLabel, day.TotalMl)
	}
	if day.DurationLabel == "" {
		return day.FullLabel + ": pumping duration not recorded"
	}
	return fmt.Sprintf("%s: %s total recorded pumping time", day.FullLabel, day.DurationLabel)
}

func pumpInsightsVisibleRangeLabel(days []backendclient.PumpInsightDay) string {
	if len(days) == 0 {
		return ""
	}
	first, last := days[0].Label, days[len(days)-1].Label
	if first == last {
		return first
	}
	return first + " – " + last
}

// pumpChartValue picks which of a day's two metric totals the chart bar
// represents — total volume in ml or total duration in minutes — matching
// whichever metric sub-tab is active.
func pumpChartValue(day backendclient.PumpInsightDay, isVolumeMetric bool) int {
	if isVolumeMetric {
		return day.TotalMl
	}
	return day.TotalMinutes
}

// pumpBarPercent turns a day's pump total into a chart bar height, giving
// any positive value a visibly solid minimum bar so it never looks like the
// hatched "no data" placeholder — the Pump counterpart of feedBarPercent.
func pumpBarPercent(day backendclient.PumpInsightDay, value, maxVal int) int {
	if !day.HasData || value <= 0 {
		return 0
	}
	pct := int(math.Round(float64(value) / float64(maxVal) * 100))
	if pct < 4 {
		pct = 4
	}
	return pct
}

func buildPumpInsightsSelectedDay(day backendclient.PumpInsightDay, rangeDays int, metric string) *InsightsSelectedPumpDay {
	rows := make([]InsightsPumpEventRow, len(day.Events))
	for i, ev := range day.Events {
		rows[i] = InsightsPumpEventRow{TimeLabel: ev.TimeLabel, DurationLabel: ev.DurationLabel, VolumeLabel: ev.VolumeLabel}
	}

	return &InsightsSelectedPumpDay{
		FullLabel:     day.FullLabel,
		SessionsLabel: strconv.Itoa(day.SessionCount),
		DurationLabel: emptyDash(day.DurationLabel),
		VolumeLabel:   fmt.Sprintf("%d ml", day.TotalMl),
		Events:        rows,
		HasEvents:     len(rows) > 0,
		CloseHref:     pumpInsightsHref(rangeDays, metric, ""),
	}
}
