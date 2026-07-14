package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeFeedLabelsDedupesSupportedLabels(t *testing.T) {
	labels, ok := normalizeFeedLabels([]string{"burped_after", "fussy", "burped_after"})
	if !ok {
		t.Fatal("normalizeFeedLabels rejected supported labels")
	}
	want := []string{"burped_after", "fussy"}
	if len(labels) != len(want) {
		t.Fatalf("len(labels) = %d, want %d: %#v", len(labels), len(want), labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestNormalizeFeedLabelsRejectsUnsupportedLabel(t *testing.T) {
	if _, ok := normalizeFeedLabels([]string{"angry"}); ok {
		t.Fatal("normalizeFeedLabels accepted unsupported label")
	}
}

func TestNormalizeFeedAttributesRejectsBreastAmount(t *testing.T) {
	if _, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeFeed, map[string]any{
		"type":      "breast",
		"amount_ml": float64(80),
	}); ok {
		t.Fatal("normalizeEventAttributes accepted amount_ml for breast feed")
	}
}

func TestNormalizeFeedAttributesRequiresBottleAmount(t *testing.T) {
	for _, tt := range []struct {
		name       string
		feedType   string
		attributes map[string]any
	}{
		{name: "formula missing amount", feedType: "formula", attributes: map[string]any{"type": "formula"}},
		{name: "expressed missing amount", feedType: "expressed", attributes: map[string]any{"type": "expressed"}},
		{name: "formula zero amount", feedType: "formula", attributes: map[string]any{"type": "formula", "amount_ml": float64(0)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeFeed, tt.attributes); ok {
				t.Fatalf("normalizeEventAttributes accepted %s feed without positive amount_ml", tt.feedType)
			}
		})
	}
}

func TestNormalizeFeedAttributesAllowsBottleAmount(t *testing.T) {
	for _, feedType := range []string{"formula", "expressed"} {
		t.Run(feedType, func(t *testing.T) {
			attrs, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeFeed, map[string]any{
				"type":      feedType,
				"amount_ml": float64(80),
			})
			if !ok {
				t.Fatalf("normalizeEventAttributes rejected %s feed with amount_ml", feedType)
			}
			if attrs["amount_ml"] != 80 {
				t.Fatalf("amount_ml = %#v, want 80", attrs["amount_ml"])
			}
		})
	}
}

func TestNormalizeFeedAttributesAllowsBreastWithoutAmount(t *testing.T) {
	attrs, ok := normalizeEventAttributes(httptest.NewRecorder(), eventTypeFeed, map[string]any{
		"type":             "breast",
		"duration_minutes": float64(15),
	})
	if !ok {
		t.Fatal("normalizeEventAttributes rejected breast feed without amount_ml")
	}
	if _, ok := attrs["amount_ml"]; ok {
		t.Fatalf("attrs should not include amount_ml: %#v", attrs)
	}
}

func TestCreateFeedRejectsBreastAmount(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/feeds", strings.NewReader(`{"type":"breast","amount_ml":80,"occurred_at":"2026-07-13T08:20:00Z"}`))
	rec := httptest.NewRecorder()

	(&Handlers{}).CreateFeed(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateFeedRequiresBottleAmount(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/feeds", strings.NewReader(`{"type":"formula","occurred_at":"2026-07-13T08:20:00Z"}`))
	rec := httptest.NewRecorder()

	(&Handlers{}).CreateFeed(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateFeedRejectsZeroBottleAmount(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/feeds", strings.NewReader(`{"type":"expressed","amount_ml":0,"occurred_at":"2026-07-13T08:20:00Z"}`))
	rec := httptest.NewRecorder()

	(&Handlers{}).CreateFeed(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
