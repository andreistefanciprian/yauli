package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

func TestInsightsRangeFromQuery(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want int
	}{
		{name: "defaults to 30", url: "/insights", want: 30},
		{name: "accepts 7", url: "/insights?range=7", want: 7},
		{name: "accepts 90", url: "/insights?range=90", want: 90},
		{name: "rejects unknown range", url: "/insights?range=14", want: 30},
		{name: "rejects garbage", url: "/insights?range=abc", want: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			if got := insightsRangeFromQuery(req); got != tt.want {
				t.Fatalf("insightsRangeFromQuery(%q) = %d, want %d", tt.url, got, tt.want)
			}
		})
	}
}

func TestBuildInsightsViewNoData(t *testing.T) {
	view := buildInsightsView(backendclient.SleepInsights{
		RangeLabel: "Jun 28 – Jul 27",
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-07-27"},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: false},
	}, 30, "")

	if view.HasAnyData {
		t.Fatalf("HasAnyData = true, want false")
	}
	if view.HeroLabel != "—" {
		t.Fatalf("HeroLabel = %q, want em dash fallback", view.HeroLabel)
	}
	if view.ShowSupportingRow {
		t.Fatalf("ShowSupportingRow = true, want false when there is no data")
	}
	if view.ShowObservations {
		t.Fatalf("ShowObservations = true, want false when there is no data")
	}
	if len(view.ChartDays) != 1 || view.ChartDays[0].HasData {
		t.Fatalf("ChartDays = %#v, want one day with HasData=false", view.ChartDays)
	}
}

func TestBuildInsightsViewTrimsOnlyLeadingEmptyDaysFromLongCharts(t *testing.T) {
	insights := backendclient.SleepInsights{
		RangeLabel: "Jul 1 – Jul 6",
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2"},
			{LocalDate: "2026-07-03", Label: "Jul 3", HasData: true, TotalMinutes: 120},
			{LocalDate: "2026-07-04", Label: "Jul 4", ShowLabel: true},
			{LocalDate: "2026-07-05", Label: "Jul 5", HasData: true, TotalMinutes: 90},
			{LocalDate: "2026-07-06", Label: "Jul 6"},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}

	for _, test := range []struct {
		name      string
		rangeDays int
	}{
		{name: "30 days", rangeDays: 30},
		{name: "90 days", rangeDays: 90},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := buildInsightsView(insights, test.rangeDays, "")

			if len(view.ChartDays) != 4 {
				t.Fatalf("len(ChartDays) = %d, want four visible days", len(view.ChartDays))
			}
			if view.ChartDays[0].Key != "2026-07-03" || view.ChartDays[3].Key != "2026-07-06" {
				t.Fatalf("ChartDays = %#v, want range from first recorded day through the original final day", view.ChartDays)
			}
			if view.ChartDays[1].HasData || view.ChartDays[3].HasData {
				t.Fatalf("empty days = %#v, %#v, want interior and trailing gaps preserved", view.ChartDays[1], view.ChartDays[3])
			}
			if !view.ChartDays[0].ShowLabel || !view.ChartDays[3].ShowLabel {
				t.Fatalf("visible edge labels = %v, %v, want both shown", view.ChartDays[0].ShowLabel, view.ChartDays[3].ShowLabel)
			}
			if !view.ChartDays[1].ShowLabel {
				t.Fatal("existing interior axis label was removed")
			}
			if !strings.Contains(view.ChartClass, "insights-chart-adaptive") {
				t.Fatalf("ChartClass = %q, want short long-range chart to expand", view.ChartClass)
			}
			if view.RecordsBeginLabel != "Records begin Jul 3" {
				t.Fatalf("RecordsBeginLabel = %q, want first recorded date", view.RecordsBeginLabel)
			}
			if view.RangeLabel != "Jul 3 – Jul 6" {
				t.Fatalf("RangeLabel = %q, want visible chart range", view.RangeLabel)
			}
		})
	}
}

