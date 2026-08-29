package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

type medicationCreateStore struct {
	*aiReportFakeStore
	createdAttributes map[string]any
	createdAt         time.Time
}

func (s *medicationCreateStore) CreateEvent(_ context.Context, _, _ uuid.UUID, eventType string, attributes map[string]any, occurredAt time.Time) (store.Event, error) {
	s.createdAttributes = attributes
	s.createdAt = occurredAt
	return store.Event{
		ID: uuid.New(), BabyID: s.baby.ID, EventType: eventType,
		Attributes: attributes, OccurredAt: occurredAt, CreatedAt: occurredAt,
	}, nil
}

func TestMedicationItemAttributes(t *testing.T) {
	doseValue := 2.5
	attrs, ok := medicationItemAttributes(httptest.NewRecorder(), MedicationKindMedicine, " Infant paracetamol ", &doseValue, MedicationDoseUnitML, "")
	if !ok {
		t.Fatal("medicationItemAttributes rejected valid medicine")
	}
	if attrs["name"] != "Infant paracetamol" || attrs["dose_value"] != 2.5 || attrs["dose_unit"] != "ml" {
		t.Fatalf("medicine attributes = %#v", attrs)
	}

	vaccineAttrs, ok := medicationItemAttributes(httptest.NewRecorder(), MedicationKindVaccine, "Rotavirus", nil, "", MedicationSeriesDoseFirst)
	if !ok {
		t.Fatal("medicationItemAttributes rejected valid vaccine")
	}
	if vaccineAttrs["series_dose"] != "first" || vaccineAttrs["kind"] != "vaccine" {
		t.Fatalf("vaccine attributes = %#v", vaccineAttrs)
	}

	otherAttrs, ok := medicationItemAttributes(httptest.NewRecorder(), MedicationKindOther, "Vitamin drops", nil, "", "")
	if !ok || otherAttrs["kind"] != "other" || otherAttrs["name"] != "Vitamin drops" {
		t.Fatalf("other attributes = %#v", otherAttrs)
	}
}

func TestMedicationItemAttributesRejectsInvalidCombinations(t *testing.T) {
	doseValue := 2.5
	tests := []struct {
		name       string
		kind       MedicationKind
		medName    string
		doseValue  *float64
		doseUnit   MedicationDoseUnit
		seriesDose MedicationSeriesDose
	}{
		{name: "invalid kind", kind: "supplement", medName: "Vitamin D"},
		{name: "missing name", kind: MedicationKindMedicine},
		{name: "dose without unit", kind: MedicationKindMedicine, medName: "Paracetamol", doseValue: &doseValue},
		{name: "unit without dose", kind: MedicationKindMedicine, medName: "Paracetamol", doseUnit: MedicationDoseUnitML},
		{name: "vaccine with medicine dose", kind: MedicationKindVaccine, medName: "Rotavirus", doseValue: &doseValue, doseUnit: MedicationDoseUnitML},
		{name: "medicine with series dose", kind: MedicationKindMedicine, medName: "Paracetamol", seriesDose: MedicationSeriesDoseFirst},
		{name: "unsupported vaccine series", kind: MedicationKindVaccine, medName: "Rotavirus", seriesDose: "fourth"},
		{name: "other with medicine dose", kind: MedicationKindOther, medName: "Supplement", doseValue: &doseValue, doseUnit: MedicationDoseUnitML},
		{name: "other with vaccine series", kind: MedicationKindOther, medName: "Supplement", seriesDose: MedicationSeriesDoseFirst},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := medicationItemAttributes(httptest.NewRecorder(), test.kind, test.medName, test.doseValue, test.doseUnit, test.seriesDose); ok {
				t.Fatal("medicationItemAttributes accepted invalid attributes")
			}
		})
	}
}

func TestNormalizeEventAttributesSupportsMultiItemMedicationUpdates(t *testing.T) {
	attrs, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeMedication, map[string]any{
		"items": []any{
			map[string]any{"kind": "vaccine", "name": "Rotavirus", "series_dose": "first"},
			map[string]any{"kind": "medicine", "name": "Paracetamol", "dose_value": float64(2.5), "dose_unit": "ml"},
		},
		"notes": "6 week appointment",
	})
	if !ok {
		t.Fatal("normalizeEventAttributes rejected valid medication event")
	}
	items, valid := medicationEventItemMaps(attrs)
	if !valid || len(items) != 2 || items[0]["name"] != "Rotavirus" || items[1]["name"] != "Paracetamol" {
		t.Fatalf("normalized attributes = %#v", attrs)
	}
	if attrs["notes"] != "6 week appointment" {
		t.Fatalf("normalized notes = %#v", attrs["notes"])
	}
}

