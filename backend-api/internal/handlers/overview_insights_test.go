package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andreistefanciprian/yauli/backend-api/internal/store"
)

func TestParseOverviewInsightsRangeDays(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   int
		wantOK bool
	}{
		{name: "empty defaults to 30", raw: "", want: 30, wantOK: true},
		{name: "7 is allowed", raw: "7", want: 7, wantOK: true},
		{name: "90 is allowed", raw: "90", want: 90, wantOK: true},
		{name: "14 is rejected", raw: "14", wantOK: false},
		{name: "non-numeric is rejected", raw: "abc", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOverviewInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetOverviewInsightsReportsAvailabilityAndGrowthChange(t *testing.T) {
	familyID := uuid.New()
	babyID := uuid.New()
	yesterday := time.Now().UTC().Truncate(24 * time.Hour).Add(-12 * time.Hour)
	fake := &overviewFakeStore{
		aiReportFakeStore: &aiReportFakeStore{
			baby: store.Baby{
				ID:            babyID,
				FamilyID:      familyID,
				Timezone:      "UTC",
				BirthDate:     "2026-01-01",
				BirthWeightKg: "3.40",
			},
			events: []store.Event{
				{
					ID:         uuid.New(),
					BabyID:     babyID,
					EventType:  eventTypeGrowthMeasurement,
					OccurredAt: yesterday,
					Attributes: map[string]any{"weight_grams": float64(5400)},
				},
				breastFeedEvent(babyID, yesterday, 30),
				formulaFeedEvent(babyID, yesterday.Add(time.Hour), 100),
				expressedFeedEvent(babyID, yesterday.Add(2*time.Hour), 50),
			},
		},
	}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, "")
	req.Method = http.MethodGet
	req.URL.Path = "/api/v1/babies/current/insights/overview"
	req.URL.RawQuery = "range=7"
	recorder := httptest.NewRecorder()

	h.GetOverviewInsights(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response overviewInsightsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RangeDays != 7 {
		t.Fatalf("RangeDays = %d, want 7", response.RangeDays)
	}
	if !response.Sleep.Available || !response.Feed.Available || !response.Nappy.Available || !response.Pump.Available || !response.Growth.Available || !response.Health.Available {
		t.Fatalf("availability = %#v, want every successful source available", response)
	}
	if !response.Growth.HasAnyData || response.Growth.LatestValueLabel != "5.400 kg" {
		t.Fatalf("Growth = %#v, want latest recorded weight", response.Growth)
	}
	if response.Growth.ChangeSinceBirthLabel != "+2.000 kg" {
		t.Fatalf("ChangeSinceBirthLabel = %q, want grams and kilograms normalized before subtraction", response.Growth.ChangeSinceBirthLabel)
	}
	if !response.Feed.HasAnyData || response.Feed.BreastTotalLabel != "30m" || response.Feed.BottleTotalLabel != "150 ml" {
		t.Fatalf("Feed = %#v, want 30m breast and 150 ml bottle", response.Feed)
	}
}

func TestGetOverviewInsightsMarksOnlyFailedSourceUnavailable(t *testing.T) {
	familyID := uuid.New()
	fake := &overviewFakeStore{
		aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{
			ID:        uuid.New(),
			FamilyID:  familyID,
			Timezone:  "UTC",
			BirthDate: "2026-01-01",
		}},
		failedEventType: eventTypeFeed,
	}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, "")
	req.Method = http.MethodGet
	req.URL.Path = "/api/v1/babies/current/insights/overview"
	recorder := httptest.NewRecorder()

	h.GetOverviewInsights(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response overviewInsightsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Feed.Available {
		t.Fatalf("Feed.Available = true, want false after its query failed")
	}
	if !response.Sleep.Available || !response.Nappy.Available || !response.Pump.Available || !response.Growth.Available || !response.Health.Available {
		t.Fatalf("availability = %#v, want only feed unavailable", response)
	}
}

func TestGetOverviewInsightsRequiresAuthentication(t *testing.T) {
	h := &Handlers{Store: &overviewFakeStore{aiReportFakeStore: &aiReportFakeStore{}}}
	recorder := httptest.NewRecorder()

	h.GetOverviewInsights(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/babies/current/insights/overview", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

type overviewFakeStore struct {
	*aiReportFakeStore
	failedEventType string
}

func (s *overviewFakeStore) ListEventsByType(ctx context.Context, familyID, babyID uuid.UUID, eventType string, from, to time.Time, limit int) ([]store.Event, error) {
	if eventType == s.failedEventType {
		return nil, errors.New("query failed")
	}
	return s.aiReportFakeStore.ListEventsByType(ctx, familyID, babyID, eventType, from, to, limit)
}