func TestBuildInsightsViewTrimsLeadingEmptyDaysFromSevenDayChart(t *testing.T) {
	insights := backendclient.SleepInsights{
		RangeLabel: "Jul 1 – Jul 3",
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2", HasData: true, TotalMinutes: 120},
			{LocalDate: "2026-07-03", Label: "Jul 3"},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}

	view := buildInsightsView(insights, 7, "")

	if len(view.ChartDays) != 2 || view.ChartDays[0].Key != "2026-07-02" || view.ChartDays[1].Key != "2026-07-03" {
		t.Fatalf("ChartDays = %#v, want first recorded day through the original final day", view.ChartDays)
	}
	if view.ChartDays[1].HasData {
		t.Fatalf("ChartDays[1] = %#v, want trailing empty day preserved", view.ChartDays[1])
	}
	if strings.Contains(view.ChartClass, "insights-chart-adaptive") {
		t.Fatalf("ChartClass = %q, want seven-day chart sizing unchanged", view.ChartClass)
	}
	if view.RecordsBeginLabel != "Records begin Jul 2" {
		t.Fatalf("RecordsBeginLabel = %q, want first recorded date", view.RecordsBeginLabel)
	}
	if view.RangeLabel != "Jul 2 – Jul 3" {
		t.Fatalf("RangeLabel = %q, want visible chart range", view.RangeLabel)
	}
}

func TestBuildInsightsViewExpandsShortLongRangeWithoutAddingDays(t *testing.T) {
	insights := backendclient.SleepInsights{
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-07-03", Label: "Jul 3", HasData: true, TotalMinutes: 120},
			{LocalDate: "2026-07-04", Label: "Jul 4"},
			{LocalDate: "2026-07-05", Label: "Jul 5", HasData: true, TotalMinutes: 90},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}

	view := buildInsightsView(insights, 30, "")

	if len(view.ChartDays) != len(insights.Days) {
		t.Fatalf("len(ChartDays) = %d, want only the %d supplied calendar days", len(view.ChartDays), len(insights.Days))
	}
	if !strings.Contains(view.ChartClass, "insights-chart-adaptive") {
		t.Fatalf("ChartClass = %q, want short chart to expand", view.ChartClass)
	}
	if view.RecordsBeginLabel != "Records begin Jul 3" {
		t.Fatalf("RecordsBeginLabel = %q, want first available date", view.RecordsBeginLabel)
	}
}

func TestBuildInsightsViewTrimsLeadingEmptyBirthDays(t *testing.T) {
	insights := backendclient.SleepInsights{
		RangeLabel:         "Jun 30 – Jul 5",
		RangeStartsAtBirth: true,
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-06-30", Label: "Jun 30"},
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2", HasData: true, TotalMinutes: 120},
			{LocalDate: "2026-07-03", Label: "Jul 3"},
			{LocalDate: "2026-07-04", Label: "Jul 4", HasData: true, TotalMinutes: 90},
			{LocalDate: "2026-07-05", Label: "Jul 5"},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}

	view := buildInsightsView(insights, 30, "")

	if len(view.ChartDays) != 4 || view.ChartDays[0].Key != "2026-07-02" || view.ChartDays[3].Key != "2026-07-05" {
		t.Fatalf("ChartDays = %#v, want first recorded day through the original final day", view.ChartDays)
	}
	if view.ChartDays[1].HasData || view.ChartDays[3].HasData {
		t.Fatalf("empty days = %#v, %#v, want interior and trailing gaps preserved", view.ChartDays[1], view.ChartDays[3])
	}
	if view.RangeLabel != "Jul 2 – Jul 5" {
		t.Fatalf("RangeLabel = %q, want visible chart range", view.RangeLabel)
	}
	if view.RecordsBeginLabel != "Records begin Jul 2" {
		t.Fatalf("RecordsBeginLabel = %q, want first recorded date", view.RecordsBeginLabel)
	}
	if !strings.Contains(view.ChartClass, "insights-chart-adaptive") {
		t.Fatalf("ChartClass = %q, want trimmed birth range to use available width", view.ChartClass)
	}
}

