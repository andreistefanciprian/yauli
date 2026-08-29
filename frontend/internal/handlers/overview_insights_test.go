package handlers

import (
	"testing"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

func TestBuildOverviewStatsViewPopulated(t *testing.T) {
	nightPercent := 62
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Sleep: backendclient.OverviewSleepStats{
			Available:              true,
			HasAnyData:             true,
			AverageTotalLabel:      "7h 45m",
			NightPercent:           &nightPercent,
			HasWakeWindow:          true,
			AverageWakeWindowLabel: "2h 10m",
		},
		Feed: backendclient.OverviewFeedStats{
			Available:          true,
			HasAnyData:         true,
			AveragePerDayLabel: "6.2",
			BreastTotalLabel:   "36h 47m",
			BottleTotalLabel:   "13010 ml",
		},
		Nappy: backendclient.OverviewNappyStats{
			Available:          true,
			HasAnyData:         true,
			AveragePerDayLabel: "8.1",
			HasAverageGap:      true,
			AverageGapLabel:    "1h 50m",
		},
		Pump: backendclient.OverviewPumpStats{
			Available:    true,
			HasAnyData:   true,
			SessionCount: 4,
			TotalMlLabel: "320 ml",
		},
		Growth: backendclient.OverviewGrowthStats{
			Available:             true,
			HasAnyData:            true,
			LatestValueLabel:      "5.4 kg",
			HasBirthWeight:        true,
			ChangeSinceBirthLabel: "+1.2 kg",
		},
	}, 30)

	if view.OverviewRangeContextLabel != "Recorded over the last 30 days" {
		t.Fatalf("OverviewRangeContextLabel = %q", view.OverviewRangeContextLabel)
	}
	if view.OverviewSleepValueLabel != "7h 45m" {
		t.Fatalf("OverviewSleepValueLabel = %q", view.OverviewSleepValueLabel)
	}
	if view.OverviewSleepNightLabel != "62% recorded overnight" {
		t.Fatalf("OverviewSleepNightLabel = %q", view.OverviewSleepNightLabel)
	}
	if view.OverviewSleepWakeLabel != "2h 10m average awake window" {
		t.Fatalf("OverviewSleepWakeLabel = %q", view.OverviewSleepWakeLabel)
	}
	if view.OverviewSleepEmptyLabel != "" {
		t.Fatalf("OverviewSleepEmptyLabel = %q, want empty when there is data", view.OverviewSleepEmptyLabel)
	}
	if view.OverviewFeedBreastLabel != "36h 47m breast" {
		t.Fatalf("OverviewFeedBreastLabel = %q", view.OverviewFeedBreastLabel)
	}
	if view.OverviewFeedBottleLabel != "13010 ml bottle" {
		t.Fatalf("OverviewFeedBottleLabel = %q", view.OverviewFeedBottleLabel)
	}
	if view.OverviewNappyGapLabel != "1h 50m average spacing" {
		t.Fatalf("OverviewNappyGapLabel = %q", view.OverviewNappyGapLabel)
	}
	if view.OverviewGrowthChangeLabel != "+1.2 kg since birth" {
		t.Fatalf("OverviewGrowthChangeLabel = %q", view.OverviewGrowthChangeLabel)
	}
	if view.OverviewPumpSummaryLabel != "4 pumping sessions · 320 ml expressed" {
		t.Fatalf("OverviewPumpSummaryLabel = %q", view.OverviewPumpSummaryLabel)
	}

	for _, r := range view.Ranges {
		if r.Label == "30 days" && !r.Active {
			t.Fatalf("30 days pill should be active for rangeDays=30")
		}
	}
}

