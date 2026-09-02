package handlers

import (
	"net/http/httptest"
	"strconv"
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

func TestInsightsCategoryFromQuery(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "defaults to overview", url: "/insights", want: insightsCategoryOverview},
		{name: "accepts explicit sleep", url: "/insights?category=sleep", want: insightsCategorySleep},
		{name: "accepts explicit overview", url: "/insights?category=overview", want: insightsCategoryOverview},
		{name: "unknown category falls back to overview", url: "/insights?category=unknown", want: insightsCategoryOverview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			if got := insightsCategoryFromQuery(req); got != tt.want {
				t.Fatalf("insightsCategoryFromQuery(%q) = %q, want %q", tt.url, got, tt.want)
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
	if len(view.ChartDays) != 1 || view.ChartDays[0].HasData {
		t.Fatalf("ChartDays = %#v, want one day with HasData=false", view.ChartDays)
	}
}

func TestBuildInsightsViewOngoingOnlySleepIsRecordedWithoutDuration(t *testing.T) {
	view := buildInsightsView(backendclient.SleepInsights{
		RangeLabel: "Jul 20",
		Days: []backendclient.SleepInsightDay{
			{
				LocalDate: "2026-07-20", Label: "Mon", FullLabel: "Monday, July 20", HasData: true,
				Periods: []backendclient.SleepInsightPeriod{
					{Type: "nap", Ongoing: true, TimeRangeLabel: "8:00 AM – ongoing", DurationLabel: "Ongoing"},
				},
			},
		},
		Aggregate: backendclient.SleepInsightAggregate{
			HasAnyData: true, PeriodCount: 1,
			AverageTotalLabel:      "Not yet available",
			AverageTotalBasisLabel: "Duration recorded for 0 of 1 sleep period",
			AverageCompletedLabel:  "Not yet available",
		},
	}, 7, "2026-07-20")

	if !view.HasAnyData || !view.ShowSupportingRow {
		t.Fatalf("view data flags = any:%v supporting:%v, want recorded activity", view.HasAnyData, view.ShowSupportingRow)
	}
	if view.HeroLabel != "Not yet available" || view.AverageBasisLabel != "Duration recorded for 0 of 1 sleep period" {
		t.Fatalf("sleep hero = %q/%q, want ongoing-only duration disclosure", view.HeroLabel, view.AverageBasisLabel)
	}
	if len(view.ChartDays) != 1 || view.ChartDays[0].HasData {
		t.Fatalf("ChartDays = %#v, want a selectable day without a duration bar", view.ChartDays)
	}
	if view.SelectedDay == nil || view.SelectedDay.TotalLabel != "—" || len(view.SelectedDay.Periods) != 1 || !view.SelectedDay.Periods[0].Ongoing {
		t.Fatalf("SelectedDay = %#v, want the ongoing period without an invented total", view.SelectedDay)
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
	if view.ChartDays[0].Href != "/insights?category=sleep&range=7&day=2026-07-26" {
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
	if view.ChartDays[0].Href != "/insights?category=sleep&range=7" {
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

func TestSleepChartAxisCeilingMinutes(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero rounds up to the 6h floor", 0, 360},
		{"exact 6h mark stays put", 360, 360},
		{"just over 6h rounds up to 12h", 361, 720},
		{"just under 24h rounds up to 24h", 1439, 1440},
		{"caps at 24h even if given more", 2000, 1440},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sleepChartAxisCeilingMinutes(tt.input); got != tt.want {
				t.Fatalf("sleepChartAxisCeilingMinutes(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSleepChartAxisGuides(t *testing.T) {
	guides := sleepChartAxisGuides(720)
	if len(guides) != 2 {
		t.Fatalf("guides = %#v, want 2 marks for a 12h ceiling", guides)
	}
	if guides[0].Label != "6h" || guides[0].Percent != "50.00" {
		t.Fatalf("guides[0] = %#v, want 6h at 50%%", guides[0])
	}
	if guides[1].Label != "12h" || guides[1].Percent != "100.00" {
		t.Fatalf("guides[1] = %#v, want 12h at 100%%", guides[1])
	}

	if fullDay := sleepChartAxisGuides(1440); len(fullDay) != 4 {
		t.Fatalf("fullDay guides = %#v, want all four 6h marks", fullDay)
	}
}

func TestAxisCeiling(t *testing.T) {
	tests := []struct {
		name     string
		maxValue int
		step     int
		want     int
	}{
		{"zero rounds up to one step", 0, 3, 3},
		{"exact multiple stays put", 6, 3, 6},
		{"rounds up to the next multiple", 7, 3, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := axisCeiling(tt.maxValue, tt.step); got != tt.want {
				t.Fatalf("axisCeiling(%d, %d) = %d, want %d", tt.maxValue, tt.step, got, tt.want)
			}
		})
	}
}

func TestBoundedAxisStep(t *testing.T) {
	tests := []struct {
		name        string
		maxValue    int
		minimumStep int
		want        int
	}{
		{"small range keeps minimum step", 6, 3, 3},
		{"busy nappy range increases count step", 13, 3, 6},
		{"busy breast range increases hour step", 360, 60, 120},
		{"busy bottle range increases volume step", 780, 60, 240},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := boundedAxisStep(tt.maxValue, tt.minimumStep, insightsChartMaxAxisGuides); got != tt.want {
				t.Fatalf("boundedAxisStep(%d, %d, %d) = %d, want %d", tt.maxValue, tt.minimumStep, insightsChartMaxAxisGuides, got, tt.want)
			}
		})
	}
}

func TestAxisGuides(t *testing.T) {
	guides := axisGuides(90, 30, func(mark int) string { return strconv.Itoa(mark) })
	if len(guides) != 3 {
		t.Fatalf("guides = %#v, want three 30-step marks", guides)
	}
	if guides[0].Label != "30" || guides[0].Percent != "33.33" {
		t.Fatalf("guides[0] = %#v, want 30 at 33.33%%", guides[0])
	}
	if guides[2].Label != "90" || guides[2].Percent != "100.00" {
		t.Fatalf("guides[2] = %#v, want 90 at 100%%", guides[2])
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
				Label:       "Jul 20",
				ShowLabel:   true,
				FullLabel:   "Monday, July 20",
				Value:       4.1,
				ValueLabel:  "4.100 kg",
				ChangeLabel: "First recorded measurement",
			},
			{
				ID:          "second",
				OccurredAt:  secondAt,
				LocalDate:   "2026-07-20",
				Label:       "Jul 20",
				ShowLabel:   true,
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
	if !view.GrowthChartPoints[0].ShowLabel || view.GrowthChartPoints[1].ShowLabel {
		t.Fatalf("same-day labels = %v/%v, want one visible date label", view.GrowthChartPoints[0].ShowLabel, view.GrowthChartPoints[1].ShowLabel)
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
	if len(view.NappyChartAxisGuides) != 2 {
		t.Fatalf("NappyChartAxisGuides = %#v, want two 3-count marks up to the 6 ceiling", view.NappyChartAxisGuides)
	}
	if got := view.NappyChartAxisGuides[0].Label; got != "3" {
		t.Fatalf("NappyChartAxisGuides[0].Label = %q, want 3", got)
	}
	if got := view.NappyChartAxisGuides[1].Label; got != "6" {
		t.Fatalf("NappyChartAxisGuides[1].Label = %q, want 6", got)
	}
}

func TestBuildNappyInsightsViewLimitsAxisGuides(t *testing.T) {
	insights := backendclient.NappyInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.NappyInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 13, WeeCount: 13},
		},
		Aggregate: backendclient.NappyInsightAggregate{HasAnyData: true, TotalCount: 13},
	}

	view := buildNappyInsightsView(insights, 7, "")

	if len(view.NappyChartAxisGuides) != 3 {
		t.Fatalf("NappyChartAxisGuides = %#v, want three 6-count marks up to the 18 ceiling", view.NappyChartAxisGuides)
	}
	if got := view.NappyChartAxisGuides[0].Label; got != "6" {
		t.Fatalf("NappyChartAxisGuides[0].Label = %q, want 6", got)
	}
	if got := view.NappyChartAxisGuides[2].Label; got != "18" {
		t.Fatalf("NappyChartAxisGuides[2].Label = %q, want 18", got)
	}
}

func TestBuildNappyInsightsViewSelectedDay(t *testing.T) {
	insights := backendclient.NappyInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.NappyInsightDay{
			{
				LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 3, WeeCount: 1, PooCount: 1, MixedCount: 1,
				Events: []backendclient.NappyInsightEvent{
					{Kind: "wee", HeavyWee: true, TimeLabel: "8:00 AM"},
					{Kind: "poo", Size: "blowout", TimeLabel: "11:00 AM"},
					{Kind: "mixed", Size: "small", TimeLabel: "2:00 PM"},
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
			BlowoutCount:       1,
			LargeCount:         0,
			HeavyWeeCount:      1,
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
	if !view.SelectedNappyDay.Events[1].HasSize || view.SelectedNappyDay.Events[1].SizeLabel != "Blowout" || view.SelectedNappyDay.Events[1].SizeClass != "blowout" {
		t.Fatalf("Events[1] size badge = %#v, want Blowout/blowout", view.SelectedNappyDay.Events[1])
	}
	if !view.SelectedNappyDay.Events[2].HasSize || view.SelectedNappyDay.Events[2].SizeLabel != "Small" || view.SelectedNappyDay.Events[2].SizeClass != "neutral" {
		t.Fatalf("Events[2] size badge = %#v, want Small/neutral", view.SelectedNappyDay.Events[2])
	}
	if view.SelectedNappyDay.Events[0].HasSize {
		t.Fatalf("Events[0] HasSize = true, want false for a wee-only event")
	}
	if view.SelectedNappyDay.Events[2].Tag != "Wee & Poo" || view.SelectedNappyDay.Events[2].TagClass != "mixed" {
		t.Fatalf("Events[2] = %#v, want Wee & Poo/mixed", view.SelectedNappyDay.Events[2])
	}
	if !view.ShowNappyBreakdown || view.NappyWeePercent != 33 || view.NappyPooPercent != 33 || view.NappyMixedPercent != 34 {
		t.Fatalf("breakdown = show=%v wee=%d poo=%d mixed=%d, want 33/33/34", view.ShowNappyBreakdown, view.NappyWeePercent, view.NappyPooPercent, view.NappyMixedPercent)
	}
	if view.NappyBlowoutCount != 1 || view.NappyLargeCount != 0 || view.NappyHeavyWeeCount != 1 {
		t.Fatalf("legend counts = blowout=%d large=%d heavy wee=%d, want 1/0/1", view.NappyBlowoutCount, view.NappyLargeCount, view.NappyHeavyWeeCount)
	}
	if view.NappyBlowoutLabel != "1 blowout" || view.NappyLargeLabel != "0 large poos" || view.NappyHeavyWeeLabel != "1 heavy wee" {
		t.Fatalf("legend labels = %q/%q/%q, want singular blowout, plural large poos, and singular heavy wee", view.NappyBlowoutLabel, view.NappyLargeLabel, view.NappyHeavyWeeLabel)
	}
	if view.NappyAverageGapLabel != "3h 0m" {
		t.Fatalf("NappyAverageGapLabel = %q, want 3h 0m", view.NappyAverageGapLabel)
	}
	if len(view.NappyChartDays) != 1 {
		t.Fatalf("len(NappyChartDays) = %d, want 1", len(view.NappyChartDays))
	}
	if len(view.NappyChartDays[0].Markers) != 2 {
		t.Fatalf("Markers = %#v, want markers for the heavy wee and blowout events", view.NappyChartDays[0].Markers)
	}
	if got := view.NappyChartDays[0].Markers[0]; got.MarkerClass != "heavy-wee" || got.BottomPercent != "16.7" {
		t.Fatalf("Markers[0] = %#v, want heavy wee at 16.7%%", got)
	}
	if got := view.NappyChartDays[0].Markers[1]; got.MarkerClass != "blowout" || got.BottomPercent != "50.0" {
		t.Fatalf("Markers[1] = %#v, want blowout at 50.0%%", got)
	}
}

func TestNappyMarkerCountLabel(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{count: 0, want: "0 blowouts"},
		{count: 1, want: "1 blowout"},
		{count: 2, want: "2 blowouts"},
	}
	for _, tt := range tests {
		if got := nappyMarkerCountLabel(tt.count, "blowout", "blowouts"); got != tt.want {
			t.Fatalf("nappyMarkerCountLabel(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestNappyDayMarkersOnlyFlagsLargeBlowoutAndHeavyWee(t *testing.T) {
	day := backendclient.NappyInsightDay{
		Events: []backendclient.NappyInsightEvent{
			{Kind: "wee", HeavyWee: true, TimeLabel: "8:00 AM"},
			{Kind: "poo", Size: "smear", TimeLabel: "10:00 AM"},
			{Kind: "poo", Size: "medium", TimeLabel: "12:00 PM"},
			{Kind: "poo", Size: "large", TimeLabel: "2:00 PM"},
			{Kind: "poo", Size: "blowout", TimeLabel: "4:00 PM"},
		},
	}

	markers := nappyDayMarkers(day)

	if len(markers) != 3 {
		t.Fatalf("len(markers) = %d, want 3 (heavy wee, large, and blowout)", len(markers))
	}
	if markers[0].MarkerClass != "heavy-wee" || markers[0].BottomPercent != "10.0" {
		t.Fatalf("markers[0] = %#v, want heavy wee at 10.0%%", markers[0])
	}
	if markers[1].MarkerClass != "large" || markers[1].BottomPercent != "70.0" {
		t.Fatalf("markers[1] = %#v, want large at 70.0%%", markers[1])
	}
	if markers[2].MarkerClass != "blowout" || markers[2].BottomPercent != "90.0" {
		t.Fatalf("markers[2] = %#v, want blowout at 90.0%%", markers[2])
	}
}

func TestNappyDayMarkersEmptyForNoEvents(t *testing.T) {
	if got := nappyDayMarkers(backendclient.NappyInsightDay{}); got != nil {
		t.Fatalf("nappyDayMarkers(empty day) = %#v, want nil", got)
	}
}

func TestNappySizeLabel(t *testing.T) {
	tests := []struct {
		size      string
		wantLabel string
		wantClass string
	}{
		{size: "smear", wantLabel: "Smear", wantClass: "neutral"},
		{size: "small", wantLabel: "Small", wantClass: "neutral"},
		{size: "medium", wantLabel: "Medium", wantClass: "neutral"},
		{size: "large", wantLabel: "Large", wantClass: "large"},
		{size: "blowout", wantLabel: "Blowout", wantClass: "blowout"},
		{size: "", wantLabel: "", wantClass: ""},
	}
	for _, tt := range tests {
		gotLabel, gotClass := nappySizeLabel(tt.size)
		if gotLabel != tt.wantLabel || gotClass != tt.wantClass {
			t.Fatalf("nappySizeLabel(%q) = %q/%q, want %q/%q", tt.size, gotLabel, gotClass, tt.wantLabel, tt.wantClass)
		}
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

func TestInsightsFeedMetricFromQuery(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "defaults to breast", url: "/insights?category=feeds", want: "breast"},
		{name: "accepts bottle", url: "/insights?category=feeds&metric=bottle", want: "bottle"},
		{name: "rejects unknown metric", url: "/insights?category=feeds&metric=formula", want: "breast"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			if got := insightsFeedMetricFromQuery(req); got != tt.want {
				t.Fatalf("insightsFeedMetricFromQuery(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestBuildFeedInsightsViewNoData(t *testing.T) {
	view := buildFeedInsightsView(backendclient.FeedInsights{
		RangeLabel: "Jun 28 – Jul 27",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-27"},
		},
		Aggregate: backendclient.FeedInsightAggregate{HasAnyData: false},
	}, 30, "breast", "")

	if view.HasFeedData {
		t.Fatalf("HasFeedData = true, want false")
	}
	if view.FeedHeroValue != "—" {
		t.Fatalf("FeedHeroValue = %q, want em dash with no data", view.FeedHeroValue)
	}
	if view.ShowFeedSupportingRow {
		t.Fatalf("ShowFeedSupportingRow = true, want false when there is no data")
	}
	if view.ShowFeedBreakdown {
		t.Fatalf("ShowFeedBreakdown = true, want false when there is no data")
	}
	if len(view.FeedChartDays) != 1 || view.FeedChartDays[0].HasData {
		t.Fatalf("FeedChartDays = %#v, want one day with HasData=false", view.FeedChartDays)
	}
}

func TestBuildFeedInsightsViewTrimsLeadingEmptyDays(t *testing.T) {
	insights := backendclient.FeedInsights{
		RangeLabel: "Jul 1 – Jul 6",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2"},
			{LocalDate: "2026-07-03", Label: "Jul 3", HasData: true, TotalCount: 3, BreastCount: 3, BreastMinutes: 45},
			{LocalDate: "2026-07-04", Label: "Jul 4"},
			{LocalDate: "2026-07-05", Label: "Jul 5", HasData: true, TotalCount: 2, BreastCount: 2, BreastMinutes: 30},
			{LocalDate: "2026-07-06", Label: "Jul 6"},
		},
		Aggregate: backendclient.FeedInsightAggregate{HasAnyData: true, TotalCount: 5},
	}

	view := buildFeedInsightsView(insights, 30, "breast", "")

	if len(view.FeedChartDays) != 4 {
		t.Fatalf("len(FeedChartDays) = %d, want four visible days", len(view.FeedChartDays))
	}
	if view.FeedChartDays[0].Key != "2026-07-03" || view.FeedChartDays[3].Key != "2026-07-06" {
		t.Fatalf("FeedChartDays = %#v, want range from first recorded day through the original final day", view.FeedChartDays)
	}
	if view.RecordsBeginLabel != "Records begin Jul 3" {
		t.Fatalf("RecordsBeginLabel = %q, want Records begin Jul 3", view.RecordsBeginLabel)
	}
}

func TestBuildFeedInsightsViewSelectedDay(t *testing.T) {
	insights := backendclient.FeedInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.FeedInsightDay{
			{
				LocalDate: "2026-07-20", Label: "Mon", HasData: true,
				TotalCount: 3, BreastCount: 1, FormulaCount: 1, ExpressedCount: 1,
				BreastMinutes: 15, FormulaMl: 120, ExpressedMl: 90, BottleMl: 210,
				Events: []backendclient.FeedInsightEvent{
					{Kind: "breast", TimeLabel: "8:00 AM", DetailLabel: "15 min"},
					{Kind: "formula", TimeLabel: "11:00 AM", DetailLabel: "120 ml"},
					{Kind: "expressed", TimeLabel: "2:00 PM", DetailLabel: "90 ml"},
				},
			},
		},
		Aggregate: backendclient.FeedInsightAggregate{
			HasAnyData: true, TotalCount: 3, BreastCount: 1, FormulaCount: 1, ExpressedCount: 1,
			AveragePerDayLabel:           "3.0",
			HasAverageGap:                true,
			AverageGapLabel:              "3h 0m",
			AverageGapCaption:            "Avg. time between feeds",
			BreastTotalMinutes:           15,
			BreastTotalLabel:             "15 min",
			BreastFeedsWithDurationCount: 1,
			BreastDurationBasisLabel:     "Duration recorded for 1 of 1 breast feed",
			BottleTotalMl:                210,
			BottleTotalLabel:             "210 ml",
			BreastPercent:                intPtrFor(33),
			FormulaPercent:               intPtrFor(33),
			ExpressedPercent:             intPtrFor(34),
		},
	}

	view := buildFeedInsightsView(insights, 7, "breast", "2026-07-20")

	if view.FeedHeroValue != "15 min" {
		t.Fatalf("FeedHeroValue = %q, want 15 min for the breast metric", view.FeedHeroValue)
	}
	if view.FeedHeroCaption != "Total recorded breast feeding time" {
		t.Fatalf("FeedHeroCaption = %q, want recorded-duration caption", view.FeedHeroCaption)
	}
	if view.FeedCountBasisLabel != "Duration recorded for 1 of 1 breast feed" {
		t.Fatalf("FeedCountBasisLabel = %q, want breast duration basis", view.FeedCountBasisLabel)
	}
	if view.SelectedFeedDay == nil {
		t.Fatalf("SelectedFeedDay = nil, want a selected day")
	}
	if view.SelectedFeedDay.TotalLabel != "3" || view.SelectedFeedDay.BreastLabel != "1" || view.SelectedFeedDay.FormulaLabel != "1" || view.SelectedFeedDay.ExpressedLabel != "1" {
		t.Fatalf("SelectedFeedDay counts = %#v, want 3/1/1/1", view.SelectedFeedDay)
	}
	if len(view.SelectedFeedDay.Events) != 3 {
		t.Fatalf("len(Events) = %d, want 3", len(view.SelectedFeedDay.Events))
	}
	if view.SelectedFeedDay.Events[1].Tag != "Formula" || view.SelectedFeedDay.Events[1].TagClass != "formula" {
		t.Fatalf("Events[1] = %#v, want Formula/formula", view.SelectedFeedDay.Events[1])
	}
	if view.SelectedFeedDay.Events[2].Tag != "Expressed" || view.SelectedFeedDay.Events[2].TagClass != "expressed" {
		t.Fatalf("Events[2] = %#v, want Expressed/expressed", view.SelectedFeedDay.Events[2])
	}
	if !view.ShowFeedBreakdown || view.FeedBreastPercent != 33 || view.FeedFormulaPercent != 33 || view.FeedExpressedPercent != 34 {
		t.Fatalf("breakdown = show=%v breast=%d formula=%d expressed=%d, want 33/33/34", view.ShowFeedBreakdown, view.FeedBreastPercent, view.FeedFormulaPercent, view.FeedExpressedPercent)
	}
	if view.FeedAverageGapLabel != "3h 0m" {
		t.Fatalf("FeedAverageGapLabel = %q, want 3h 0m", view.FeedAverageGapLabel)
	}
}

func TestBuildFeedInsightsViewDisclosesOngoingBreastDuration(t *testing.T) {
	view := buildFeedInsightsView(backendclient.FeedInsights{
		RangeLabel: "Jul 20",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 1, BreastCount: 1},
		},
		Aggregate: backendclient.FeedInsightAggregate{
			HasAnyData: true, TotalCount: 1, BreastCount: 1,
			BreastTotalLabel:         "Not yet available",
			BreastDurationBasisLabel: "Duration recorded for 0 of 1 breast feed",
			AveragePerDayLabel:       "1.0",
			AverageGapLabel:          "Not yet available",
			AverageGapCaption:        "Needs more recorded feeds",
		},
	}, 7, "breast", "")

	if !view.HasFeedData || view.FeedHeroValue != "Not yet available" {
		t.Fatalf("feed data/hero = %v/%q, want ongoing feed with unavailable duration", view.HasFeedData, view.FeedHeroValue)
	}
	if view.FeedCountBasisLabel != "Duration recorded for 0 of 1 breast feed" {
		t.Fatalf("FeedCountBasisLabel = %q, want ongoing duration disclosure", view.FeedCountBasisLabel)
	}
	if len(view.FeedChartDays) != 1 || view.FeedChartDays[0].HasData {
		t.Fatalf("FeedChartDays = %#v, want no duration bar for the ongoing feed", view.FeedChartDays)
	}
}

func TestBuildFeedInsightsViewBottleMetric(t *testing.T) {
	insights := backendclient.FeedInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 2, FormulaCount: 1, ExpressedCount: 1, FormulaMl: 120, ExpressedMl: 80, BottleMl: 200},
		},
		Aggregate: backendclient.FeedInsightAggregate{
			HasAnyData: true, TotalCount: 2, FormulaCount: 1, ExpressedCount: 1,
			BreastTotalLabel: "0 min",
			BottleTotalMl:    200,
			BottleTotalLabel: "200 ml",
		},
	}

	view := buildFeedInsightsView(insights, 7, "bottle", "")

	if view.IsBreastMetric {
		t.Fatalf("IsBreastMetric = true, want false for the bottle metric")
	}
	if view.FeedHeroValue != "200 ml" {
		t.Fatalf("FeedHeroValue = %q, want 200 ml for the bottle metric", view.FeedHeroValue)
	}
	if view.FeedHeroCaption != "Total formula & expressed volume" {
		t.Fatalf("FeedHeroCaption = %q, want the bottle caption", view.FeedHeroCaption)
	}
	if view.FeedCountBasisLabel != "2 out of 2 feeds recorded" {
		t.Fatalf("FeedCountBasisLabel = %q, want formula and expressed count out of all feeds", view.FeedCountBasisLabel)
	}
	if len(view.FeedChartDays) != 1 {
		t.Fatalf("len(FeedChartDays) = %d, want 1", len(view.FeedChartDays))
	}
	day := view.FeedChartDays[0]
	if !day.IsBottleMetric || day.IsBreastMetric {
		t.Fatalf("day metric flags = breast:%v bottle:%v, want bottle only", day.IsBreastMetric, day.IsBottleMetric)
	}
	if day.FormulaSharePercent != 60 || day.ExpressedSharePercent != 40 {
		t.Fatalf("day shares = formula:%d expressed:%d, want 60/40", day.FormulaSharePercent, day.ExpressedSharePercent)
	}
	if len(view.FeedChartAxisGuides) != 4 {
		t.Fatalf("FeedChartAxisGuides = %#v, want four 60ml marks up to the 240ml ceiling", view.FeedChartAxisGuides)
	}
	if got := view.FeedChartAxisGuides[0].Label; got != "60 ml" {
		t.Fatalf("FeedChartAxisGuides[0].Label = %q, want 60 ml", got)
	}
	if got := view.FeedChartAxisGuides[3].Label; got != "240 ml" {
		t.Fatalf("FeedChartAxisGuides[3].Label = %q, want 240 ml", got)
	}
}

func TestBuildFeedInsightsViewBreastMetricAxisUsesHourMarks(t *testing.T) {
	insights := backendclient.FeedInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 3, BreastCount: 3, BreastMinutes: 130},
		},
		Aggregate: backendclient.FeedInsightAggregate{HasAnyData: true, TotalCount: 3, BreastTotalLabel: "2h 10m"},
	}

	view := buildFeedInsightsView(insights, 7, "breast", "")

	if len(view.FeedChartAxisGuides) != 3 {
		t.Fatalf("FeedChartAxisGuides = %#v, want three 1h marks up to the 3h ceiling", view.FeedChartAxisGuides)
	}
	if got := view.FeedChartAxisGuides[0].Label; got != "1h" {
		t.Fatalf("FeedChartAxisGuides[0].Label = %q, want 1h", got)
	}
	if got := view.FeedChartAxisGuides[2].Label; got != "3h" {
		t.Fatalf("FeedChartAxisGuides[2].Label = %q, want 3h", got)
	}
}

func TestBuildFeedInsightsViewLimitsAxisGuides(t *testing.T) {
	tests := []struct {
		name       string
		metric     string
		day        backendclient.FeedInsightDay
		aggregate  backendclient.FeedInsightAggregate
		wantFirst  string
		wantLast   string
		wantGuides int
	}{
		{
			name:       "breast duration",
			metric:     "breast",
			day:        backendclient.FeedInsightDay{LocalDate: "2026-07-20", Label: "Mon", HasData: true, BreastMinutes: 360},
			aggregate:  backendclient.FeedInsightAggregate{HasAnyData: true, BreastTotalLabel: "6h"},
			wantFirst:  "2h",
			wantLast:   "6h",
			wantGuides: 3,
		},
		{
			name:       "bottle volume",
			metric:     "bottle",
			day:        backendclient.FeedInsightDay{LocalDate: "2026-07-20", Label: "Mon", HasData: true, FormulaMl: 780, BottleMl: 780},
			aggregate:  backendclient.FeedInsightAggregate{HasAnyData: true, BottleTotalLabel: "780 ml"},
			wantFirst:  "240 ml",
			wantLast:   "960 ml",
			wantGuides: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights := backendclient.FeedInsights{
				RangeLabel: "Jul 20 – Jul 26",
				Days:       []backendclient.FeedInsightDay{tt.day},
				Aggregate:  tt.aggregate,
			}

			view := buildFeedInsightsView(insights, 7, tt.metric, "")

			if len(view.FeedChartAxisGuides) != tt.wantGuides {
				t.Fatalf("FeedChartAxisGuides = %#v, want %d guides", view.FeedChartAxisGuides, tt.wantGuides)
			}
			if got := view.FeedChartAxisGuides[0].Label; got != tt.wantFirst {
				t.Fatalf("FeedChartAxisGuides[0].Label = %q, want %q", got, tt.wantFirst)
			}
			if got := view.FeedChartAxisGuides[len(view.FeedChartAxisGuides)-1].Label; got != tt.wantLast {
				t.Fatalf("last FeedChartAxisGuides label = %q, want %q", got, tt.wantLast)
			}
		})
	}
}

func TestBuildFeedInsightsViewDoesNotShowBottleOnlyDayAsBreastData(t *testing.T) {
	insights := backendclient.FeedInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.FeedInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", HasData: true, TotalCount: 1, FormulaCount: 1, FormulaMl: 120, BottleMl: 120},
		},
		Aggregate: backendclient.FeedInsightAggregate{
			HasAnyData:       true,
			TotalCount:       1,
			BreastTotalLabel: "0 min",
			BottleTotalLabel: "120 ml",
		},
	}

	view := buildFeedInsightsView(insights, 7, "breast", "")

	if len(view.FeedChartDays) != 1 {
		t.Fatalf("len(FeedChartDays) = %d, want 1", len(view.FeedChartDays))
	}
	if view.FeedChartDays[0].HasData || view.FeedChartDays[0].BarPercent != 0 {
		t.Fatalf("FeedChartDays[0] = %#v, want no breast-metric chart data", view.FeedChartDays[0])
	}
}