func TestBuildInsightsViewKeepsBirthContextWhenBirthDayHasData(t *testing.T) {
	view := buildInsightsView(backendclient.SleepInsights{
		RangeLabel:         "Jun 30 – Jul 1",
		RangeStartsAtBirth: true,
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-06-30", Label: "Jun 30", HasData: true, TotalMinutes: 120},
			{LocalDate: "2026-07-01", Label: "Jul 1"},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}, 30, "")

	if len(view.ChartDays) != 2 || view.ChartDays[0].Key != "2026-06-30" {
		t.Fatalf("ChartDays = %#v, want recorded birth date retained as first day", view.ChartDays)
	}
	if view.RangeLabel != "Jun 30 – Jul 1" {
		t.Fatalf("RangeLabel = %q, want birth-clamped range", view.RangeLabel)
	}
	if view.RecordsBeginLabel != "Since birth" {
		t.Fatalf("RecordsBeginLabel = %q, want birth context", view.RecordsBeginLabel)
	}
}

func TestBuildInsightsViewSelectsDayAndTogglesHref(t *testing.T) {
	insights := backendclient.SleepInsights{
		Days: []backendclient.SleepInsightDay{
			{
				LocalDate:      "2026-07-26",
				HasData:        true,
				TotalMinutes:   480,
				CompletedCount: 1,
				NightMinutes:   480,
				FullLabel:      "Sunday, July 26",
				TotalLabel:     "8 hr",
				CarryoverNote:  "1 listed sleep period started the previous day.",
				Periods: []backendclient.SleepInsightPeriod{
					{Type: "night", StartedPreviousDay: true, TimeRangeLabel: "Previous day – 6:00 AM", DurationLabel: "6 hr"},
				},
			},
			{LocalDate: "2026-07-27", HasData: false},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true, RecordedDays: 1, AverageTotalLabel: "8 hr"},
	}

	// No day selected: every bar's href should select itself.
	view := buildInsightsView(insights, 7, "")
	if view.SelectedDay != nil {
		t.Fatalf("SelectedDay = %#v, want nil when no day param is set", view.SelectedDay)
	}
	if view.ChartDays[0].Href != "/insights?range=7&day=2026-07-26" {
		t.Fatalf("unselected day href = %q", view.ChartDays[0].Href)
	}
	if view.ChartDays[0].Selected {
		t.Fatalf("ChartDays[0].Selected = true, want false")
	}
	if view.AverageBasisLabel != "Based on 1 recorded day" {
		t.Fatalf("AverageBasisLabel = %q, want singular recorded-day basis", view.AverageBasisLabel)
	}

	// Day selected: that bar's href should clear the selection, and the
	// selected-day panel should be populated from its periods.
	view = buildInsightsView(insights, 7, "2026-07-26")
	if view.SelectedDay == nil {
		t.Fatalf("SelectedDay = nil, want populated panel")
	}
	if view.SelectedDay.FullLabel != "Sunday, July 26" || view.SelectedDay.TotalLabel != "8 hr" {
		t.Fatalf("SelectedDay = %#v", view.SelectedDay)
	}
	if !view.SelectedDay.HasPeriods || len(view.SelectedDay.Periods) != 1 {
		t.Fatalf("SelectedDay.Periods = %#v, want one row", view.SelectedDay.Periods)
	}
	if view.SelectedDay.Periods[0].Tag != "Night" || view.SelectedDay.Periods[0].TagClass != "night" {
		t.Fatalf("SelectedDay.Periods[0] = %#v", view.SelectedDay.Periods[0])
	}
	if !view.SelectedDay.Periods[0].Boundary {
		t.Fatalf("SelectedDay.Periods[0].Boundary = false, want true")
	}
	if view.SelectedDay.BoundaryNote != insightsSleepBoundaryFootnote {
		t.Fatalf("SelectedDay.BoundaryNote = %q, want calendar-boundary footnote", view.SelectedDay.BoundaryNote)
	}
	if !view.ChartDays[0].Selected {
		t.Fatalf("ChartDays[0].Selected = false, want true")
	}
	if view.ChartDays[0].Href != "/insights?range=7" {
		t.Fatalf("selected day href = %q, want the clearing href", view.ChartDays[0].Href)
	}
}

