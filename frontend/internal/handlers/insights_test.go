package handlers

import (
	"net/http/httptest"
	"testing"

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
			{LocalDate: "2026-07-27", IsToday: true},
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
			{LocalDate: "2026-07-27", HasData: false, IsToday: true},
		},
		Aggregate: backendclient.SleepInsightAggregate{HasAnyData: true, AverageTotalLabel: "8 hr"},
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

func TestSplitPercentFallsBackTo50WhenTotalIsZero(t *testing.T) {
	if got := splitPercent(0, 0); got != 50 {
		t.Fatalf("splitPercent(0, 0) = %d, want 50", got)
	}
}
