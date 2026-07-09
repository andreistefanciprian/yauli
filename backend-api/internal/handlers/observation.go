package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"yauyau/backend-api/internal/store"
)

const eventTypeObservation = "observation"

// defaultObservationCategory is used when category is omitted. Categories
// are suggestions only (general, behaviour, feeding, sleep, health, doctor,
// milestone) and are deliberately not validated against a fixed set — any
// non-empty string is accepted, unlike NappyKind/FeedType/BathType.
const defaultObservationCategory = "general"

type createObservationRequest struct {
	Text       string `json:"text"`
	Category   string `json:"category"`
	OccurredAt string `json:"occurred_at"`
}

// observationResponse is an observation event as returned to API consumers.
type observationResponse struct {
	ID         uuid.UUID `json:"id"`
	BabyID     uuid.UUID `json:"baby_id"`
	Text       string    `json:"text"`
	Category   string    `json:"category"`
	OccurredAt time.Time `json:"occurred_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *Handlers) CreateObservation(w http.ResponseWriter, r *http.Request) {
	var req createObservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	category := req.Category
	if category == "" {
		category = defaultObservationCategory
	}

	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "occurred_at must be RFC3339 formatted")
		return
	}

	attributes := map[string]any{"text": req.Text, "category": category}

	ev, err := h.Store.CreateEvent(r.Context(), eventTypeObservation, attributes, occurredAt)
	if err != nil {
		log.Printf("create observation event: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save observation event")
		return
	}

	writeJSON(w, http.StatusCreated, observationFromEvent(ev))
}

func (h *Handlers) ListObservations(w http.ResponseWriter, r *http.Request) {
	events, err := h.Store.ListEvents(r.Context(), eventTypeObservation, 20)
	if err != nil {
		log.Printf("list observation events: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load observation events")
		return
	}

	observations := make([]observationResponse, len(events))
	for i, ev := range events {
		observations[i] = observationFromEvent(ev)
	}

	writeJSON(w, http.StatusOK, observations)
}

func observationFromEvent(ev store.Event) observationResponse {
	resp := observationResponse{ID: ev.ID, BabyID: ev.BabyID, OccurredAt: ev.OccurredAt, CreatedAt: ev.CreatedAt}
	if text, ok := ev.Attributes["text"].(string); ok {
		resp.Text = text
	}
	if category, ok := ev.Attributes["category"].(string); ok {
		resp.Category = category
	}
	return resp
}