func TestBuildInsightsViewSupportingRowAndNapNightFallback(t *testing.T) {
	insights := backendclient.SleepInsights{
		Aggregate: backendclient.SleepInsightAggregate{
			HasAnyData:            true,
			RecordedDays:          18,
			AverageTotalLabel:     "7 hr 30 min",
			AverageCompletedLabel: "2.5",
			// LongestOverallLabel and nap/night percent deliberately absent,
			// as backend-api leaves them when there isn't enough data.
			AverageWakeWindowLabel:   "Not yet available",
			AverageWakeWindowCaption: "Needs more recorded sleep periods",
		},
		Observations: []string{"Not enough recorded sleep yet to calculate an average wake window."},
	}

	view := buildInsightsView(insights, 30, "")

	if !view.ShowSupportingRow {
		t.Fatalf("ShowSupportingRow = false, want true")
	}
	if view.AverageBasisLabel != "Based on 18 recorded days" {
		t.Fatalf("AverageBasisLabel = %q, want recorded-day denominator", view.AverageBasisLabel)
	}
	if view.LongestOverallLabel != "—" {
		t.Fatalf("LongestOverallLabel = %q, want em dash fallback", view.LongestOverallLabel)
	}
	if view.ShowNapNight {
		t.Fatalf("ShowNapNight = true, want false when nap/night percentages are absent")
	}
	if !view.ShowObservations || len(view.Observations) != 1 {
		t.Fatalf("Observations = %#v, want the single fallback sentence", view.Observations)
	}
}

func TestBuildInsightsSelectedDayAddsBoundaryFootnoteForEitherBoundary(t *testing.T) {
	tests := []struct {
		name         string
		period       backendclient.SleepInsightPeriod
		wantFootnote bool
	}{
		{
			name:         "started previous day",
			period:       backendclient.SleepInsightPeriod{StartedPreviousDay: true},
			wantFootnote: true,
		},
		{
			name:         "continues next day",
			period:       backendclient.SleepInsightPeriod{ContinuesNextDay: true},
			wantFootnote: true,
		},
		{
			name:   "within selected day",
			period: backendclient.SleepInsightPeriod{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := buildInsightsSelectedDay(backendclient.SleepInsightDay{
				Periods: []backendclient.SleepInsightPeriod{tt.period},
			}, 7)
			if got := selected.BoundaryNote != ""; got != tt.wantFootnote {
				t.Fatalf("BoundaryNote = %q, presence = %v; want presence %v", selected.BoundaryNote, got, tt.wantFootnote)
			}
			if got := selected.Periods[0].Boundary; got != tt.wantFootnote {
				t.Fatalf("Periods[0].Boundary = %v, want %v", got, tt.wantFootnote)
			}
		})
	}
}

func TestBarPercentGivesNoDataDaysZeroHeight(t *testing.T) {
	if got := barPercent(backendclient.SleepInsightDay{HasData: false, TotalMinutes: 999}, 999); got != 0 {
		t.Fatalf("barPercent for a no-data day = %d, want 0", got)
	}
}

func TestBarPercentEnforcesMinimumForTinyValues(t *testing.T) {
	got := barPercent(backendclient.SleepInsightDay{HasData: true, TotalMinutes: 1}, 480)
	if got != 4 {
		t.Fatalf("barPercent = %d, want the 4%% floor for a barely-recorded day", got)
	}
}

func TestSplitPercentsSumToOneHundredAfterRounding(t *testing.T) {
	night, nap := splitPercents(1, 8)
	if night != 13 || nap != 87 {
		t.Fatalf("splitPercents(1, 8) = %d, %d, want 13, 87", night, nap)
	}

	night, nap = splitPercents(0, 0)
	if night != 0 || nap != 0 {
		t.Fatalf("splitPercents(0, 0) = %d, %d, want 0, 0", night, nap)
	}
}