func TestInsightsPumpMetricFromQuery(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "", want: insightsPumpMetricVolume},
		{raw: "volume", want: insightsPumpMetricVolume},
		{raw: "duration", want: insightsPumpMetricDuration},
		{raw: "bogus", want: insightsPumpMetricVolume},
	}
	for _, tt := range tests {
		r := httptest.NewRequest("GET", "/insights?metric="+tt.raw, nil)
		if got := insightsPumpMetricFromQuery(r); got != tt.want {
			t.Fatalf("insightsPumpMetricFromQuery(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestBuildPumpInsightsViewNoData(t *testing.T) {
	view := buildPumpInsightsView(backendclient.PumpInsights{
		RangeLabel: "Jun 28 – Jul 27",
		Days: []backendclient.PumpInsightDay{
			{LocalDate: "2026-07-27"},
		},
		Aggregate: backendclient.PumpInsightAggregate{HasAnyData: false},
	}, 30, "volume", "")

	if view.HasPumpData {
		t.Fatalf("HasPumpData = true, want false")
	}
	if view.PumpHeroValue != "—" {
		t.Fatalf("PumpHeroValue = %q, want em dash with no data", view.PumpHeroValue)
	}
	if view.ShowPumpSupportingRow {
		t.Fatalf("ShowPumpSupportingRow = true, want false when there is no data")
	}
	if len(view.PumpChartDays) != 1 || view.PumpChartDays[0].HasData {
		t.Fatalf("PumpChartDays = %#v, want one day with HasData=false", view.PumpChartDays)
	}
}

func TestBuildPumpInsightsViewTrimsLeadingEmptyDays(t *testing.T) {
	insights := backendclient.PumpInsights{
		RangeLabel: "Jul 1 – Jul 6",
		Days: []backendclient.PumpInsightDay{
			{LocalDate: "2026-07-01", Label: "Jul 1"},
			{LocalDate: "2026-07-02", Label: "Jul 2"},
			{LocalDate: "2026-07-03", Label: "Jul 3", HasData: true, SessionCount: 2, TotalMl: 180},
			{LocalDate: "2026-07-04", Label: "Jul 4"},
			{LocalDate: "2026-07-05", Label: "Jul 5", HasData: true, SessionCount: 1, TotalMl: 90},
			{LocalDate: "2026-07-06", Label: "Jul 6"},
		},
		Aggregate: backendclient.PumpInsightAggregate{HasAnyData: true, SessionCount: 3},
	}

	view := buildPumpInsightsView(insights, 30, "volume", "")

	if len(view.PumpChartDays) != 4 {
		t.Fatalf("len(PumpChartDays) = %d, want four visible days", len(view.PumpChartDays))
	}
	if view.PumpChartDays[0].Key != "2026-07-03" || view.PumpChartDays[3].Key != "2026-07-06" {
		t.Fatalf("PumpChartDays = %#v, want range from first recorded day through the original final day", view.PumpChartDays)
	}
	if view.RecordsBeginLabel != "Records begin Jul 3" {
		t.Fatalf("RecordsBeginLabel = %q, want Records begin Jul 3", view.RecordsBeginLabel)
	}
}

func TestBuildPumpInsightsViewSelectedDay(t *testing.T) {
	insights := backendclient.PumpInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.PumpInsightDay{
			{
				LocalDate: "2026-07-20", Label: "Mon", FullLabel: "Monday, July 20", HasData: true,
				SessionCount: 2, TotalMl: 180, TotalMinutes: 40, DurationLabel: "40m",
				Events: []backendclient.PumpInsightEvent{
					{TimeLabel: "8:00 AM", VolumeLabel: "80 ml", DurationLabel: "18m"},
					{TimeLabel: "11:00 AM", VolumeLabel: "100 ml", DurationLabel: "22m"},
				},
			},
		},
		Aggregate: backendclient.PumpInsightAggregate{
			HasAnyData: true, SessionCount: 2,
			AveragePerDayLabel:            "2.0",
			HasAverageGap:                 true,
			AverageGapLabel:               "3h 0m",
			AverageGapCaption:             "Avg. time between sessions",
			TotalMlLabel:                  "180 ml",
			TotalDurationLabel:            "40m",
			AverageSessionMlLabel:         "90 ml",
			AverageSessionDurationLabel:   "20m",
			AverageSessionDurationCaption: "Avg. recorded session length",
			SessionsWithDurationCount:     2,
		},
	}

	view := buildPumpInsightsView(insights, 7, "volume", "2026-07-20")

	if view.PumpHeroValue != "180 ml" {
		t.Fatalf("PumpHeroValue = %q, want 180 ml for the volume metric", view.PumpHeroValue)
	}
	if view.PumpHeroCaption != "Total expressed volume" {
		t.Fatalf("PumpHeroCaption = %q, want volume caption", view.PumpHeroCaption)
	}
	if view.PumpCountBasisLabel != "2 sessions recorded" {
		t.Fatalf("PumpCountBasisLabel = %q, want 2 sessions recorded", view.PumpCountBasisLabel)
	}
	if view.SelectedPumpDay == nil {
		t.Fatalf("SelectedPumpDay = nil, want a selected day")
	}
	if view.SelectedPumpDay.SessionsLabel != "2" || view.SelectedPumpDay.DurationLabel != "40m" || view.SelectedPumpDay.VolumeLabel != "180 ml" {
		t.Fatalf("SelectedPumpDay = %#v, want 2/40m/180 ml", view.SelectedPumpDay)
	}
	if len(view.SelectedPumpDay.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(view.SelectedPumpDay.Events))
	}
	if view.SelectedPumpDay.Events[1].VolumeLabel != "100 ml" || view.SelectedPumpDay.Events[1].DurationLabel != "22m" {
		t.Fatalf("Events[1] = %#v, want 100 ml/22m", view.SelectedPumpDay.Events[1])
	}
	if view.PumpAverageGapLabel != "3h 0m" {
		t.Fatalf("PumpAverageGapLabel = %q, want 3h 0m", view.PumpAverageGapLabel)
	}
	if view.PumpAverageMlLabel != "90 ml" {
		t.Fatalf("PumpAverageMlLabel = %q, want 90 ml", view.PumpAverageMlLabel)
	}
	if view.PumpAverageSessionCaption != "Avg. recorded session length" {
		t.Fatalf("PumpAverageSessionCaption = %q, want recorded-duration caption", view.PumpAverageSessionCaption)
	}
	if view.PumpChartDays[0].AriaLabel != "Monday, July 20: 180 ml pumped" {
		t.Fatalf("PumpChartDays[0].AriaLabel = %q, want date and plotted volume", view.PumpChartDays[0].AriaLabel)
	}
}

