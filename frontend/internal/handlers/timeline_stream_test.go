package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTimelineEventStreamProxiesSSEBodyAndHeaders(t *testing.T) {
	const stream = "event: ready\ndata: connected\n\nevent: timeline_changed\ndata: refresh\n\n"
	h := &Handlers{
		TimelineStream: timelineStreamBackendFunc(func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(stream)), nil
		}),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/timeline/events/stream", nil)
	h.TimelineEventStream(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if rec.Body.String() != stream {
		t.Fatalf("body = %q, want %q", rec.Body.String(), stream)
	}
	if !rec.Flushed {
		t.Fatal("stream response was not flushed")
	}
}

func TestTimelineEventStreamReportsUnavailableUpstream(t *testing.T) {
	h := &Handlers{
		TimelineStream: timelineStreamBackendFunc(func(context.Context) (io.ReadCloser, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/timeline/events/stream", nil)
	h.TimelineEventStream(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

type timelineStreamBackendFunc func(context.Context) (io.ReadCloser, error)

func (f timelineStreamBackendFunc) StreamTimelineEvents(ctx context.Context) (io.ReadCloser, error) {
	return f(ctx)
}