func TestBuildGrowthInsightsViewSelectsSameDayMeasurementByID(t *testing.T) {
	firstAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	secondAt := firstAt.Add(2 * time.Hour)
	insights := backendclient.GrowthInsights{
		MetricLabel: "Weight",
		HasAnyData:  true,
		Points: []backendclient.GrowthInsightPoint{
			{
				ID:          "first",
				OccurredAt:  firstAt,
				LocalDate:   "2026-07-20",
				FullLabel:   "Monday, July 20",
				Value:       4.1,
				ValueLabel:  "4.100 kg",
				ChangeLabel: "First recorded measurement",
			},
			{
				ID:          "second",
				OccurredAt:  secondAt,
				LocalDate:   "2026-07-20",
				FullLabel:   "Monday, July 20",
				Value:       4.2,
				ValueLabel:  "4.200 kg",
				ChangeLabel: "+0.100 kg",
			},
		},
		Aggregate: backendclient.GrowthInsightAggregate{LatestValueLabel: "4.200 kg"},
	}

	view := buildGrowthInsightsView(insights, 90, "weight", "second")

	if view.SelectedGrowthPoint == nil || view.SelectedGrowthPoint.ValueLabel != "4.200 kg" {
		t.Fatalf("SelectedGrowthPoint = %#v, want second same-day measurement", view.SelectedGrowthPoint)
	}
	if view.GrowthChartPoints[0].Selected || !view.GrowthChartPoints[1].Selected {
		t.Fatalf("selected chart points = %v, %v, want only second selected", view.GrowthChartPoints[0].Selected, view.GrowthChartPoints[1].Selected)
	}
	if !strings.Contains(view.GrowthChartPoints[0].Href, "point=first") {
		t.Fatalf("first point href = %q, want stable event ID", view.GrowthChartPoints[0].Href)
	}
	if len(view.GrowthAxisGuides) != 2 ||
		view.GrowthAxisGuides[0].Label != "4.200 kg" ||
		view.GrowthAxisGuides[1].Label != "4.100 kg" {
		t.Fatalf("GrowthAxisGuides = %#v, want max then min value labels", view.GrowthAxisGuides)
	}
}

func TestBuildGrowthChartPointsScalesHorizontalPositionByElapsedTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	points := []backendclient.GrowthInsightPoint{
		{ID: "first", OccurredAt: start, Value: 4.0},
		{ID: "second", OccurredAt: start.AddDate(0, 0, 1), Value: 4.1},
		{ID: "third", OccurredAt: start.AddDate(0, 0, 10), Value: 4.3},
	}

	chartPoints, _ := buildGrowthChartPoints(points, nil, time.Time{}, "", 90, "weight")

	if chartPoints[0].CX != "20.0" || chartPoints[1].CX != "76.0" || chartPoints[2].CX != "580.0" {
		t.Fatalf("chart x positions = %q, %q, %q, want elapsed-time scale", chartPoints[0].CX, chartPoints[1].CX, chartPoints[2].CX)
	}
	if chartPoints[1].LeftPercent != "12.67" {
		t.Fatalf("middle LeftPercent = %q, want label and hit target aligned to chart point", chartPoints[1].LeftPercent)
	}
	if chartPoints[0].CalloutClass != "" || chartPoints[1].CalloutClass != "" || chartPoints[2].CalloutClass != "" {
		t.Fatalf("callout classes = %q, %q, %q; min/max endpoints should use axis labels only", chartPoints[0].CalloutClass, chartPoints[1].CalloutClass, chartPoints[2].CalloutClass)
	}
}

func TestBuildGrowthChartPointsLabelsEndpointsNotShownOnAxis(t *testing.T) {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	points := []backendclient.GrowthInsightPoint{
		{ID: "first", OccurredAt: start, Value: 5.0},
		{ID: "minimum", OccurredAt: start.AddDate(0, 0, 1), Value: 4.8},
		{ID: "maximum", OccurredAt: start.AddDate(0, 0, 2), Value: 6.1},
		{ID: "last", OccurredAt: start.AddDate(0, 0, 3), Value: 5.9},
	}

	chartPoints, _ := buildGrowthChartPoints(points, nil, time.Time{}, "", 90, "weight")

	if chartPoints[0].CalloutClass != "first" || chartPoints[3].CalloutClass != "last" {
		t.Fatalf("endpoint callout classes = %q, %q; want first and last", chartPoints[0].CalloutClass, chartPoints[3].CalloutClass)
	}
	if chartPoints[1].CalloutClass != "" || chartPoints[2].CalloutClass != "" {
		t.Fatalf("interior callout classes = %q, %q; want none", chartPoints[1].CalloutClass, chartPoints[2].CalloutClass)
	}
}