func TestBuildPumpInsightsViewDurationMetric(t *testing.T) {
	insights := backendclient.PumpInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.PumpInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", FullLabel: "Monday, July 20", HasData: true, SessionCount: 1, TotalMl: 80, TotalMinutes: 18, DurationLabel: "18m"},
		},
		Aggregate: backendclient.PumpInsightAggregate{
			HasAnyData: true, SessionCount: 1, SessionsWithDurationCount: 1,
			TotalMlLabel:       "80 ml",
			TotalDurationLabel: "18m",
			DurationBasisLabel: "Duration recorded for 1 of 1 session",
		},
	}

	view := buildPumpInsightsView(insights, 7, "duration", "")

	if view.IsPumpVolumeMetric {
		t.Fatalf("IsPumpVolumeMetric = true, want false for the duration metric")
	}
	if view.PumpHeroValue != "18m" {
		t.Fatalf("PumpHeroValue = %q, want 18m for the duration metric", view.PumpHeroValue)
	}
	if view.PumpHeroCaption != "Total recorded pumping time" {
		t.Fatalf("PumpHeroCaption = %q, want duration caption", view.PumpHeroCaption)
	}
	if view.PumpCountBasisLabel != "Duration recorded for 1 of 1 session" {
		t.Fatalf("PumpCountBasisLabel = %q, want duration basis label", view.PumpCountBasisLabel)
	}
	if view.PumpChartDays[0].AriaLabel != "Monday, July 20: 18m total recorded pumping time" {
		t.Fatalf("PumpChartDays[0].AriaLabel = %q, want date and plotted duration", view.PumpChartDays[0].AriaLabel)
	}
}