func TestNormalizeEventAttributesRejectsMalformedMedicationUpdates(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]any
	}{
		{
			name:       "missing items array",
			attributes: map[string]any{"kind": "medicine", "name": "Paracetamol"},
		},
		{
			name: "dose value has wrong type",
			attributes: map[string]any{"items": []any{
				map[string]any{"kind": "medicine", "name": "Paracetamol", "dose_value": "2.5"},
			}},
		},
		{
			name: "dose unit has wrong type",
			attributes: map[string]any{"items": []any{
				map[string]any{"kind": "medicine", "name": "Paracetamol", "dose_unit": true},
			}},
		},
		{
			name: "series dose has wrong type",
			attributes: map[string]any{"items": []any{
				map[string]any{"kind": "vaccine", "name": "Rotavirus", "series_dose": float64(1)},
			}},
		},
		{
			name: "notes have wrong type",
			attributes: map[string]any{
				"items": []any{map[string]any{"kind": "vaccine", "name": "Rotavirus"}},
				"notes": true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeMedication, test.attributes); ok {
				t.Fatalf("normalizeEventAttributes accepted %#v", test.attributes)
			}
		})
	}
}

func TestCreateMedicationStoresOneEventWithMultipleItems(t *testing.T) {
	familyID := uuid.New()
	babyID := uuid.New()
	fake := &medicationCreateStore{aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{
		ID: babyID, FamilyID: familyID, Timezone: "Australia/Adelaide",
	}}}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, `{
		"items": [
			{"kind":"vaccine","name":"Rotavirus","series_dose":"first"},
			{"kind":"vaccine","name":"Pneumococcal","series_dose":"first"},
			{"kind":"medicine","name":"Infant paracetamol","dose_value":2.5,"dose_unit":"ml"}
		],
		"notes":" 6 week appointment ",
		"occurred_at":"2026-08-28T10:30:00+09:30"
	}`)
	req.Method = http.MethodPost
	recorder := httptest.NewRecorder()

	h.CreateMedication(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	items, valid := medicationEventItemMaps(fake.createdAttributes)
	if !valid || len(items) != 3 {
		t.Fatalf("created attributes = %#v, want one event with 3 items", fake.createdAttributes)
	}
	if fake.createdAttributes["notes"] != "6 week appointment" {
		t.Fatalf("created notes = %#v", fake.createdAttributes["notes"])
	}
	if _, exists := fake.createdAttributes["session_id"]; exists {
		t.Fatalf("created event unexpectedly has session_id: %#v", fake.createdAttributes)
	}
	if fake.createdAt.Format(time.RFC3339) != "2026-08-28T10:30:00+09:30" {
		t.Fatalf("createdAt = %s", fake.createdAt.Format(time.RFC3339))
	}

	var response medicationResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID == uuid.Nil || len(response.Items) != 3 || response.Notes != "6 week appointment" {
		t.Fatalf("response = %#v", response)
	}
}

func TestCreateMedicationValidatesEveryItemBeforeWriting(t *testing.T) {
	familyID := uuid.New()
	fake := &medicationCreateStore{aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{
		ID: uuid.New(), FamilyID: familyID, Timezone: "UTC",
	}}}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, `{
		"items": [
			{"kind":"vaccine","name":"Rotavirus"},
			{"kind":"medicine","name":"Paracetamol","dose_value":2.5}
		],
		"occurred_at":"2026-08-28T10:30:00Z"
	}`)
	req.Method = http.MethodPost
	recorder := httptest.NewRecorder()

	h.CreateMedication(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if fake.createdAttributes != nil {
		t.Fatalf("created attributes = %#v, want no write", fake.createdAttributes)
	}
}

func TestCreateMedicationRejectsEmptyItems(t *testing.T) {
	familyID := uuid.New()
	fake := &medicationCreateStore{aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{
		ID: uuid.New(), FamilyID: familyID, Timezone: "UTC",
	}}}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, `{"items":[],"occurred_at":"2026-08-28T10:30:00Z"}`)
	req.Method = http.MethodPost
	recorder := httptest.NewRecorder()

	h.CreateMedication(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if fake.createdAttributes != nil {
		t.Fatalf("created attributes = %#v, want no write", fake.createdAttributes)
	}
}
