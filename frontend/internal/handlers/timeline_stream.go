package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// TimelineEventStream proxies backend-api's private authenticated SSE stream
// to the browser's same-origin, cookie-authenticated connection.
func (h *Handlers) TimelineEventStream(w http.ResponseWriter, r *http.Request) {
	if h.TimelineStream == nil {
		http.Error(w, "timeline stream unavailable", http.StatusServiceUnavailable)
		return
	}

	upstream, err := h.TimelineStream.StreamTimelineEvents(r.Context())
	if err != nil {
		if r.Context().Err() == nil {
			log.Printf("open timeline event stream: %v", err)
			http.Error(w, "timeline stream unavailable", http.StatusBadGateway)
		}
		return
	}
	defer upstream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")

	if err := proxyTimelineEventStream(w, upstream); err != nil && r.Context().Err() == nil {
		log.Printf("proxy timeline event stream: %v", err)
	}
}

func proxyTimelineEventStream(w http.ResponseWriter, upstream io.Reader) error {
	controller := http.NewResponseController(w)
	buffer := make([]byte, 4096)

	for {
		n, readErr := upstream.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return fmt.Errorf("write timeline stream: %w", err)
			}
			if err := controller.Flush(); err != nil {
				return fmt.Errorf("flush timeline stream: %w", err)
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, context.Canceled) {
				return nil
			}
			return fmt.Errorf("read timeline stream: %w", readErr)
		}
	}
}
