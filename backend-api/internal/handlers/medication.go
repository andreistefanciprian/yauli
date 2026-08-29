package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

const eventTypeMedication = "medication"

type MedicationKind string

const (
	MedicationKindMedicine MedicationKind = "medicine"
	MedicationKindVaccine  MedicationKind = "vaccine"
	MedicationKindOther    MedicationKind = "other"
)

func (k MedicationKind) Valid() bool {
	switch k {
	case MedicationKindMedicine, MedicationKindVaccine, MedicationKindOther:
		return true
	default:
		return false
	}
}

type MedicationDoseUnit string

const (
	MedicationDoseUnitML    MedicationDoseUnit = "ml"
	MedicationDoseUnitMG    MedicationDoseUnit = "mg"
	MedicationDoseUnitDrops MedicationDoseUnit = "drops"
	MedicationDoseUnitDose  MedicationDoseUnit = "dose"
	MedicationDoseUnitOther MedicationDoseUnit = "other"
)

func (u MedicationDoseUnit) Valid() bool {
	switch u {
	case MedicationDoseUnitML, MedicationDoseUnitMG, MedicationDoseUnitDrops, MedicationDoseUnitDose, MedicationDoseUnitOther:
		return true
	default:
		return false
	}
}

type MedicationSeriesDose string

const (
	MedicationSeriesDoseFirst   MedicationSeriesDose = "first"
	MedicationSeriesDoseSecond  MedicationSeriesDose = "second"
	MedicationSeriesDoseThird   MedicationSeriesDose = "third"
	MedicationSeriesDoseBooster MedicationSeriesDose = "booster"
)

func (d MedicationSeriesDose) Valid() bool {
	switch d {
	case "", MedicationSeriesDoseFirst, MedicationSeriesDoseSecond, MedicationSeriesDoseThird, MedicationSeriesDoseBooster:
		return true
	default:
		return false
	}
}

type createMedicationRequest struct {
	Items      []createMedicationItemRequest `json:"items"`
	Notes      string                        `json:"notes"`
	OccurredAt string                        `json:"occurred_at"`
}

type createMedicationItemRequest struct {
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	DoseValue  *float64 `json:"dose_value"`
	DoseUnit   string   `json:"dose_unit"`
	SeriesDose string   `json:"series_dose"`
}

type medicationItemResponse struct {
	Kind       MedicationKind       `json:"kind"`
	Name       string               `json:"name"`
	DoseValue  *float64             `json:"dose_value,omitempty"`
	DoseUnit   MedicationDoseUnit   `json:"dose_unit,omitempty"`
	SeriesDose MedicationSeriesDose `json:"series_dose,omitempty"`
}

type medicationResponse struct {
	ID         uuid.UUID                `json:"id"`
	BabyID     uuid.UUID                `json:"baby_id"`
	Items      []medicationItemResponse `json:"items"`
	Notes      string                   `json:"notes,omitempty"`
	OccurredAt time.Time                `json:"occurred_at"`
	CreatedAt  time.Time                `json:"created_at"`
}

func (h *Handlers) CreateMedication(w http.ResponseWriter, r *http.Request) {
	var req createMedicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	occurredAt, ok := parseOccurredAt(w, req.OccurredAt)
	if !ok {
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items must contain at least one medication")
		return
	}

	attributes, ok := medicationEventAttributes(w, req.Items, req.Notes)
	if !ok {
		return
	}
	createAndRespond(w, r, h, eventTypeMedication, attributes, occurredAt, medicationFromEvent)
}

func medicationEventAttributes(w http.ResponseWriter, items []createMedicationItemRequest, notes string) (map[string]any, bool) {
	itemAttributes := make([]map[string]any, len(items))
	for i, item := range items {
		var ok bool
		itemAttributes[i], ok = medicationItemAttributes(w, MedicationKind(item.Kind), item.Name, item.DoseValue, MedicationDoseUnit(item.DoseUnit), MedicationSeriesDose(item.SeriesDose))
		if !ok {
			return nil, false
		}
	}

	attributes := map[string]any{"items": itemAttributes}
	if notes = strings.TrimSpace(notes); notes != "" {
		attributes["notes"] = notes
	}
	return attributes, true
}

