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

type timelineListAllEventsCall struct {
	Limit int
}

type timelineFakeStore struct {
	*aiReportFakeStore
	responses [][]store.Event
	calls     []timelineListAllEventsCall
}

func (s *timelineFakeStore) ListAllEvents(_ context.Context, _, _ uuid.UUID, _, _ time.Time, limit int) ([]store.Event, error) {
	s.calls = append(s.calls, timelineListAllEventsCall{Limit: limit})
	response := s.responses[len(s.calls)-1]
	if len(response) > limit {
		response = response[:limit]
	}
	return response, nil
}

func TestOrderTimelineEventsFloatsOngoingFeedsPumpsAndSleeps(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	events := []store.Event{
		{EventType: eventTypeNappy, OccurredAt: now},
		{EventType: eventTypeSleep, Attributes: map[string]any{"duration_minutes": float64(60)}, OccurredAt: now.Add(-time.Hour)},
		{EventType: eventTypeFeed, Attributes: map[string]any{"type": string(FeedTypeBreast)}, OccurredAt: now.Add(-2 * time.Hour)},
		{EventType: eventTypePump, Attributes: map[string]any{"amount_ml": float64(80)}, OccurredAt: now.Add(-3 * time.Hour)},
		{EventType: eventTypeSleep, Attributes: map[string]any{}, OccurredAt: now.Add(-4 * time.Hour)},
		{EventType: eventTypePump, Attributes: map[string]any{"amount_ml": float64(70), "duration_minutes": float64(20)}, OccurredAt: now.Add(-5 * time.Hour)},
		{EventType: eventTypeFeed, Attributes: map[string]any{"duration_minutes": float64(10)}, OccurredAt: now.Add(-6 * time.Hour)},
	}

	orderTimelineEvents(events)

	if events[0].EventType != eventTypeFeed || !isOngoingFeed(events[0]) {
		t.Fatalf("first event = %#v, want ongoing feed", events[0])
	}
	if events[1].EventType != eventTypePump || !isOngoingPump(events[1]) {
		t.Fatalf("second event = %#v, want ongoing pump", events[1])
	}
	if events[2].EventType != eventTypeSleep || !isOngoingSleep(events[2]) {
		t.Fatalf("third event = %#v, want ongoing sleep", events[2])
	}
	if events[3].EventType != eventTypeNappy {
		t.Fatalf("fourth event = %s, want nappy to preserve non-ongoing order", events[3].EventType)
	}
	if events[4].EventType != eventTypeSleep || isOngoingSleep(events[4]) {
		t.Fatalf("fifth event = %#v, want completed sleep to stay with regular events", events[4])
	}
	if events[5].EventType != eventTypePump || isOngoingPump(events[5]) {
		t.Fatalf("sixth event = %#v, want completed pump to stay with regular events", events[5])
	}
}

func TestAppendOngoingTimelineCarryoverIncludesOnlySupportedOngoingEvents(t *testing.T) {
	now := time.Date(2026, 8, 11, 0, 5, 0, 0, time.UTC)
	current := []store.Event{{EventType: eventTypeNappy, OccurredAt: now}}
	previous := []store.Event{
		{EventType: eventTypeFeed, Attributes: map[string]any{"type": string(FeedTypeBreast)}, OccurredAt: now.Add(-10 * time.Minute)},
		{EventType: eventTypePump, Attributes: map[string]any{"amount_ml": float64(80)}, OccurredAt: now.Add(-20 * time.Minute)},
		{EventType: eventTypeSleep, Attributes: map[string]any{}, OccurredAt: now.Add(-time.Hour)},
		{EventType: eventTypeFeed, Attributes: map[string]any{"duration_minutes": float64(15)}, OccurredAt: now.Add(-2 * time.Hour)},
		{EventType: eventTypeNappy, Attributes: map[string]any{"kind": "wet"}, OccurredAt: now.Add(-3 * time.Hour)},
	}

	events := appendOngoingTimelineCarryover(current, previous)
	if len(events) != 4 {
		t.Fatalf("events = %#v, want current event plus three ongoing carryovers", events)
	}
	for i, eventType := range []string{eventTypeFeed, eventTypePump, eventTypeSleep} {
		if events[i+1].EventType != eventType || !isOngoingTimelineEvent(events[i+1]) {
			t.Fatalf("carryover %d = %#v, want ongoing %s", i, events[i+1], eventType)
		}
	}
}

