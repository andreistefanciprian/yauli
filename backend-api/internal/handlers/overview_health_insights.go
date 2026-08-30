package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

// overviewHealthEventsLimit is generous on purpose, same reasoning as
// growthInsightsEventsLimit: medication events (vaccines + medicine) are
// logged far less often than sleeps or feeds, so this cap should retain years
// of recent history without an unbounded query.
const overviewHealthEventsLimit = 5000

// overviewHealthStats is the Insights Overview tab's "Health & medicine"
// card payload. Independent of the range pill — like Growth, vaccination and
// medicine history is independent of the 7/30/90-day range pill. The query
// returns the newest overviewHealthEventsLimit medication events.
//
// All three medication kinds (vaccine, medicine, other) get the same
// "count + most recent + recent history" shape — Vaccinations groups by visit
// (RecentGroupLabel is "Vaccinations at 6 weeks", not one dose's name, since
// several are often logged in the same visit); Medicine and Other report the
// single most recent item's own name instead, since those aren't
// visit-grouped the same way.
type overviewHealthStats struct {
	Available bool `json:"available"`

	VaccinationCount int    `json:"vaccination_count"`
	HasVaccinations  bool   `json:"has_vaccinations"`
	RecentGroupLabel string `json:"recent_group_label,omitempty"`
	RecentDateLabel  string `json:"recent_date_label,omitempty"`
	RecentAgeLabel   string `json:"recent_age_label,omitempty"`
	// VaccineHistory contains vaccine doses from the capped query, newest first.
	VaccineHistory []overviewHealthEvent `json:"vaccine_history,omitempty"`

	MedicineCount           int    `json:"medicine_count"`
	HasMedicine             bool   `json:"has_medicine"`
	RecentMedicineNameLabel string `json:"recent_medicine_name_label,omitempty"`
	RecentMedicineDateLabel string `json:"recent_medicine_date_label,omitempty"`
	RecentMedicineAgeLabel  string `json:"recent_medicine_age_label,omitempty"`
	// MedicineHistory contains medicine doses from the capped query, newest first.
	MedicineHistory []overviewHealthEvent `json:"medicine_history,omitempty"`

	// Other covers medication items logged with kind "other".
	OtherCount           int    `json:"other_count"`
	HasOther             bool   `json:"has_other"`
	RecentOtherNameLabel string `json:"recent_other_name_label,omitempty"`
	RecentOtherDateLabel string `json:"recent_other_date_label,omitempty"`
	RecentOtherAgeLabel  string `json:"recent_other_age_label,omitempty"`
	// OtherHistory contains "other" items from the capped query, newest first.
	OtherHistory []overviewHealthEvent `json:"other_history,omitempty"`
}

// overviewHealthEvent is one recorded vaccine dose, medicine dose, or other
// medication item.
type overviewHealthEvent struct {
	NameLabel        string `json:"name_label"`
	DescriptionLabel string `json:"description_label,omitempty"`
	// WhenLabel is the full "Jan 24, 2026 · 10:35 AM · at 6 weeks" form used
	// in expanded history.
	WhenLabel string `json:"when_label"`
}

func (h *Handlers) overviewHealthStats(ctx context.Context, baby store.Baby, loc *time.Location, now time.Time) overviewHealthStats {
	birthStart, err := growthInsightsBirthStart(baby.BirthDate, loc, now)
	if err != nil {
		log.Printf("overview insights: parse baby birth date %q: %v", baby.BirthDate, err)
		return overviewHealthStats{}
	}

	events, err := h.Store.ListEventsByType(ctx, baby.FamilyID, baby.ID, eventTypeMedication, time.Time{}, now.AddDate(0, 0, 1), overviewHealthEventsLimit)
	if err != nil {
		log.Printf("overview insights: list medication events: %v", err)
		return overviewHealthStats{}
	}

	stats := overviewHealthStats{Available: true}

	// ListEventsByType returns events newest-first. Appending as we iterate
	// preserves that order, and the first event carrying a vaccine item is the
	// most recent vaccination group.
	for _, ev := range events {
		occurredAt := ev.OccurredAt.In(loc)
		var ageLabel string
		if birthStart != nil {
			ageLabel = healthInsightsAgeLabel(*birthStart, occurredAt)
		}

		med := medicationFromEvent(ev)
		var vaccinesInEvent []overviewHealthEvent
		for _, item := range med.Items {
			switch item.Kind {
			case MedicationKindVaccine:
				vaccinesInEvent = append(vaccinesInEvent, overviewHealthEvent{
					NameLabel:        vaccineNameLabel(item),
					DescriptionLabel: item.Description,
					WhenLabel:        healthEventWhenLabel(occurredAt, ageLabel),
				})
			case MedicationKindMedicine:
				stats.MedicineHistory = append(stats.MedicineHistory, overviewHealthEvent{
					NameLabel:        medicineNameLabel(item),
					DescriptionLabel: item.Description,
					WhenLabel:        healthEventWhenLabel(occurredAt, ageLabel),
				})
				if stats.RecentMedicineNameLabel == "" {
					stats.RecentMedicineNameLabel = medicineNameLabel(item)
					stats.RecentMedicineDateLabel = occurredAt.Format("Jan 2")
					stats.RecentMedicineAgeLabel = ageLabel
				}
			case MedicationKindOther:
				stats.OtherHistory = append(stats.OtherHistory, overviewHealthEvent{
					NameLabel:        item.Name,
					DescriptionLabel: item.Description,
					WhenLabel:        healthEventWhenLabel(occurredAt, ageLabel),
				})
				if stats.RecentOtherNameLabel == "" {
					stats.RecentOtherNameLabel = item.Name
					stats.RecentOtherDateLabel = occurredAt.Format("Jan 2")
					stats.RecentOtherAgeLabel = ageLabel
				}
			}
		}

		if len(vaccinesInEvent) > 0 {
			if stats.RecentGroupLabel == "" {
				stats.RecentGroupLabel = "Vaccinations"
				if ageLabel != "" {
					stats.RecentGroupLabel += " " + ageLabel
				}
				stats.RecentDateLabel = occurredAt.Format("Jan 2")
				stats.RecentAgeLabel = ageLabel
			}
			stats.VaccineHistory = append(stats.VaccineHistory, vaccinesInEvent...)
		}
	}

	stats.VaccinationCount = len(stats.VaccineHistory)
	stats.HasVaccinations = stats.VaccinationCount > 0

	stats.MedicineCount = len(stats.MedicineHistory)
	stats.HasMedicine = stats.MedicineCount > 0

	stats.OtherCount = len(stats.OtherHistory)
	stats.HasOther = stats.OtherCount > 0

	return stats
}

