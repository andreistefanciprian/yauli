package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func medicationEvent(babyID uuid.UUID, occurredAt time.Time, items ...map[string]any) store.Event {
	return store.Event{
		ID:         uuid.New(),
		BabyID:     babyID,
		EventType:  eventTypeMedication,
		OccurredAt: occurredAt,
		Attributes: map[string]any{"items": items},
	}
}

func vaccineItem(name, seriesDose, description string) map[string]any {
	item := map[string]any{"kind": "vaccine", "name": name}
	if seriesDose != "" {
		item["series_dose"] = seriesDose
	}
	if description != "" {
		item["description"] = description
	}
	return item
}

func medicineItem(name string, doseValue float64, doseUnit string) map[string]any {
	return map[string]any{"kind": "medicine", "name": name, "dose_value": doseValue, "dose_unit": doseUnit}
}

func TestOverviewHealthStatsBuildsHistoriesNewestFirst(t *testing.T) {
	babyID := uuid.New()
	loc := time.UTC

	// ListEventsByType returns newest-first.
	events := []store.Event{
		medicationEvent(babyID, time.Date(2026, 8, 1, 9, 0, 0, 0, loc), medicineItem("Calpol", 2, "ml")),
		medicationEvent(babyID, time.Date(2026, 7, 1, 10, 0, 0, 0, loc),
			vaccineItem("6-in-1", "second", "Vaxelis"),
			medicineItem("Ibuprofen", 5, "mg"),
		),
		medicationEvent(babyID, time.Date(2026, 2, 12, 10, 35, 0, 0, loc),
			vaccineItem("Rotavirus", "first", ""),
			vaccineItem("Pneumococcal", "first", ""),
			medicineItem("Paracetamol", 1.5, "ml"),
		),
		medicationEvent(babyID, time.Date(2026, 1, 15, 8, 0, 0, 0, loc), medicineItem("Vitamin D", 1, "drops")),
	}

	fake := &overviewFakeStore{aiReportFakeStore: &aiReportFakeStore{
		baby: store.Baby{
			ID: babyID, Timezone: "UTC", BirthDate: "2026-01-01",
		},
		events: events,
	}}
	h := &Handlers{Store: fake}
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, loc)

	stats := h.overviewHealthStats(context.Background(), fake.baby, loc, now)

	if !stats.Available {
		t.Fatalf("Available = false, want true")
	}

	if stats.VaccinationCount != 3 {
		t.Fatalf("VaccinationCount = %d, want 3", stats.VaccinationCount)
	}
	if !stats.HasVaccinations {
		t.Fatalf("HasVaccinations = false, want true")
	}
	if stats.RecentGroupLabel != "Vaccinations at 6 months" {
		t.Fatalf("RecentGroupLabel = %q, want %q", stats.RecentGroupLabel, "Vaccinations at 6 months")
	}
	if stats.RecentDateLabel != "Jul 1" {
		t.Fatalf("RecentDateLabel = %q, want %q", stats.RecentDateLabel, "Jul 1")
	}
	if stats.RecentAgeLabel != "at 6 months" {
		t.Fatalf("RecentAgeLabel = %q, want %q", stats.RecentAgeLabel, "at 6 months")
	}

	wantVaccineNames := []string{"6-in-1 · Dose 2", "Rotavirus · Dose 1", "Pneumococcal · Dose 1"}
	if len(stats.VaccineHistory) != len(wantVaccineNames) {
		t.Fatalf("VaccineHistory = %#v, want %d rows", stats.VaccineHistory, len(wantVaccineNames))
	}
	for i, want := range wantVaccineNames {
		if stats.VaccineHistory[i].NameLabel != want {
			t.Errorf("VaccineHistory[%d].NameLabel = %q, want %q", i, stats.VaccineHistory[i].NameLabel, want)
		}
	}
	if stats.VaccineHistory[0].DescriptionLabel != "Vaxelis" {
		t.Errorf("VaccineHistory[0].DescriptionLabel = %q, want %q", stats.VaccineHistory[0].DescriptionLabel, "Vaxelis")
	}
	if stats.VaccineHistory[1].DescriptionLabel != "" {
		t.Errorf("VaccineHistory[1].DescriptionLabel = %q, want empty", stats.VaccineHistory[1].DescriptionLabel)
	}
	if want := "Jul 1, 2026 · 10:00 AM · at 6 months"; stats.VaccineHistory[0].WhenLabel != want {
		t.Errorf("VaccineHistory[0].WhenLabel = %q, want %q", stats.VaccineHistory[0].WhenLabel, want)
	}

	if !stats.HasMedicine {
		t.Fatalf("HasMedicine = false, want true")
	}
	wantMedicineNames := []string{"Calpol · 2 ml", "Ibuprofen · 5 mg", "Paracetamol · 1.5 ml", "Vitamin D · 1 drop"}
	if len(stats.MedicineHistory) != len(wantMedicineNames) {
		t.Fatalf("MedicineHistory = %#v, want %d rows", stats.MedicineHistory, len(wantMedicineNames))
	}
	for i, want := range wantMedicineNames {
		if stats.MedicineHistory[i].NameLabel != want {
			t.Errorf("MedicineHistory[%d].NameLabel = %q, want %q", i, stats.MedicineHistory[i].NameLabel, want)
		}
	}
	if len(stats.MedicineRecent) != overviewHealthMedicineRecentLimit {
		t.Fatalf("MedicineRecent = %#v, want %d rows (capped)", stats.MedicineRecent, overviewHealthMedicineRecentLimit)
	}
	for i, want := range wantMedicineNames[:overviewHealthMedicineRecentLimit] {
		if stats.MedicineRecent[i].NameLabel != want {
			t.Errorf("MedicineRecent[%d].NameLabel = %q, want %q", i, stats.MedicineRecent[i].NameLabel, want)
		}
	}
	if want := "Aug 1, 9:00 AM"; stats.MedicineRecent[0].ShortWhenLabel != want {
		t.Errorf("MedicineRecent[0].ShortWhenLabel = %q, want %q", stats.MedicineRecent[0].ShortWhenLabel, want)
	}
}

