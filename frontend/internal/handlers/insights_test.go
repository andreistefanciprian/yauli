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

func TestBuildInsightsViewKeepsLeadingEmptyDaysInSevenDayChart(t *testing.T) {
	insights := backendclient.SleepInsights{
		RangeLabel: "Jul 1 – Jul 2",
		Days: []backendclient.SleepInsightDay{
			{LocalDate: "2026-07-01"},
			{LocalDate: "2026-07-02", HasData: true, TotalMinutes: 120},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true},
	}

	view := buildInsightsView(insights, 7, "")

	if len(view.ChartDays) != 2 || view.ChartDays[0].Key != "2026-07-01" {
		t.Fatalf("ChartDays = %#v, want the complete seven-day chart unchanged", view.ChartDays)
	}
	if strings.Contains(view.ChartClass, "insights-chart-adaptive") {
		t.Fatalf("ChartClass = %q, want seven-day chart sizing unchanged", view.ChartClass)
	}
	if view.RecordsBeginLabel != "" {
		t.Fatalf("RecordsBeginLabel = %q, want no start note for the complete chart", view.RecordsBeginLabel)
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

func TestBuildInsightsViewKeepsBirthDateAsFirstLongRangeDay(t *testing.T) {
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

	if len(view.ChartDays) != len(insights.Days) || view.ChartDays[0].Key != "2026-06-30" {
		t.Fatalf("ChartDays = %#v, want birth date retained as first day", view.ChartDays)
	}
	if view.RangeLabel != "Jun 30 – Jul 5" {
		t.Fatalf("RangeLabel = %q, want birth-clamped range", view.RangeLabel)
	}
	if view.RecordsBeginLabel != "Since birth" {
		t.Fatalf("RecordsBeginLabel = %q, want birth context", view.RecordsBeginLabel)
	}
	if !strings.Contains(view.ChartClass, "insights-chart-adaptive") {
		t.Fatalf("ChartClass = %q, want shortened birth range to use available width", view.ChartClass)
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
				Periods: []backendclient.SleepInsightPeriod{
					{Type: "night", TimeRangeLabel: "8:00 PM – 6:00 AM", DurationLabel: "10 hr"},
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
}

func TestBuildGrowthChartPointsScalesHorizontalPositionByElapsedTime(t *testing.T) {
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	points := []backendclient.GrowthInsightPoint{
		{ID: "first", OccurredAt: start, Value: 4.0},
		{ID: "second", OccurredAt: start.AddDate(0, 0, 1), Value: 4.1},
		{ID: "third", OccurredAt: start.AddDate(0, 0, 10), Value: 4.3},
	}

	chartPoints, _ := buildGrowthChartPoints(points, "", 90, "weight")

	if chartPoints[0].CX != "20.0" || chartPoints[1].CX != "76.0" || chartPoints[2].CX != "580.0" {
		t.Fatalf("chart x positions = %q, %q, %q, want elapsed-time scale", chartPoints[0].CX, chartPoints[1].CX, chartPoints[2].CX)
	}
	if chartPoints[1].LeftPercent != "12.67" {
		t.Fatalf("middle LeftPercent = %q, want label and hit target aligned to chart point", chartPoints[1].LeftPercent)
	}
}
