package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

func TestFeedAmountFromFormIgnoresBreastAmount(t *testing.T) {
	amount, err := feedAmountFromForm("breast", "80")
	if err != nil {
		t.Fatalf("feedAmountFromForm returned error: %v", err)
	}
	if amount != nil {
		t.Fatalf("feedAmountFromForm breast amount = %v, want nil", *amount)
	}
}

func TestFeedAmountFromFormRequiresBottleAmount(t *testing.T) {
	for _, feedType := range []string{"formula", "expressed"} {
		t.Run(feedType, func(t *testing.T) {
			if _, err := feedAmountFromForm(feedType, ""); err == nil {
				t.Fatalf("feedAmountFromForm accepted empty %s amount", feedType)
			}
			if _, err := feedAmountFromForm(feedType, "0"); err == nil {
				t.Fatalf("feedAmountFromForm accepted zero %s amount", feedType)
			}
		})
	}
}

func TestBathUpdatePayloadUsesEditedSettings(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	tests := []struct {
		name           string
		bathType       string
		time           string
		timeBasis      string
		wantOccurredAt string
	}{
		{name: "type and clock time", bathType: "bottom_part", time: "08:15", timeBasis: "start", wantOccurredAt: "2026-07-20T08:15:00+09:30"},
		{name: "end time", bathType: "whole_body", time: "08:15", timeBasis: "end", wantOccurredAt: "2026-07-20T08:05:00+09:30"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := url.Values{
				"event_type":       {"bath"},
				"type":             {test.bathType},
				"date":             {"2026-07-20"},
				"time":             {test.time},
				"bath_time_basis":  {test.timeBasis},
				"duration_minutes": {"10"},
			}
			req := httptest.NewRequest(http.MethodPatch, "/events/bath-id", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if err := req.ParseForm(); err != nil {
				t.Fatalf("ParseForm returned error: %v", err)
			}

			payload, err := (&Handlers{}).eventUpdatePayloadFromForm(loc, req)
			if err != nil {
				t.Fatalf("eventUpdatePayloadFromForm returned error: %v", err)
			}
			if payload["occurred_at"] != test.wantOccurredAt {
				t.Errorf("occurred_at = %q, want %q", payload["occurred_at"], test.wantOccurredAt)
			}
			attributes, ok := payload["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("attributes = %#v, want map[string]any", payload["attributes"])
			}
			if attributes["type"] != test.bathType {
				t.Errorf("type = %q, want %q", attributes["type"], test.bathType)
			}
		})
	}
}

func TestPumpUpdatePayloadPreservesDuration(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	form := url.Values{
		"event_type":       {"pump"},
		"amount_ml":        {"80"},
		"duration_minutes": {"15"},
		"date":             {"2026-07-20"},
		"time":             {"08:15"},
	}
	req := httptest.NewRequest(http.MethodPatch, "/events/pump-id", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}

	payload, err := (&Handlers{}).eventUpdatePayloadFromForm(loc, req)
	if err != nil {
		t.Fatalf("eventUpdatePayloadFromForm returned error: %v", err)
	}
	attributes := payload["attributes"].(map[string]any)
	duration, ok := attributes["duration_minutes"].(*int)
	if !ok || *duration != 15 {
		t.Fatalf("duration_minutes = %#v, want 15", attributes["duration_minutes"])
	}
	if _, exists := attributes["ongoing"]; exists {
		t.Fatalf("completed pump gained ongoing marker: %#v", attributes)
	}
}

func TestPumpUpdatePayloadWithoutDurationRemainsOngoing(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	form := url.Values{
		"event_type": {"pump"},
		"amount_ml":  {"80"},
		"date":       {"2026-07-20"},
		"time":       {"08:15"},
	}
	req := httptest.NewRequest(http.MethodPatch, "/events/pump-id", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}

	payload, err := (&Handlers{}).eventUpdatePayloadFromForm(loc, req)
	if err != nil {
		t.Fatalf("eventUpdatePayloadFromForm returned error: %v", err)
	}
	attributes := payload["attributes"].(map[string]any)
	duration, ok := attributes["duration_minutes"].(*int)
	if !ok || duration != nil {
		t.Fatalf("duration_minutes = %#v, want nil", attributes["duration_minutes"])
	}
}

