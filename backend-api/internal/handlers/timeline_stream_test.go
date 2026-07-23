package handlers

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestServeTimelineEventStreamSendsReadyAndChangeEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := newSSETestWriter()
	changes := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- serveTimelineEventStream(writer, ctx, changes, time.Now().Add(time.Minute))
	}()

	writer.waitForFlush(t)
	changes <- struct{}{}
	writer.waitForFlush(t)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve stream: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after cancellation")
	}

	if got := writer.header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	body := writer.bodyString()
	for _, want := range []string{
		"retry: 3000\n",
		"event: ready\ndata: connected\n\n",
		"event: timeline_changed\ndata: refresh\n\n",
	} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("stream body does not contain %q: %q", want, body)
		}
	}
}

func TestServeTimelineEventStreamStopsAtTokenExpiry(t *testing.T) {
	writer := newSSETestWriter()
	done := make(chan error, 1)
	go func() {
		done <- serveTimelineEventStream(
			writer,
			context.Background(),
			make(chan struct{}),
			time.Now().Add(20*time.Millisecond),
		)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve stream: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stream remained open after token expiry")
	}
}

type sseTestWriter struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	status  int
	flushed chan struct{}
}

func newSSETestWriter() *sseTestWriter {
	return &sseTestWriter{
		header:  make(http.Header),
		flushed: make(chan struct{}, 10),
	}
}

func (w *sseTestWriter) Header() http.Header {
	return w.header
}

func (w *sseTestWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

func (w *sseTestWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *sseTestWriter) Flush() {
	w.flushed <- struct{}{}
}

func (w *sseTestWriter) waitForFlush(t *testing.T) {
	t.Helper()
	select {
	case <-w.flushed:
	case <-time.After(time.Second):
		t.Fatal("stream did not flush")
	}
}

func (w *sseTestWriter) bodyString() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}