func TestOverviewHealthStatsEmptyWhenNothingRecorded(t *testing.T) {
	babyID := uuid.New()
	fake := &overviewFakeStore{aiReportFakeStore: &aiReportFakeStore{
		baby: store.Baby{ID: babyID, Timezone: "UTC", BirthDate: "2026-01-01"},
	}}
	h := &Handlers{Store: fake}

	stats := h.overviewHealthStats(context.Background(), fake.baby, time.UTC, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC))

	if !stats.Available {
		t.Fatalf("Available = false, want true")
	}
	if stats.HasVaccinations || stats.VaccinationCount != 0 {
		t.Fatalf("vaccinations = %#v, want none", stats)
	}
	if stats.HasMedicine || len(stats.MedicineHistory) != 0 {
		t.Fatalf("medicine = %#v, want none", stats)
	}
}

func TestOverviewHealthStatsOmitsAgeWhenBirthDateIsMissing(t *testing.T) {
	babyID := uuid.New()
	occurredAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	fake := &overviewFakeStore{aiReportFakeStore: &aiReportFakeStore{
		baby: store.Baby{ID: babyID, Timezone: "UTC"},
		events: []store.Event{
			medicationEvent(babyID, occurredAt, vaccineItem("Rotavirus", "first", "")),
		},
	}}
	h := &Handlers{Store: fake}

	stats := h.overviewHealthStats(context.Background(), fake.baby, time.UTC, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC))

	if stats.RecentGroupLabel != "Vaccinations" {
		t.Fatalf("RecentGroupLabel = %q, want %q", stats.RecentGroupLabel, "Vaccinations")
	}
	if stats.RecentAgeLabel != "" {
		t.Fatalf("RecentAgeLabel = %q, want empty", stats.RecentAgeLabel)
	}
	if stats.VaccineHistory[0].WhenLabel != "Jul 1, 2026 · 10:00 AM" {
		t.Fatalf("WhenLabel = %q, want date and time without age", stats.VaccineHistory[0].WhenLabel)
	}
}

func TestOverviewHealthStatsUnavailableOnStoreError(t *testing.T) {
	babyID := uuid.New()
	fake := &overviewFakeStore{
		aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{ID: babyID, Timezone: "UTC", BirthDate: "2026-01-01"}},
		failedEventType:   eventTypeMedication,
	}
	h := &Handlers{Store: fake}

	stats := h.overviewHealthStats(context.Background(), fake.baby, time.UTC, time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC))

	if stats.Available {
		t.Fatalf("Available = true, want false after the store query failed")
	}
}

func TestHealthInsightsAgeLabel(t *testing.T) {
	birth := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		occurredAt time.Time
		want       string
	}{
		{name: "day of birth", occurredAt: birth, want: "at 0 weeks"},
		{name: "exactly one week", occurredAt: birth.AddDate(0, 0, 7), want: "at 1 week"},
		{name: "under 13 weeks stays in weeks", occurredAt: birth.AddDate(0, 0, 84), want: "at 12 weeks"},
		{name: "13 weeks crosses into months", occurredAt: birth.AddDate(0, 0, 91), want: "at 3 months"},
		{name: "six months", occurredAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), want: "at 6 months"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthInsightsAgeLabel(birth, tt.occurredAt); got != tt.want {
				t.Errorf("healthInsightsAgeLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHealthInsightsAgeLabelUsesLocalCalendarDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	birth := time.Date(2026, 9, 28, 0, 0, 0, 0, loc)
	occurredAt := time.Date(2026, 10, 5, 0, 0, 0, 0, loc)

	if got := healthInsightsAgeLabel(birth, occurredAt); got != "at 1 week" {
		t.Fatalf("healthInsightsAgeLabel() = %q, want %q", got, "at 1 week")
	}
}

func TestDoseAmountLabel(t *testing.T) {
	tests := []struct {
		value float64
		unit  MedicationDoseUnit
		want  string
	}{
		{value: 1.5, unit: MedicationDoseUnitML, want: "1.5 ml"},
		{value: 2.5, unit: MedicationDoseUnitMG, want: "2.5 mg"},
		{value: 1, unit: MedicationDoseUnitDrops, want: "1 drop"},
		{value: 2, unit: MedicationDoseUnitDrops, want: "2 drops"},
		{value: 1, unit: MedicationDoseUnitDose, want: "1 dose"},
		{value: 2, unit: MedicationDoseUnitDose, want: "2 doses"},
		{value: 1, unit: MedicationDoseUnitOther, want: "1 unit"},
		{value: 3, unit: MedicationDoseUnitOther, want: "3 units"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := doseAmountLabel(tt.value, tt.unit); got != tt.want {
				t.Errorf("doseAmountLabel(%v, %q) = %q, want %q", tt.value, tt.unit, got, tt.want)
			}
		})
	}
}