func TestMedicationUpdatePayloadContainsAllItems(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	form := url.Values{
		"event_type":         {"medication"},
		"medication_item":    {"0", "1"},
		"item_kind_0":        {"vaccine"},
		"item_name_0":        {"Rotavirus"},
		"item_series_dose_0": {"first"},
		"item_kind_1":        {"medicine"},
		"item_name_1":        {"Infant paracetamol"},
		"item_description_1": {"For fever after immunisations"},
		"item_dose_value_1":  {"2.5"},
		"item_dose_unit_1":   {"ml"},
		"notes":              {"6 week appointment"},
		"date":               {"2026-07-20"},
		"time":               {"08:15"},
	}
	req := httptest.NewRequest(http.MethodPatch, "/events/medication-id", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}

	payload, err := (&Handlers{}).eventUpdatePayloadFromForm(loc, req)
	if err != nil {
		t.Fatalf("eventUpdatePayloadFromForm returned error: %v", err)
	}
	attributes := payload["attributes"].(map[string]any)
	items := attributes["items"].([]map[string]any)
	if len(items) != 2 || items[0]["name"] != "Rotavirus" || items[0]["series_dose"] != "first" {
		t.Fatalf("vaccine item = %#v", items)
	}
	if items[1]["name"] != "Infant paracetamol" || items[1]["description"] != "For fever after immunisations" || items[1]["dose_value"] != 2.5 || items[1]["dose_unit"] != "ml" {
		t.Fatalf("medicine item = %#v", items)
	}
	if attributes["notes"] != "6 week appointment" {
		t.Fatalf("attributes = %#v", attributes)
	}
}

func TestMedicationUpdatePayloadAllowsOneRemainingItemAfterRemoval(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	form := url.Values{
		"event_type":      {"medication"},
		"medication_item": {"1"},
		"item_kind_1":     {"other"},
		"item_name_1":     {"Vitamin drops"},
		"date":            {"2026-07-20"},
		"time":            {"08:15"},
	}
	req := httptest.NewRequest(http.MethodPatch, "/events/medication-id", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}

	payload, err := (&Handlers{}).eventUpdatePayloadFromForm(loc, req)
	if err != nil {
		t.Fatalf("eventUpdatePayloadFromForm returned error: %v", err)
	}
	items := payload["attributes"].(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["kind"] != "other" || items[0]["name"] != "Vitamin drops" {
		t.Fatalf("remaining medication items = %#v", items)
	}
}

func TestMedicationItemsFromFormBuildsMixedEvent(t *testing.T) {
	form := url.Values{
		"medication_item":    {"0", "1", "2", "3"},
		"item_kind_0":        {"vaccine"},
		"item_name_0":        {"Rotavirus"},
		"item_series_dose_0": {"first"},
		"item_kind_1":        {"vaccine"},
		"item_name_1":        {"Pneumococcal"},
		"item_series_dose_1": {"first"},
		"item_kind_2":        {"medicine"},
		"item_name_2":        {"Infant paracetamol"},
		"item_description_2": {"For fever after immunisations"},
		"item_dose_value_2":  {"2.5"},
		"item_dose_unit_2":   {"ml"},
		"item_kind_3":        {"other"},
		"item_name_3":        {"Vitamin drops"},
	}
	req := httptest.NewRequest(http.MethodPost, "/medications", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	items, err := medicationItemsFromForm(req)
	if err != nil {
		t.Fatalf("medicationItemsFromForm returned error: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %#v, want 4", items)
	}
	if items[0]["kind"] != "vaccine" || items[0]["series_dose"] != "first" || items[0]["name"] != "Rotavirus" {
		t.Fatalf("first vaccine = %#v", items[0])
	}
	if items[2]["kind"] != "medicine" || items[2]["description"] != "For fever after immunisations" || items[2]["dose_value"] != 2.5 || items[2]["dose_unit"] != "ml" {
		t.Fatalf("medicine = %#v", items[2])
	}
	if items[3]["kind"] != "other" || items[3]["name"] != "Vitamin drops" {
		t.Fatalf("other item = %#v", items[3])
	}
}

func TestMedicationItemsFromFormRejectsMissingItemsAndInvalidDose(t *testing.T) {
	tests := []url.Values{
		{},
		{
			"medication_item":   {"0"},
			"item_kind_0":       {"medicine"},
			"item_name_0":       {"Paracetamol"},
			"item_dose_value_0": {"not-a-number"},
		},
	}
	for _, form := range tests {
		req := httptest.NewRequest(http.MethodPost, "/medications", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if _, err := medicationItemsFromForm(req); err == nil {
			t.Fatalf("medicationItemsFromForm accepted %#v", form)
		}
	}
}

func TestFeedTimelineEventMarksMissingDurationOngoing(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "feed",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"type":      "expressed",
			"amount_ml": float64(80),
			"labels":    []any{"burped_after"},
		},
	}

	timelineEvent := feedTimelineEvent(ev, loc, occurredAt.Add(15*time.Minute))
	if !timelineEvent.Ongoing {
		t.Fatal("Ongoing = false, want true")
	}
	if !timelineEvent.CanFinishFeed {
		t.Fatal("CanFinishFeed = false, want true")
	}
	if timelineEvent.DurationMinutes != "" {
		t.Fatalf("DurationMinutes = %q, want empty", timelineEvent.DurationMinutes)
	}
	if timelineEvent.AmountMl != "80" {
		t.Fatalf("AmountMl = %q, want 80", timelineEvent.AmountMl)
	}
	if timelineEvent.DurationLabel != "15m" || timelineEvent.LiveStatusLabel != "feeding now" {
		t.Fatalf("live duration = %q/%q, want 15m/feeding now", timelineEvent.DurationLabel, timelineEvent.LiveStatusLabel)
	}
	if timelineEvent.LiveDurationStartMS != strconv.FormatInt(occurredAt.UnixMilli(), 10) {
		t.Fatalf("LiveDurationStartMS = %q, want event start", timelineEvent.LiveDurationStartMS)
	}
	if timelineEvent.FinishedTime != "" || timelineEvent.FinishedTimeValue != "" {
		t.Fatalf("ongoing feed has finished time %q/%q", timelineEvent.FinishedTime, timelineEvent.FinishedTimeValue)
	}
}

func TestMedicationTimelineEventFormatsMultipleItems(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)

	medication := medicationTimelineEvent(backendclient.Event{
		EventType:  "medication",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"items": []any{
				map[string]any{"kind": "vaccine", "name": "Rotavirus", "series_dose": "first"},
				map[string]any{"kind": "medicine", "name": "Infant paracetamol", "description": "For fever after immunisations", "dose_value": float64(2.5), "dose_unit": "ml"},
				map[string]any{"kind": "other", "name": "Vitamin drops"},
			},
			"notes": "6 week appointment",
		},
	}, loc, occurredAt.Add(time.Minute))
	if medication.TypeLabel != "Medication" || medication.InlineDetail != "3 items" {
		t.Fatalf("medication timeline event = %#v", medication)
	}
	if got := medication.MedicationItemRows; len(got) != 3 || got[0] != "Vaccine · Rotavirus · First dose" || got[1] != "Medicine · Infant paracetamol · For fever after immunisations · 2.5 mL" || got[2] != "Other · Vitamin drops" {
		t.Fatalf("medication item rows = %#v", got)
	}
	if !strings.Contains(medication.MedicationItemsJSON, `"name":"Rotavirus"`) || !strings.Contains(medication.MedicationItemsJSON, `"description":"For fever after immunisations"`) {
		t.Fatalf("medication edit JSON = %q", medication.MedicationItemsJSON)
	}
}