func TestBuildGrowthChartPointsUsesEffectiveRangeBounds(t *testing.T) {
	rangeStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	points := []backendclient.GrowthInsightPoint{
		{ID: "first", OccurredAt: time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), Value: 4.0},
		{ID: "second", OccurredAt: time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), Value: 4.3},
	}

	chartPoints, _ := buildGrowthChartPoints(points, &rangeStart, rangeEnd, "", 90, "weight")

	if chartPoints[0].CX != "231.3" || chartPoints[1].CX != "434.9" {
		t.Fatalf("chart x positions = %q, %q, want points placed within the birth-to-today range", chartPoints[0].CX, chartPoints[1].CX)
	}
}

func TestBuildGrowthChartPointsPositionsSinglePointWithinEffectiveRange(t *testing.T) {
	rangeStart := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	rangeEnd := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	points := []backendclient.GrowthInsightPoint{
		{ID: "only", OccurredAt: time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC), Value: 4.0, ValueLabel: "4.0 kg"},
	}

	chartPoints, _ := buildGrowthChartPoints(points, &rangeStart, rangeEnd, "", 90, "weight")

	if chartPoints[0].CX != "231.3" {
		t.Fatalf("single point x position = %q, want placement within the effective range", chartPoints[0].CX)
	}
	if chartPoints[0].CalloutClass != "first" {
		t.Fatalf("single point CalloutClass = %q, want first-point label only", chartPoints[0].CalloutClass)
	}
	guides := buildGrowthAxisGuides(points)
	if len(guides) != 0 {
		t.Fatalf("single-point guides = %#v, want endpoint callout as the only visible value label", guides)
	}
	if emptyGuides := buildGrowthAxisGuides(nil); len(emptyGuides) != 0 {
		t.Fatalf("no-data guides = %#v, want none", emptyGuides)
	}
}

func TestBuildGrowthAxisGuidesUsesFormattedMinAndMaxLabels(t *testing.T) {
	points := []backendclient.GrowthInsightPoint{
		{Value: 5100, ValueLabel: "5.1 kg"},
		{Value: 6100, ValueLabel: "6.1 kg"},
		{Value: 4800, ValueLabel: "4.8 kg"},
	}

	guides := buildGrowthAxisGuides(points)

	if len(guides) != 2 {
		t.Fatalf("len(guides) = %d, want max and min guides", len(guides))
	}
	if guides[0].Label != "6.1 kg" || guides[0].Y != "20.0" || guides[0].TopPercent != "12.50" {
		t.Fatalf("max guide = %#v, want top-gridline max label", guides[0])
	}
	if guides[1].Label != "4.8 kg" || guides[1].Y != "140.0" || guides[1].TopPercent != "87.50" {
		t.Fatalf("min guide = %#v, want bottom-gridline min label", guides[1])
	}
}