func normalizeMedicationEventAttributes(w http.ResponseWriter, raw map[string]any) (map[string]any, bool) {
	rawItems, ok := medicationEventItemMaps(raw)
	if !ok || len(rawItems) == 0 {
		writeError(w, http.StatusBadRequest, "items must contain at least one medication")
		return nil, false
	}

	items := make([]createMedicationItemRequest, len(rawItems))
	for i, item := range rawItems {
		doseUnit, ok := optionalMedicationString(item, "dose_unit")
		if !ok {
			writeError(w, http.StatusBadRequest, "dose_unit must be a string")
			return nil, false
		}
		seriesDose, ok := optionalMedicationString(item, "series_dose")
		if !ok {
			writeError(w, http.StatusBadRequest, "series_dose must be a string")
			return nil, false
		}
		items[i] = createMedicationItemRequest{
			Kind:       attributeString(item, "kind"),
			Name:       attributeString(item, "name"),
			DoseUnit:   doseUnit,
			SeriesDose: seriesDose,
		}
		if rawDoseValue, exists := item["dose_value"]; exists && rawDoseValue != nil {
			doseValue, valid := attributeFloat(item, "dose_value")
			if !valid {
				writeError(w, http.StatusBadRequest, "dose_value must be a number")
				return nil, false
			}
			items[i].DoseValue = &doseValue
		}
	}
	notes, ok := optionalMedicationString(raw, "notes")
	if !ok {
		writeError(w, http.StatusBadRequest, "notes must be a string")
		return nil, false
	}
	return medicationEventAttributes(w, items, notes)
}

func optionalMedicationString(attributes map[string]any, key string) (string, bool) {
	value, exists := attributes[key]
	if !exists || value == nil {
		return "", true
	}
	result, ok := value.(string)
	return result, ok
}

func medicationEventItemMaps(attributes map[string]any) ([]map[string]any, bool) {
	value, hasItems := attributes["items"]
	if !hasItems {
		return nil, false
	}

	switch items := value.(type) {
	case []map[string]any:
		return items, true
	case []any:
		result := make([]map[string]any, len(items))
		for i, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			result[i] = itemMap
		}
		return result, true
	default:
		return nil, false
	}
}

func medicationFromEvent(ev store.Event) medicationResponse {
	resp := medicationResponse{
		ID: ev.ID, BabyID: ev.BabyID, Notes: attributeString(ev.Attributes, "notes"),
		OccurredAt: ev.OccurredAt, CreatedAt: ev.CreatedAt,
	}
	items, _ := medicationEventItemMaps(ev.Attributes)
	resp.Items = make([]medicationItemResponse, len(items))
	for i, item := range items {
		resp.Items[i] = medicationItemResponse{
			Kind:       MedicationKind(attributeString(item, "kind")),
			Name:       attributeString(item, "name"),
			DoseUnit:   MedicationDoseUnit(attributeString(item, "dose_unit")),
			SeriesDose: MedicationSeriesDose(attributeString(item, "series_dose")),
		}
		if doseValue, ok := attributeFloat(item, "dose_value"); ok {
			resp.Items[i].DoseValue = &doseValue
		}
	}
	return resp
}

func medicationItemAttributes(w http.ResponseWriter, kind MedicationKind, name string, doseValue *float64, doseUnit MedicationDoseUnit, seriesDose MedicationSeriesDose) (map[string]any, bool) {
	if !kind.Valid() {
		writeError(w, http.StatusBadRequest, "kind must be one of: medicine, vaccine, other")
		return nil, false
	}

	name = strings.TrimSpace(name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return nil, false
	}

	attributes := map[string]any{
		"kind": string(kind),
		"name": name,
	}

	switch kind {
	case MedicationKindMedicine:
		if seriesDose != "" {
			writeError(w, http.StatusBadRequest, "series_dose is only supported for vaccines")
			return nil, false
		}
		if doseValue == nil {
			if doseUnit != "" {
				writeError(w, http.StatusBadRequest, "dose_unit requires dose_value")
				return nil, false
			}
		} else {
			if *doseValue <= 0 {
				writeError(w, http.StatusBadRequest, "dose_value must be a positive number")
				return nil, false
			}
			if !doseUnit.Valid() {
				writeError(w, http.StatusBadRequest, "dose_unit must be one of: ml, mg, drops, dose, other")
				return nil, false
			}
			attributes["dose_value"] = *doseValue
			attributes["dose_unit"] = string(doseUnit)
		}
	case MedicationKindVaccine:
		if doseValue != nil || doseUnit != "" {
			writeError(w, http.StatusBadRequest, "dose_value and dose_unit are only supported for medicines")
			return nil, false
		}
		if !seriesDose.Valid() {
			writeError(w, http.StatusBadRequest, "series_dose must be one of: first, second, third, booster")
			return nil, false
		}
		if seriesDose != "" {
			attributes["series_dose"] = string(seriesDose)
		}
	case MedicationKindOther:
		if doseValue != nil || doseUnit != "" || seriesDose != "" {
			writeError(w, http.StatusBadRequest, "dose and series fields are not supported for other items")
			return nil, false
		}
	}

	return attributes, true
}