func TestMedicationTimelineEventFallsBackToValidEditJSON(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)

	medication := medicationTimelineEvent(backendclient.Event{
		ID:         "medication-1",
		EventType:  "medication",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"items": []map[string]any{{
				"kind":         "medicine",
				"name":         "Infant paracetamol",
				"invalid_json": make(chan struct{}),
			}},
		},
	}, loc, occurredAt.Add(time.Minute))

	if medication.MedicationItemsJSON != "[]" {
		t.Fatalf("medication edit JSON = %q, want valid empty array fallback", medication.MedicationItemsJSON)
	}
}

func TestPumpTimelineEventMarksMissingDurationOngoing(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)

	ongoing := pumpTimelineEvent(backendclient.Event{
		EventType:  "pump",
		OccurredAt: occurredAt,
		Attributes: map[string]any{"amount_ml": float64(80)},
	}, loc, occurredAt.Add(15*time.Minute))
	if !ongoing.Ongoing || !ongoing.CanFinishPump {
		t.Fatalf("duration-less pump not marked ongoing: %#v", ongoing)
	}
	if ongoing.DurationLabel != "15m" || ongoing.LiveStatusLabel != "pumping now" || ongoing.LiveDurationStartMS == "" {
		t.Fatalf("ongoing pump live duration = %#v", ongoing)
	}

	completed := pumpTimelineEvent(backendclient.Event{
		EventType:  "pump",
		OccurredAt: occurredAt,
		Attributes: map[string]any{"amount_ml": float64(80), "duration_minutes": float64(15)},
	}, loc, occurredAt.Add(15*time.Minute))
	if completed.Ongoing || completed.CanFinishPump {
		t.Fatalf("completed pump marked ongoing: %#v", completed)
	}
}