func TestGrowthChartLabelsPreserveMetricUnits(t *testing.T) {
	start := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		metric    string
		first     backendclient.GrowthInsightPoint
		last      backendclient.GrowthInsightPoint
		wantFirst string
		wantLast  string
	}{
		{
			name:      "weight",
			metric:    "weight",
			first:     backendclient.GrowthInsightPoint{ID: "first", OccurredAt: start, Value: 4800, ValueLabel: "4.8 kg"},
			last:      backendclient.GrowthInsightPoint{ID: "last", OccurredAt: start.AddDate(0, 0, 7), Value: 5100, ValueLabel: "5.1 kg"},
			wantFirst: "4.8 kg",
			wantLast:  "5.1 kg",
		},
		{
			name:      "length",
			metric:    "length",
			first:     backendclient.GrowthInsightPoint{ID: "first", OccurredAt: start, Value: 49.2, ValueLabel: "49.2 cm"},
			last:      backendclient.GrowthInsightPoint{ID: "last", OccurredAt: start.AddDate(0, 0, 7), Value: 52.1, ValueLabel: "52.1 cm"},
			wantFirst: "49.2 cm",
			wantLast:  "52.1 cm",
		},
		{
			name:      "head circumference",
			metric:    "head_circumference",
			first:     backendclient.GrowthInsightPoint{ID: "first", OccurredAt: start, Value: 34.0, ValueLabel: "34 cm"},
			last:      backendclient.GrowthInsightPoint{ID: "last", OccurredAt: start.AddDate(0, 0, 7), Value: 36.2, ValueLabel: "36.2 cm"},
			wantFirst: "34 cm",
			wantLast:  "36.2 cm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			points := []backendclient.GrowthInsightPoint{tt.first, tt.last}
			chartPoints, _ := buildGrowthChartPoints(points, nil, time.Time{}, "", 90, tt.metric)
			guides := buildGrowthAxisGuides(points)

			if chartPoints[0].ValueLabel != tt.wantFirst || chartPoints[1].ValueLabel != tt.wantLast {
				t.Fatalf("callout labels = %q, %q; want %q, %q", chartPoints[0].ValueLabel, chartPoints[1].ValueLabel, tt.wantFirst, tt.wantLast)
			}
			if guides[0].Label != tt.wantLast || guides[1].Label != tt.wantFirst {
				t.Fatalf("axis labels = %q, %q; want %q, %q", guides[0].Label, guides[1].Label, tt.wantLast, tt.wantFirst)
			}
		})
	}
}

func TestBuildNappyInsightsViewNoData(t *testing.T) {
	view := buildNappyInsightsView(backendclient.NappyInsights{
		RangeLabel: "Jun 28 – Jul 27",
		Days: []backendclient.NappyInsightDay{
			{LocalDate: "2026-07-27"},
		},
		Aggregate: backendclient.NappyInsightAggregate{HasAnyData: false},
	}, 30, "")

	if view.HasNappyData {
		t.Fatalf("HasNappyData = true, want false")
	}
	if view.NappyHeroValue != "0" {
		t.Fatalf("NappyHeroValue = %q, want 0 with no data", view.NappyHeroValue)
	}
	if view.ShowNappySupportingRow {
		t.Fatalf("ShowNappySupportingRow = true, want false when there is no data")
	}
	if view.ShowNappyBreakdown {
		t.Fatalf("ShowNappyBreakdown = true, want false when there is no data")
	}
	if view.ShowObservations {
		t.Fatalf("ShowObservations = true, want false when there is no data")
	}
	if len(view.NappyChartDays) != 1 || view.NappyChartDays[0].HasData {
		t.Fatalf("NappyChartDays = %#v, want one day with HasData=false", view.NappyChartDays)
	}
}

func TestBuildNappyInsightsViewTrimsLeadingEmptyDays(t *testing.T) {
	insights := backendclient.NappyInsights{
		RangeLabel: "Jul 1 – Jul 6",
		Days: []backendclient.NappyInsightDay{
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2"},
			{LocalDate: "2026-07-03", Label: "Jul 3", HasData: true, TotalCount: 6, WeeCount: 4, PooCount: 2},
			{LocalDate: "2026-07-04", Label: "Jul 4"},
			{LocalDate: "2026-07-05", Label: "Jul 5", HasData: true, TotalCount: 5, WeeCount: 3, PooCount: 2},
			{LocalDate: "2026-07-06", Label: "Jul 6"},
		},
		Aggregate: backendclient.NappyInsightAggregate{HasAnyData: true, TotalCount: 11},
	}

	view := buildNappyInsightsView(insights, 30, "")

	if len(view.NappyChartDays) != 4 {
		t.Fatalf("len(NappyChartDays) = %d, want four visible days", len(view.NappyChartDays))
	}
	if view.NappyChartDays[0].Key != "2026-07-03" || view.NappyChartDays[3].Key != "2026-07-06" {
		t.Fatalf("NappyChartDays = %#v, want range from first recorded day through the original final day", view.NappyChartDays)
	}
	if view.RecordsBeginLabel != "Records begin Jul 3" {
		t.Fatalf("RecordsBeginLabel = %q, want Records begin Jul 3", view.RecordsBeginLabel)
	}
}