func TestListAllEventsScansBeyondResponseLimitForOngoingCarryover(t *testing.T) {
	familyID := uuid.New()
	babyID := uuid.New()
	current := make([]store.Event, allEventsLimit)
	previous := make([]store.Event, allEventsLimit+1)
	for i := range current {
		current[i] = store.Event{ID: uuid.New(), BabyID: babyID, EventType: eventTypeNappy}
		previous[i] = store.Event{ID: uuid.New(), BabyID: babyID, EventType: eventTypeNappy}
	}
	ongoingID := uuid.New()
	previous[allEventsLimit] = store.Event{
		ID:         ongoingID,
		BabyID:     babyID,
		EventType:  eventTypeSleep,
		Attributes: map[string]any{},
	}

	fake := &timelineFakeStore{
		aiReportFakeStore: &aiReportFakeStore{baby: store.Baby{
			ID:       babyID,
			FamilyID: familyID,
			Timezone: "UTC",
		}},
		responses: [][]store.Event{current, previous},
	}
	h := &Handlers{Store: fake}
	req := authenticatedAIReportRequest(t, familyID, "")
	req.Method = http.MethodGet
	req.URL.Path = "/api/v1/babies/current/events"
	recorder := httptest.NewRecorder()

	h.ListAllEvents(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(fake.calls) != 2 {
		t.Fatalf("ListAllEvents calls = %d, want current and previous day", len(fake.calls))
	}
	if fake.calls[0].Limit != allEventsLimit || fake.calls[1].Limit != timelineCarryoverScanLimit {
		t.Fatalf("ListAllEvents limits = %#v, want %d then %d", fake.calls, allEventsLimit, timelineCarryoverScanLimit)
	}

	var response []eventResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != allEventsLimit {
		t.Fatalf("response events = %d, want cap %d", len(response), allEventsLimit)
	}
	if response[0].ID != ongoingID {
		t.Fatalf("first event ID = %s, want older ongoing carryover %s", response[0].ID, ongoingID)
	}
}

func TestTimelineDayWindowForExplicitDateUsesBabyTimezone(t *testing.T) {
	window, err := timelineDayWindowFor("2026-07-11", "Australia/Adelaide")
	if err != nil {
		t.Fatalf("timelineDayWindowFor returned error: %v", err)
	}

	loc, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	wantFrom := time.Date(2026, 7, 11, 0, 0, 0, 0, loc)
	wantTo := wantFrom.AddDate(0, 0, 1)
	if !window.From.Equal(wantFrom) || !window.To.Equal(wantTo) {
		t.Fatalf("window = %s to %s, want %s to %s", window.From, window.To, wantFrom, wantTo)
	}
	if window.Today {
		t.Fatal("historical timeline window marked as Today")
	}
}

func TestTimelineDayWindowForTodayMarksTodayInBabyTimezone(t *testing.T) {
	window, err := timelineDayWindowFor("", "Australia/Adelaide")
	if err != nil {
		t.Fatalf("timelineDayWindowFor returned error: %v", err)
	}
	if !window.Today {
		t.Fatal("default timeline window not marked as Today")
	}
}

func TestTimelineDayWindowForRejectsInvalidDate(t *testing.T) {
	_, err := timelineDayWindowFor("day-2", "Australia/Adelaide")
	if !errors.Is(err, errInvalidTimelineDate) {
		t.Fatalf("error = %v, want errInvalidTimelineDate", err)
	}
}

func TestTimelineDayWindowForRejectsFutureDate(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Adelaide")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1).Format(time.DateOnly)

	_, err = timelineDayWindowFor(tomorrow, "Australia/Adelaide")
	if !errors.Is(err, errInvalidTimelineDate) {
		t.Fatalf("error = %v, want errInvalidTimelineDate", err)
	}
}