func TestBuildPumpInsightsViewMissingDurationFallsBackToNotYetAvailable(t *testing.T) {
	insights := backendclient.PumpInsights{
		RangeLabel: "Jul 20 – Jul 26",
		Days: []backendclient.PumpInsightDay{
			{LocalDate: "2026-07-20", Label: "Mon", FullLabel: "Monday, July 20", HasData: true, SessionCount: 1, TotalMl: 80},
		},
		Aggregate: backendclient.PumpInsightAggregate{
			HasAnyData: true, SessionCount: 1, SessionsWithDurationCount: 0,
			TotalMlLabel:       "80 ml",
			TotalDurationLabel: "Not yet available",
			DurationBasisLabel: "Duration recorded for 0 of 1 session",
		},
	}

	view := buildPumpInsightsView(insights, 7, "duration", "")

	if view.PumpHeroValue != "Not yet available" {
		t.Fatalf("PumpHeroValue = %q, want the missing-duration fallback", view.PumpHeroValue)
	}
	if view.PumpCountBasisLabel != "Duration recorded for 0 of 1 session" {
		t.Fatalf("PumpCountBasisLabel = %q, want ongoing duration disclosure", view.PumpCountBasisLabel)
	}
	if len(view.PumpChartDays) != 1 || view.PumpChartDays[0].HasData {
		t.Fatalf("PumpChartDays = %#v, want no duration-metric chart data", view.PumpChartDays)
	}
	if view.PumpChartDays[0].AriaLabel != "Monday, July 20: pumping duration not recorded" {
		t.Fatalf("PumpChartDays[0].AriaLabel = %q, want missing-duration explanation", view.PumpChartDays[0].AriaLabel)
	}
}