// healthInsightsAgeLabel formats a medication event's age at recording the
// same way growth insights format age-at-measurement: under 13 weeks in
// weeks, otherwise in whole calendar months.
func healthInsightsAgeLabel(birthStart, occurredAt time.Time) string {
	// Compare local calendar dates in UTC so daylight-saving transitions do
	// not turn seven calendar days into 167 elapsed hours.
	birthDate := time.Date(birthStart.Year(), birthStart.Month(), birthStart.Day(), 0, 0, 0, 0, time.UTC)
	eventDate := time.Date(occurredAt.Year(), occurredAt.Month(), occurredAt.Day(), 0, 0, 0, 0, time.UTC)
	days := int(eventDate.Sub(birthDate).Hours() / 24)
	if days < 0 {
		days = 0
	}
	weeks := days / 7
	if weeks < 13 {
		return fmt.Sprintf("at %d %s", weeks, pluralWord(weeks, "week", "weeks"))
	}
	months := monthsBetween(birthStart, occurredAt)
	return fmt.Sprintf("at %d %s", months, pluralWord(months, "month", "months"))
}

func monthsBetween(birth, at time.Time) int {
	months := (at.Year()-birth.Year())*12 + int(at.Month()) - int(birth.Month())
	if at.Day() < birth.Day() {
		months--
	}
	if months < 0 {
		months = 0
	}
	return months
}

func pluralWord(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func healthEventWhenLabel(occurredAt time.Time, ageLabel string) string {
	base := occurredAt.Format("Jan 2, 2006 · 3:04 PM")
	if ageLabel == "" {
		return base
	}
	return base + " · " + ageLabel
}

func vaccineNameLabel(item medicationItemResponse) string {
	if item.SeriesDose == "" {
		return item.Name
	}
	return item.Name + " · " + seriesDoseLabel(item.SeriesDose)
}

func seriesDoseLabel(dose MedicationSeriesDose) string {
	switch dose {
	case MedicationSeriesDoseFirst:
		return "Dose 1"
	case MedicationSeriesDoseSecond:
		return "Dose 2"
	case MedicationSeriesDoseThird:
		return "Dose 3"
	case MedicationSeriesDoseBooster:
		return "Booster"
	default:
		return ""
	}
}

func medicineNameLabel(item medicationItemResponse) string {
	if item.DoseValue == nil {
		return item.Name
	}
	return item.Name + " · " + doseAmountLabel(*item.DoseValue, item.DoseUnit)
}

func doseAmountLabel(value float64, unit MedicationDoseUnit) string {
	amount := strconv.FormatFloat(value, 'f', -1, 64)
	plural := value != 1

	var unitLabel string
	switch unit {
	case MedicationDoseUnitDrops:
		unitLabel = "drop"
		if plural {
			unitLabel = "drops"
		}
	case MedicationDoseUnitDose:
		unitLabel = "dose"
		if plural {
			unitLabel = "doses"
		}
	case MedicationDoseUnitOther:
		unitLabel = "unit"
		if plural {
			unitLabel = "units"
		}
	default:
		unitLabel = string(unit)
	}
	return amount + " " + unitLabel
}
