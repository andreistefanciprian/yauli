package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

const eventTypeFeed = "feed"

// FeedType is the set of valid feed types.
type FeedType string

const (
	FeedTypeBreast    FeedType = "breast"
	FeedTypeFormula   FeedType = "formula"
	FeedTypeExpressed FeedType = "expressed"
)

func (t FeedType) Valid() bool {
	switch t {
	case FeedTypeBreast, FeedTypeFormula, FeedTypeExpressed:
		return true
	default:
		return false
	}
}

type createFeedRequest struct {
	Type            string   `json:"type"`
	AmountMl        *int     `json:"amount_ml"`
	DurationMinutes *int     `json:"duration_minutes"`
	Labels          []string `json:"labels"`
	Notes           string   `json:"notes"`
	OccurredAt      string   `json:"occurred_at"`
}

// feedResponse is a feed event as returned to API consumers.
type feedResponse struct {
	ID              uuid.UUID `json:"id"`
	BabyID          uuid.UUID `json:"baby_id"`
	Type            FeedType  `json:"type"`
	AmountMl        *int      `json:"amount_ml,omitempty"`
	DurationMinutes *int      `json:"duration_minutes,omitempty"`
	Labels          []string  `json:"labels,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *Handlers) CreateFeed(w http.ResponseWriter, r *http.Request) {
	var req createFeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	feedType := FeedType(req.Type)
	if !feedType.Valid() {
		writeError(w, http.StatusBadRequest, "type must be one of: breast, formula, expressed")
		return
	}

	occurredAt, ok := parseOccurredAt(w, req.OccurredAt)
	if !ok {
		return
	}

	attributes := map[string]any{"type": string(feedType)}
	if req.AmountMl != nil {
		attributes["amount_ml"] = *req.AmountMl
	}
	if req.DurationMinutes != nil {
		attributes["duration_minutes"] = *req.DurationMinutes
	}
	labels, ok := normalizeFeedLabels(req.Labels)
	if !ok {
		writeError(w, http.StatusBadRequest, "labels include an unsupported feed label")
		return
	}
	if len(labels) > 0 {
		attributes["labels"] = labels
	}
	if req.Notes != "" {
		attributes["notes"] = req.Notes
	}

	createAndRespond(w, r, h, eventTypeFeed, attributes, occurredAt, feedFromEvent)
}

func feedFromEvent(ev store.Event) feedResponse {
	resp := feedResponse{ID: ev.ID, BabyID: ev.BabyID, OccurredAt: ev.OccurredAt, CreatedAt: ev.CreatedAt}
	if t, ok := ev.Attributes["type"].(string); ok {
		resp.Type = FeedType(t)
	}
	if v, ok := attributeInt(ev.Attributes, "amount_ml"); ok {
		resp.AmountMl = &v
	}
	if v, ok := attributeInt(ev.Attributes, "duration_minutes"); ok {
		resp.DurationMinutes = &v
	}
	if labels, ok := feedLabelsFromAttribute(ev.Attributes["labels"]); ok {
		resp.Labels = labels
	}
	if notes, ok := ev.Attributes["notes"].(string); ok {
		resp.Notes = notes
	}
	return resp
}

func normalizeFeedLabels(raw []string) ([]string, bool) {
	seen := map[string]bool{}
	labels := make([]string, 0, len(raw))
	for _, label := range raw {
		if !validFeedLabel(label) {
			return nil, false
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		labels = append(labels, label)
	}
	return labels, true
}

func feedLabelsFromAttribute(raw any) ([]string, bool) {
	switch labels := raw.(type) {
	case nil:
		return nil, true
	case []string:
		return normalizeFeedLabels(labels)
	case []any:
		values := make([]string, 0, len(labels))
		for _, label := range labels {
			value, ok := label.(string)
			if !ok {
				return nil, false
			}
			values = append(values, value)
		}
		return normalizeFeedLabels(values)
	default:
		return nil, false
	}
}

func validFeedLabel(label string) bool {
	switch label {
	case "burped_halfway", "burped_after", "spit_up", "fussy", "sleepy", "settled_after":
		return true
	default:
		return false
	}
}

// attributeInt reads an int out of an events.attributes map. The value is a
// native Go int right after CreateEvent builds the map in-process, but a
// float64 once it round-trips through Postgres JSONB and back (pgx decodes
// JSON numbers as float64), so both forms have to be handled.
func attributeInt(attributes map[string]any, key string) (int, bool) {
	switch v := attributes[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}