func TestNappyTimelineEventUsesPlainPooSizeLabel(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "nappy",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"kind":     "both",
			"poo_size": "large",
		},
	}

	timelineEvent := nappyTimelineEvent(ev, loc, occurredAt.Add(15*time.Minute))
	if timelineEvent.Detail != "Large" {
		t.Fatalf("Detail = %q, want Large", timelineEvent.Detail)
	}
	if timelineEvent.PooSizeValue != "large" {
		t.Fatalf("PooSizeValue = %q, want large", timelineEvent.PooSizeValue)
	}
}

func TestNappyTimelineEventUsesKindAsLabel(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{kind: "wet", want: "Wee"},
		{kind: "both", want: "Wee Poo"},
		{kind: "poo", want: "Poo"},
	}

	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			ev := backendclient.Event{
				EventType:  "nappy",
				OccurredAt: occurredAt,
				Attributes: map[string]any{"kind": test.kind},
			}

			timelineEvent := nappyTimelineEvent(ev, loc, occurredAt.Add(15*time.Minute))
			if timelineEvent.TypeLabel != test.want {
				t.Fatalf("TypeLabel = %q, want %q", timelineEvent.TypeLabel, test.want)
			}
			if timelineEvent.Kind != "" {
				t.Fatalf("Kind = %q, want empty", timelineEvent.Kind)
			}
			if timelineEvent.KindValue != test.kind {
				t.Fatalf("KindValue = %q, want %q", timelineEvent.KindValue, test.kind)
			}
		})
	}
}

func TestFormatDurationMinutes(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{minutes: 42, want: "42m"},
		{minutes: 60, want: "1h"},
		{minutes: 61, want: "1h 1m"},
		{minutes: 105, want: "1h 45m"},
		{minutes: 152, want: "2h 32m"},
		{minutes: 120, want: "2h"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDurationMinutes(tt.minutes); got != tt.want {
				t.Fatalf("formatDurationMinutes(%d) = %q, want %q", tt.minutes, got, tt.want)
			}
		})
	}
}

func TestSleepTimelineEventFormatsDurationAsHoursAndMinutes(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 16, 30, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "sleep",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"type":             "nap",
			"duration_minutes": float64(105),
		},
	}

	timelineEvent := sleepTimelineEvent(ev, loc, occurredAt.Add(105*time.Minute))

	if timelineEvent.DurationLabel != "1h 45m" || timelineEvent.FinishedTime != "6:15 PM" {
		t.Fatalf("duration = %q ending %q, want 1h 45m ending 6:15 PM", timelineEvent.DurationLabel, timelineEvent.FinishedTime)
	}
	if timelineEvent.Detail != "" {
		t.Fatalf("Detail = %q, want duration kept in its own row", timelineEvent.Detail)
	}
}

func TestBathTimelineEventFormatsDurationAsHoursAndMinutes(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 16, 30, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "bath",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"type":             "bath",
			"duration_minutes": float64(60),
		},
	}

	timelineEvent := bathTimelineEvent(ev, loc, occurredAt.Add(60*time.Minute))

	if timelineEvent.DurationLabel != "1h" || timelineEvent.FinishedTime != "5:30 PM" {
		t.Fatalf("duration = %q ending %q, want 1h ending 5:30 PM", timelineEvent.DurationLabel, timelineEvent.FinishedTime)
	}
}

func TestFeedTimelineEventFormatsDurationAsHoursAndMinutes(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "feed",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"type":             "breast",
			"duration_minutes": float64(152),
		},
	}

	timelineEvent := feedTimelineEvent(ev, loc, occurredAt.Add(152*time.Minute))

	if timelineEvent.DurationLabel != "2h 32m" || timelineEvent.FinishedTime != "11:47 AM" {
		t.Fatalf("duration = %q ending %q, want 2h 32m ending 11:47 AM", timelineEvent.DurationLabel, timelineEvent.FinishedTime)
	}
}

func TestPumpTimelineEventFormatsDurationAsHoursAndMinutes(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "pump",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"amount_ml":        float64(80),
			"duration_minutes": float64(61),
		},
	}

	timelineEvent := pumpTimelineEvent(ev, loc, occurredAt.Add(61*time.Minute))

	if timelineEvent.DurationLabel != "1h 1m" || timelineEvent.FinishedTime != "10:16 AM" {
		t.Fatalf("duration = %q ending %q, want 1h 1m ending 10:16 AM", timelineEvent.DurationLabel, timelineEvent.FinishedTime)
	}
}

