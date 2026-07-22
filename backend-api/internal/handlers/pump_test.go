package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestNormalizePumpAttributesWithoutDurationIsOngoing(t *testing.T) {
	attributes, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypePump, map[string]any{
		"amount_ml": float64(80),
	})
	if !ok {
		t.Fatal("normalizeEventAttributes rejected an ongoing pump")
	}
	if !isOngoingPump(store.Event{EventType: eventTypePump, Attributes: attributes}) {
		t.Fatalf("duration-less pump not marked ongoing: %#v", attributes)
	}
}

func TestNormalizePumpAttributesWithDurationIsCompleted(t *testing.T) {
	attributes, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypePump, map[string]any{
		"amount_ml":        float64(80),
		"duration_minutes": float64(15),
	})
	if !ok {
		t.Fatal("normalizeEventAttributes rejected a completed pump")
	}
	if isOngoingPump(store.Event{EventType: eventTypePump, Attributes: attributes}) {
		t.Fatalf("pump with duration marked ongoing: %#v", attributes)
	}
}
