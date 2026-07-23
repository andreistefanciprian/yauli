package backendclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamTimelineEventsUsesBearerTokenAndStreamingClient(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: ready\ndata: connected\n\n"))
	}))
	t.Cleanup(server.Close)

	client := New(server.URL)
	client.http.Timeout = time.Millisecond
	ctx := ContextWithToken(context.Background(), "stream-token")

	body, err := client.StreamTimelineEvents(ctx)
	if err != nil {
		t.Fatalf("stream timeline events: %v", err)
	}
	defer body.Close()
	if _, err := io.ReadAll(body); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if authorization != "Bearer stream-token" {
		t.Fatalf("Authorization = %q, want bearer token", authorization)
	}
}