func TestSleepTimelineEventMarksMissingDurationOngoing(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 13, 53, 0, 0, loc)
	timelineEvent := sleepTimelineEvent(backendclient.Event{
		EventType:  "sleep",
		OccurredAt: occurredAt,
		Attributes: map[string]any{"type": "nap"},
	}, loc, occurredAt.Add(28*time.Minute))

	if timelineEvent.DurationLabel != "28m" || timelineEvent.LiveStatusLabel != "sleeping now" {
		t.Fatalf("live duration = %q/%q, want 28m/sleeping now", timelineEvent.DurationLabel, timelineEvent.LiveStatusLabel)
	}
	if !timelineEvent.Ongoing || !timelineEvent.CanFinishSleep || timelineEvent.LiveDurationStartMS == "" {
		t.Fatalf("duration-less sleep not marked ongoing: %#v", timelineEvent)
	}
}

func TestCompletedTimelineDurationIncludesDateWhenEventEndsAnotherDay(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	startedAt := time.Date(2026, 7, 14, 23, 30, 0, 0, loc)
	duration, finishedTime, finishedTimeValue := completedTimelineDuration(
		map[string]any{"duration_minutes": float64(90)},
		startedAt,
	)

	if duration != "1h 30m" || finishedTime != "Jul 15, 1:00 AM" {
		t.Fatalf("completed duration = %q ending %q", duration, finishedTime)
	}
	if finishedTimeValue != "2026-07-15T01:00:00+09:30" {
		t.Fatalf("FinishedTimeValue = %q, want RFC3339 baby-local finish time", finishedTimeValue)
	}
}

func TestSleepTimelineEventUsesSleepTypeAsLabel(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 16, 30, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "sleep",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"type":             "nap",
			"duration_minutes": float64(10),
		},
	}

	timelineEvent := sleepTimelineEvent(ev, loc, occurredAt.Add(10*time.Minute))

	if timelineEvent.TypeLabel != "Nap" {
		t.Fatalf("TypeLabel = %q, want Nap", timelineEvent.TypeLabel)
	}
	if timelineEvent.Kind != "" {
		t.Fatalf("Kind = %q, want empty", timelineEvent.Kind)
	}
	if timelineEvent.TypeValue != "nap" {
		t.Fatalf("TypeValue = %q, want nap", timelineEvent.TypeValue)
	}
}

func TestGrowthMeasurementTimelineEventPrefillsEditValues(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "growth_measurement",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"weight_grams":          float64(3135),
			"length_cm":             float64(52.4),
			"head_circumference_cm": float64(35.7),
			"notes":                 "checkup",
		},
	}

	timelineEvent := growthMeasurementTimelineEvent(ev, loc, occurredAt.Add(15*time.Minute))

	if timelineEvent.WeightKg != "3.135" {
		t.Fatalf("WeightKg = %q, want 3.135", timelineEvent.WeightKg)
	}
	if timelineEvent.LengthCM != "52.4" {
		t.Fatalf("LengthCM = %q, want 52.4", timelineEvent.LengthCM)
	}
	if timelineEvent.HeadCircumferenceCM != "35.7" {
		t.Fatalf("HeadCircumferenceCM = %q, want 35.7", timelineEvent.HeadCircumferenceCM)
	}
	if timelineEvent.Notes != "checkup" {
		t.Fatalf("Notes = %q, want checkup", timelineEvent.Notes)
	}
	if timelineEvent.Detail != "3.135 kg · Length 52.4 cm · Head 35.7 cm · checkup" {
		t.Fatalf("Detail = %q", timelineEvent.Detail)
	}
}

func TestGrowthMeasurementTimelineEventAcceptsStoredNumberTypes(t *testing.T) {
	loc := time.FixedZone("ACST", 9*60*60+30*60)
	occurredAt := time.Date(2026, 7, 14, 9, 15, 0, 0, loc)
	ev := backendclient.Event{
		EventType:  "growth_measurement",
		OccurredAt: occurredAt,
		Attributes: map[string]any{
			"weight_grams":          int64(3135),
			"length_cm":             json.Number("52.4"),
			"head_circumference_cm": json.Number("35.7"),
		},
	}

	timelineEvent := growthMeasurementTimelineEvent(ev, loc, occurredAt.Add(15*time.Minute))

	if timelineEvent.WeightKg != "3.135" || timelineEvent.LengthCM != "52.4" || timelineEvent.HeadCircumferenceCM != "35.7" {
		t.Fatalf("growth edit values = weight %q length %q head %q, want 3.135/52.4/35.7", timelineEvent.WeightKg, timelineEvent.LengthCM, timelineEvent.HeadCircumferenceCM)
	}
}