func TestBuildNappyInsightsViewSelectedDay(t *testing.T) {
	insights := backendclient.NappyInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.NappyInsightDay{
			{
				LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 3, WeeCount: 1, PooCount: 1, MixedCount: 1,
				Events: []backendclient.NappyInsightEvent{
					{Kind: "wee", TimeLabel: "8:00 AM"},
					{Kind: "poo", TimeLabel: "11:00 AM"},
					{Kind: "mixed", TimeLabel: "2:00 PM"},
				},
			},
		},
		Aggregate: backendclient.NappyInsightAggregate{
			HasAnyData: true, TotalCount: 3,
			AveragePerDayLabel: "3.0",
			HasAverageGap:      true,
			AverageGapLabel:    "3h 0m",
			AverageGapCaption:  "Avg. time between changes",
			WeePercent:         intPtrFor(33),
			PooPercent:         intPtrFor(33),
			MixedPercent:       intPtrFor(34),
		},
	}

	view := buildNappyInsightsView(insights, 7, "2026-07-20")

	if view.SelectedNappyDay == nil {
		t.Fatalf("SelectedNappyDay = nil, want a selected day")
	}
	if view.SelectedNappyDay.TotalLabel != "3" || view.SelectedNappyDay.WeeLabel != "1" || view.SelectedNappyDay.PooLabel != "1" || view.SelectedNappyDay.MixedLabel != "1" {
		t.Fatalf("SelectedNappyDay counts = %#v, want 3/1/1/1", view.SelectedNappyDay)
	}
	if len(view.SelectedNappyDay.Events) != 3 {
		t.Fatalf("len(Events) = %d, want 3", len(view.SelectedNappyDay.Events))
	}
	if view.SelectedNappyDay.Events[1].Tag != "Poo" || view.SelectedNappyDay.Events[1].TagClass != "poo" {
		t.Fatalf("Events[1] = %#v, want Poo/poo", view.SelectedNappyDay.Events[1])
	}
	if view.SelectedNappyDay.Events[2].Tag != "Wee & Poo" || view.SelectedNappyDay.Events[2].TagClass != "mixed" {
		t.Fatalf("Events[2] = %#v, want Wee & Poo/mixed", view.SelectedNappyDay.Events[2])
	}
	if !view.ShowNappyBreakdown || view.NappyWeePercent != 33 || view.NappyPooPercent != 33 || view.NappyMixedPercent != 34 {
		t.Fatalf("breakdown = show=%v wee=%d poo=%d mixed=%d, want 33/33/34", view.ShowNappyBreakdown, view.NappyWeePercent, view.NappyPooPercent, view.NappyMixedPercent)
	}
	if view.NappyAverageGapLabel != "3h 0m" {
		t.Fatalf("NappyAverageGapLabel = %q, want 3h 0m", view.NappyAverageGapLabel)
	}
}

func TestNappySplitPercentsStayValidAfterRounding(t *testing.T) {
	tests := []struct {
		name                        string
		day                         backendclient.NappyInsightDay
		wantWee, wantPoo, wantMixed int
	}{
		{
			name:      "two categories do not make an absent category negative",
			day:       backendclient.NappyInsightDay{TotalCount: 8, WeeCount: 1, PooCount: 7},
			wantWee:   13,
			wantPoo:   87,
			wantMixed: 0,
		},
		{
			name:      "equal thirds allocate the rounding remainder once",
			day:       backendclient.NappyInsightDay{TotalCount: 3, WeeCount: 1, PooCount: 1, MixedCount: 1},
			wantWee:   34,
			wantPoo:   33,
			wantMixed: 33,
		},
		{
			name: "no data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wee, poo, mixed := nappySplitPercents(tt.day)
			if wee != tt.wantWee || poo != tt.wantPoo || mixed != tt.wantMixed {
				t.Fatalf("nappySplitPercents(%#v) = %d, %d, %d; want %d, %d, %d",
					tt.day,
					wee, poo, mixed,
					tt.wantWee, tt.wantPoo, tt.wantMixed,
				)
			}
		})
	}
}

func intPtrFor(v int) *int {
	return &v
}
