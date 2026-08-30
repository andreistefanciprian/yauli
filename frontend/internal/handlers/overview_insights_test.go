package handlers

import (
	"strings"
	"testing"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

func TestBuildOverviewStatsViewPopulated(t *testing.T) {
	nightPercent := 62
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		AgeLabel:       "6 weeks, 3 days old",
		BirthDateLabel: "12 June 2026",
		Sleep: backendclient.OverviewSleepStats{
			Available:              true,
			HasAnyData:             true,
			AverageTotalLabel:      "7h 45m",
			NightPercent:           &nightPercent,
			HasWakeWindow:          true,
			AverageWakeWindowLabel: "2h 10m",
		},
		Feed: backendclient.OverviewFeedStats{
			Available:           true,
			HasAnyData:          true,
			AveragePerDayLabel:  "6.2",
			BreastTotalLabel:    "36h 47m",
			FormulaTotalLabel:   "8.2 L",
			ExpressedTotalLabel: "4.8 L",
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
			Available:                   true,
			HasAnyData:                  true,
			LatestValueLabel:            "5.4 kg",
			HasBirthWeight:              true,
			ChangeSinceBirthLabel:       "+1.2 kg",
			HasLengthData:               true,
			LatestLengthLabel:           "58.3 cm",
			HasBirthLength:              true,
			LengthChangeSinceBirthLabel: "+7.8 cm",
		},
		Health: backendclient.OverviewHealthStats{
			Available:        true,
			VaccinationCount: 3,
			HasVaccinations:  true,
			RecentGroupLabel: "Vaccinations at 6 weeks",
			RecentDateLabel:  "Jul 24",
			RecentAgeLabel:   "at 6 weeks",
			VaccineHistory: []backendclient.OverviewHealthEvent{
				{NameLabel: "6-in-1 · Dose 1", DescriptionLabel: "Vaxelis", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
				{NameLabel: "Rotavirus · Dose 1", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
				{NameLabel: "Pneumococcal · Dose 1", DescriptionLabel: "Prevenar 13", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
			},
			MedicineCount:           2,
			HasMedicine:             true,
			RecentMedicineNameLabel: "Paracetamol · 1.5 ml",
			RecentMedicineDateLabel: "Jul 24",
			RecentMedicineAgeLabel:  "at 6 weeks",
			MedicineHistory: []backendclient.OverviewHealthEvent{
				{NameLabel: "Paracetamol · 1.5 ml", DescriptionLabel: "For fever", WhenLabel: "Jul 24, 2026 · 7:20 PM · at 6 weeks"},
				{NameLabel: "Vitamin D · 1 drop", WhenLabel: "Jul 26, 2026 · 8:00 AM · at 6 weeks"},
			},
			OtherCount:           1,
			HasOther:             true,
			RecentOtherNameLabel: "Sunscreen",
			RecentOtherDateLabel: "Aug 1",
			RecentOtherAgeLabel:  "at 7 weeks",
			OtherHistory: []backendclient.OverviewHealthEvent{
				{NameLabel: "Sunscreen", DescriptionLabel: "SPF 50", WhenLabel: "Aug 1, 2026 · 9:00 AM · at 7 weeks"},
			},
		},
	}, 30)

	if view.OverviewRangeContextLabel != "Recorded over the last 30 days" {
		t.Fatalf("OverviewRangeContextLabel = %q", view.OverviewRangeContextLabel)
	}
	if view.OverviewAgeLabel != "6 weeks, 3 days old" {
		t.Fatalf("OverviewAgeLabel = %q", view.OverviewAgeLabel)
	}
	if view.OverviewBirthDateLabel != "12 June 2026" {
		t.Fatalf("OverviewBirthDateLabel = %q", view.OverviewBirthDateLabel)
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
	if view.OverviewFeedFormulaLabel != "8.2 L formula" {
		t.Fatalf("OverviewFeedFormulaLabel = %q", view.OverviewFeedFormulaLabel)
	}
	if view.OverviewFeedExpressedLabel != "4.8 L expressed" {
		t.Fatalf("OverviewFeedExpressedLabel = %q", view.OverviewFeedExpressedLabel)
	}
	if view.OverviewNappyGapLabel != "1h 50m average spacing" {
		t.Fatalf("OverviewNappyGapLabel = %q", view.OverviewNappyGapLabel)
	}
	if view.OverviewGrowthChangeLabel != "+1.2 kg since birth" {
		t.Fatalf("OverviewGrowthChangeLabel = %q", view.OverviewGrowthChangeLabel)
	}
	if view.OverviewGrowthLengthLabel != "58.3 cm length (+7.8 cm since birth)" {
		t.Fatalf("OverviewGrowthLengthLabel = %q", view.OverviewGrowthLengthLabel)
	}
	if view.OverviewPumpSummaryLabel != "4 pumping sessions · 320 ml expressed" {
		t.Fatalf("OverviewPumpSummaryLabel = %q", view.OverviewPumpSummaryLabel)
	}

	if !view.OverviewHealthAvailable {
		t.Fatal("OverviewHealthAvailable = false, want true")
	}
	if view.OverviewHealthVaxCountLabel != "3 recorded" {
		t.Fatalf("OverviewHealthVaxCountLabel = %q", view.OverviewHealthVaxCountLabel)
	}
	if view.OverviewHealthVaxRecentLabel != "Most recent: Vaccinations at 6 weeks" {
		t.Fatalf("OverviewHealthVaxRecentLabel = %q", view.OverviewHealthVaxRecentLabel)
	}
	if view.OverviewHealthVaxMetaLabel != "Jul 24 · at 6 weeks" {
		t.Fatalf("OverviewHealthVaxMetaLabel = %q", view.OverviewHealthVaxMetaLabel)
	}
	if view.OverviewHealthVaxEmptyLabel != "" {
		t.Fatalf("OverviewHealthVaxEmptyLabel = %q, want empty when there is data", view.OverviewHealthVaxEmptyLabel)
	}
	if view.OverviewHealthMedCountLabel != "2 recorded" {
		t.Fatalf("OverviewHealthMedCountLabel = %q", view.OverviewHealthMedCountLabel)
	}
	if view.OverviewHealthMedRecentLabel != "Most recent: Paracetamol · 1.5 ml" {
		t.Fatalf("OverviewHealthMedRecentLabel = %q", view.OverviewHealthMedRecentLabel)
	}
	if view.OverviewHealthMedMetaLabel != "Jul 24 · at 6 weeks" {
		t.Fatalf("OverviewHealthMedMetaLabel = %q", view.OverviewHealthMedMetaLabel)
	}
	if view.OverviewHealthMedEmptyLabel != "" {
		t.Fatalf("OverviewHealthMedEmptyLabel = %q, want empty when there is data", view.OverviewHealthMedEmptyLabel)
	}
	if view.OverviewHealthOtherCountLabel != "1 recorded" {
		t.Fatalf("OverviewHealthOtherCountLabel = %q", view.OverviewHealthOtherCountLabel)
	}
	if view.OverviewHealthOtherRecentLabel != "Most recent: Sunscreen" {
		t.Fatalf("OverviewHealthOtherRecentLabel = %q", view.OverviewHealthOtherRecentLabel)
	}
	if view.OverviewHealthOtherMetaLabel != "Aug 1 · at 7 weeks" {
		t.Fatalf("OverviewHealthOtherMetaLabel = %q", view.OverviewHealthOtherMetaLabel)
	}
	if view.OverviewHealthOtherEmptyLabel != "" {
		t.Fatalf("OverviewHealthOtherEmptyLabel = %q, want empty when there is data", view.OverviewHealthOtherEmptyLabel)
	}
	// The history panel is a client-side disclosure now (see
	// insights-health-history.js) — the rows are always built into the view
	// regardless of any "open" state, since the response has no server-tracked
	// toggle state.
	if len(view.OverviewHealthVaxHistory) != 3 || view.OverviewHealthVaxHistory[0].NameLabel != "6-in-1 · Dose 1" {
		t.Fatalf("OverviewHealthVaxHistory = %#v, want 3 rows built unconditionally", view.OverviewHealthVaxHistory)
	}
	if len(view.OverviewHealthMedHistory) != 2 || view.OverviewHealthMedHistory[0].DescriptionLabel != "For fever" || !view.OverviewHealthMedHistory[0].HasDescription {
		t.Fatalf("OverviewHealthMedHistory = %#v, want 2 rows with the first description preserved", view.OverviewHealthMedHistory)
	}
	if len(view.OverviewHealthOtherHistory) != 1 || view.OverviewHealthOtherHistory[0].DescriptionLabel != "SPF 50" {
		t.Fatalf("OverviewHealthOtherHistory = %#v, want 1 row with description", view.OverviewHealthOtherHistory)
	}

	for _, want := range []string{
		"Here is a summary of my baby's recorded data from Yauli (30 days for feeds/sleep/nappies; growth and health cover the whole recorded history):",
		"- Age: 6 weeks, 3 days old (born 12 June 2026)",
		"- Sleep: 7h 45m per day, 62% recorded overnight, 2h 10m average awake window",
		"- Feeds: 6.2 per day (36h 47m breast, 8.2 L formula, 4.8 L expressed)",
		"- Nappies: 8.1 per day, 1h 50m average spacing",
		"- Growth: 5.4 kg (+1.2 kg since birth); 58.3 cm length (+7.8 cm since birth)",
		"- Feeding support: 4 pumping sessions · 320 ml expressed",
		"- Vaccinations: 3 recorded",
		"  · 6-in-1 · Dose 1 (Vaxelis) — Jul 24, 2026 · 10:35 AM · at 6 weeks",
		"  · Rotavirus · Dose 1 — Jul 24, 2026 · 10:35 AM · at 6 weeks",
		"  · Pneumococcal · Dose 1 (Prevenar 13) — Jul 24, 2026 · 10:35 AM · at 6 weeks",
		"- Medicine: 2 recorded",
		"  · Paracetamol · 1.5 ml (For fever) — Jul 24, 2026 · 7:20 PM · at 6 weeks",
		"  · Vitamin D · 1 drop — Jul 26, 2026 · 8:00 AM · at 6 weeks",
		"- Other: 1 recorded",
		"  · Sunscreen (SPF 50) — Aug 1, 2026 · 9:00 AM · at 7 weeks",
	} {
		if !strings.Contains(view.OverviewChatGptSummary, want) {
			t.Fatalf("OverviewChatGptSummary missing %q, got %q", want, view.OverviewChatGptSummary)
		}
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
		Health: backendclient.OverviewHealthStats{Available: true},
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
	if view.OverviewHealthVaxEmptyLabel != "None recorded" || !view.OverviewHealthVaxShowEmptyNote {
		t.Fatalf("vax empty state = %q / note:%v", view.OverviewHealthVaxEmptyLabel, view.OverviewHealthVaxShowEmptyNote)
	}
	if view.OverviewHealthMedEmptyLabel != "None recorded" {
		t.Fatalf("OverviewHealthMedEmptyLabel = %q", view.OverviewHealthMedEmptyLabel)
	}
	if view.OverviewHealthOtherEmptyLabel != "None recorded" {
		t.Fatalf("OverviewHealthOtherEmptyLabel = %q", view.OverviewHealthOtherEmptyLabel)
	}

	for _, want := range []string{
		"- Sleep: Not enough recorded sleep yet",
		"- Feeds: Not enough recorded feeds yet",
		"- Nappies: Not enough recorded changes yet",
		"- Vaccinations: None recorded",
		"- Medicine: None recorded",
		"- Other: None recorded",
	} {
		if !strings.Contains(view.OverviewChatGptSummary, want) {
			t.Fatalf("OverviewChatGptSummary missing %q, got %q", want, view.OverviewChatGptSummary)
		}
	}
}

// TestBuildOverviewChatGptSummaryIncludesLengthOnlyGrowth is a regression
// test: weight and length are recorded independently (HasAnyData reflects
// weight only, HasLengthData is separate), so a baby with a length
// measurement but no weight measurement must still get its length reported
// in the prompt rather than falling through to "No recorded weight yet" and
// losing the length data entirely.
func TestBuildOverviewChatGptSummaryIncludesLengthOnlyGrowth(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{
		Growth: backendclient.OverviewGrowthStats{
			Available:                   true,
			HasAnyData:                  false,
			HasLengthData:               true,
			LatestLengthLabel:           "58.3 cm",
			HasBirthLength:              true,
			LengthChangeSinceBirthLabel: "+7.8 cm",
		},
	}, 30)

	if !strings.Contains(view.OverviewChatGptSummary, "- Growth: 58.3 cm length (+7.8 cm since birth)") {
		t.Fatalf("OverviewChatGptSummary missing length-only growth line, got %q", view.OverviewChatGptSummary)
	}
	if strings.Contains(view.OverviewChatGptSummary, "No recorded weight yet") {
		t.Fatalf("length-only growth should not fall through to the no-data copy, got %q", view.OverviewChatGptSummary)
	}
}

func TestBuildOverviewChatGptSummaryLabelsFutureBirthDateAsExpected(t *testing.T) {
	summary := buildOverviewChatGptSummary(backendclient.OverviewInsights{
		BirthDateLabel: "12 September 2026",
	}, "30 days")

	if !strings.Contains(summary, "- Expected: 12 September 2026") {
		t.Fatalf("summary missing expected birth date, got %q", summary)
	}
	if strings.Contains(summary, "Born:") || strings.Contains(summary, "0 days old") {
		t.Fatalf("summary describes a future birth date as already born: %q", summary)
	}
}

func TestBuildOverviewChatGptSummaryLimitsHealthHistory(t *testing.T) {
	history := []backendclient.OverviewHealthEvent{
		{NameLabel: "Newest", WhenLabel: "Aug 7"},
		{NameLabel: "Second", WhenLabel: "Aug 6"},
		{NameLabel: "Third", WhenLabel: "Aug 5"},
		{NameLabel: "Fourth", WhenLabel: "Aug 4"},
		{NameLabel: "Fifth", WhenLabel: "Aug 3"},
		{NameLabel: "Sixth", WhenLabel: "Aug 2"},
		{NameLabel: "Oldest", WhenLabel: "Aug 1"},
	}

	summary := buildOverviewChatGptSummary(backendclient.OverviewInsights{
		Health: backendclient.OverviewHealthStats{
			Available:        true,
			HasVaccinations:  true,
			VaccinationCount: len(history),
			VaccineHistory:   history,
			HasMedicine:      true,
			MedicineCount:    len(history),
			MedicineHistory:  history,
			HasOther:         true,
			OtherCount:       len(history),
			OtherHistory:     history,
		},
	}, "30 days")

	for _, want := range []string{
		"  · Newest — Aug 7",
		"  · Fifth — Aug 3",
		"  · 2 older vaccination entries omitted",
		"  · 2 older medicine entries omitted",
		"  · 2 older other entries omitted",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q, got %q", want, summary)
		}
	}
	for _, omitted := range []string{"Sixth", "Oldest"} {
		if strings.Contains(summary, omitted) {
			t.Fatalf("summary contains older entry %q beyond the limit: %q", omitted, summary)
		}
	}
}

func TestAppendOverviewChatGptHealthHistoryUsesSingularOmissionLabel(t *testing.T) {
	history := make([]backendclient.OverviewHealthEvent, overviewChatGptHealthHistoryLimit+1)
	for i := range history {
		history[i] = backendclient.OverviewHealthEvent{NameLabel: "Entry", WhenLabel: "Aug 1"}
	}

	lines := appendOverviewChatGptHealthHistory(nil, history, "medicine")
	if got := lines[len(lines)-1]; got != "  · 1 older medicine entry omitted" {
		t.Fatalf("omission label = %q, want singular entry", got)
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
	if view.OverviewHealthVaxEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewHealthVaxEmptyLabel = %q", view.OverviewHealthVaxEmptyLabel)
	}
	if view.OverviewHealthVaxShowEmptyNote {
		t.Fatalf("OverviewHealthVaxShowEmptyNote = true, want false when the source failed rather than legitimately having nothing")
	}
	if view.OverviewHealthMedEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewHealthMedEmptyLabel = %q", view.OverviewHealthMedEmptyLabel)
	}
	if view.OverviewHealthOtherEmptyLabel != "Temporarily unavailable" {
		t.Fatalf("OverviewHealthOtherEmptyLabel = %q", view.OverviewHealthOtherEmptyLabel)
	}
	if view.OverviewHealthAvailable {
		t.Fatal("OverviewHealthAvailable = true, want false when the source failed")
	}
}

func TestBuildOverviewStatsViewHealthWithoutBirthDate(t *testing.T) {
	view := buildOverviewStatsView(backendclient.OverviewInsights{Health: backendclient.OverviewHealthStats{
		Available:        true,
		HasVaccinations:  true,
		VaccinationCount: 1,
		RecentGroupLabel: "Vaccinations",
		RecentDateLabel:  "Jul 24",
	}}, 30)

	if view.OverviewHealthVaxRecentLabel != "Most recent: Vaccinations" {
		t.Fatalf("OverviewHealthVaxRecentLabel = %q", view.OverviewHealthVaxRecentLabel)
	}
	if view.OverviewHealthVaxMetaLabel != "Jul 24" {
		t.Fatalf("OverviewHealthVaxMetaLabel = %q, want date without trailing separator", view.OverviewHealthVaxMetaLabel)
	}
}

func TestBuildOverviewStatsViewAlwaysBuildsHealthHistory(t *testing.T) {
	health := backendclient.OverviewHealthStats{
		Available:        true,
		HasVaccinations:  true,
		VaccinationCount: 2,
		VaccineHistory: []backendclient.OverviewHealthEvent{
			{NameLabel: "6-in-1 · Dose 1", DescriptionLabel: "Vaxelis", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
			{NameLabel: "Rotavirus · Dose 1", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
		},
		HasMedicine: true,
		MedicineHistory: []backendclient.OverviewHealthEvent{
			{NameLabel: "Paracetamol · 1.5 ml", WhenLabel: "Jul 24, 2026 · 7:20 PM · at 6 weeks"},
		},
		HasOther: true,
		OtherHistory: []backendclient.OverviewHealthEvent{
			{NameLabel: "Sunscreen", DescriptionLabel: "SPF 50", WhenLabel: "Aug 1, 2026 · 9:00 AM · at 7 weeks"},
		},
	}

	// No "history open" flag exists anymore — buildOverviewStatsView only
	// takes insights and rangeDays. Expanding/collapsing the panel happens
	// entirely client-side (insights-health-history.js) against these rows,
	// which are always present in the response.
	view := buildOverviewStatsView(backendclient.OverviewInsights{Health: health}, 30)

	if len(view.OverviewHealthVaxHistory) != 2 {
		t.Fatalf("OverviewHealthVaxHistory = %#v, want 2 rows", view.OverviewHealthVaxHistory)
	}
	if !view.OverviewHealthVaxHistory[0].HasDescription || view.OverviewHealthVaxHistory[0].DescriptionLabel != "Vaxelis" {
		t.Fatalf("OverviewHealthVaxHistory[0] description = %#v, want Vaxelis", view.OverviewHealthVaxHistory[0])
	}
	if view.OverviewHealthVaxHistory[1].HasDescription {
		t.Fatalf("OverviewHealthVaxHistory[1] = %#v, want no description recorded", view.OverviewHealthVaxHistory[1])
	}
	if len(view.OverviewHealthMedHistory) != 1 {
		t.Fatalf("OverviewHealthMedHistory = %#v", view.OverviewHealthMedHistory)
	}
	if len(view.OverviewHealthOtherHistory) != 1 || view.OverviewHealthOtherHistory[0].DescriptionLabel != "SPF 50" {
		t.Fatalf("OverviewHealthOtherHistory = %#v", view.OverviewHealthOtherHistory)
	}
}