func TestBuildOverviewStatsViewEmpty(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Sleep:  backendclient.OverviewSleepStats{Available: true},
		Feed:   backendclient.OverviewFeedStats{Available: true},
		Nappy:  backendclient.OverviewNappyStats{Available: true},
		Pump:   backendclient.OverviewPumpStats{Available: true},
		Growth: backendclient.OverviewGrowthStats{Available: true},
	}, 7)

	if view.OverviewRangeContextLabel != "Recorded over the last 7 days" {
		t.Fatalf("OverviewRangeContextLabel = %q", view.OverviewRangeContextLabel)
	}
	if view.OverviewSleepValueLabel != "—" || view.OverviewSleepEmptyLabel != "Not enough recorded sleep yet" {
		t.Fatalf("sleep empty state = %q / %q", view.OverviewSleepValueLabel, view.OverviewSleepEmptyLabel)
	}
	if view.OverviewSleepNightLabel != "" || view.OverviewSleepWakeLabel != "" {
		t.Fatalf("sleep support lines should be blank when there is no data, got %q / %q", view.OverviewSleepNightLabel, view.OverviewSleepWakeLabel)
	}
	if view.OverviewFeedEmptyLabel != "Not enough recorded feeds yet" {
		t.Fatalf("OverviewFeedEmptyLabel = %q", view.OverviewFeedEmptyLabel)
	}
	if view.OverviewNappyEmptyLabel != "Not enough recorded changes yet" {
		t.Fatalf("OverviewNappyEmptyLabel = %q", view.OverviewNappyEmptyLabel)
	}
	if view.OverviewGrowthChangeLabel != "No recorded weight yet" {
		t.Fatalf("OverviewGrowthChangeLabel = %q", view.OverviewGrowthChangeLabel)
	}
	if view.OverviewPumpSummaryLabel != "No pumping sessions recorded" {
		t.Fatalf("OverviewPumpSummaryLabel = %q", view.OverviewPumpSummaryLabel)
	}
}

func TestBuildOverviewStatsViewFallsBackToLegacyFeedGap(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Feed: backendclient.OverviewFeedStats{
			Available:          true,
			HasAnyData:         true,
			AveragePerDayLabel: "6.2",
			HasAverageGap:      true,
			AverageGapLabel:    "2h 30m",
		},
	}, 30)

	if view.OverviewFeedGapLabel != "2h 30m average spacing" {
		t.Fatalf("OverviewFeedGapLabel = %q", view.OverviewFeedGapLabel)
	}
	if view.OverviewFeedBreastLabel != "" || view.OverviewFeedBottleLabel != "" || view.OverviewFeedEmptyLabel != "" {
		t.Fatalf("feed fallback state = %q / %q / %q, want only legacy gap", view.OverviewFeedBreastLabel, view.OverviewFeedBottleLabel, view.OverviewFeedEmptyLabel)
	}
}

func TestBuildOverviewStatsViewGrowthWithoutBirthWeight(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Growth: backendclient.OverviewGrowthStats{
			Available:        true,
			HasAnyData:       true,
			LatestValueLabel: "5.4 kg",
			HasBirthWeight:   false,
		},
	}, 30)

	if view.OverviewGrowthValueLabel != "5.4 kg" {
		t.Fatalf("OverviewGrowthValueLabel = %q", view.OverviewGrowthValueLabel)
	}
	if view.OverviewGrowthChangeLabel != "Birth weight not recorded" {
		t.Fatalf("OverviewGrowthChangeLabel = %q, want the missing-birth-weight copy, not the no-data copy", view.OverviewGrowthChangeLabel)
	}
}

func TestBuildOverviewStatsViewSleepWithoutWakeWindow(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Sleep: backendclient.OverviewSleepStats{
			Available:         true,
			HasAnyData:        true,
			AverageTotalLabel: "3h 10m",
			HasWakeWindow:     false,
		},
	}, 30)

	if view.OverviewSleepValueLabel != "3h 10m" {
		t.Fatalf("OverviewSleepValueLabel = %q", view.OverviewSleepValueLabel)
	}
	if view.OverviewSleepWakeLabel != "Not enough recorded sleep yet for an average awake window" {
		t.Fatalf("OverviewSleepWakeLabel = %q", view.OverviewSleepWakeLabel)
	}
	if view.OverviewSleepEmptyLabel != "" {
		t.Fatalf("OverviewSleepEmptyLabel = %q, want empty — there is sleep data, just not enough for a wake window", view.OverviewSleepEmptyLabel)
	}
}

func TestBuildOverviewStatsViewUnavailableSources(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{}, 30)

	if view.OverviewSleepEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewSleepEmptyLabel = %q", view.OverviewSleepEmptyLabel)
	}
	if view.OverviewFeedEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewFeedEmptyLabel = %q", view.OverviewFeedEmptyLabel)
	}
	if view.OverviewNappyEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewNappyEmptyLabel = %q", view.OverviewNappyEmptyLabel)
	}
	if view.OverviewGrowthChangeLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewGrowthChangeLabel = %q", view.OverviewGrowthChangeLabel)
	}
	if view.OverviewPumpSummaryLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewPumpSummaryLabel = %q", view.OverviewPumpSummaryLabel)
	}
}
