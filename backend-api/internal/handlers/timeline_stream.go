package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/andreistefanciprian/yauli/backend-api/internal/authctx"
)

const timelineStreamHeartbeatInterval = 20 * time.Second

// StreamTimelineEvents keeps an authenticated SSE response open for the
// caller's current baby. Messages carry invalidation signals, never event
// contents; clients re-fetch the canonical timeline after a change.
func (h *Handlers) StreamTimelineEvents(w http.ResponseWriter, r *http.Request) {
	baby, ok := h.currentBabyForRequest(w, r)
	if !ok {
		return
	}
	claims, ok := authctx.FromContext(r.Context())
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if h.TimelineChanges == nil {
		http.Error(w, "timeline stream unavailable", http.StatusServiceUnavailable)
		return
	}

	changes, unsubscribe := h.TimelineChanges.Subscribe(baby.ID)
	defer unsubscribe()

	if err := serveTimelineEventStream(w, r.Context(), changes, claims.ExpiresAt); err != nil && r.Context().Err() == nil {
		log.Printf("stream timeline events: %v", err)
	}
}

func serveTimelineEventStream(w http.ResponseWriter, ctx context.Context, changes <-chan struct{}, expiresAt time.Time) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")

	controller := http.NewResponseController(w)
	if _, err := fmt.Fprint(w, "retry: 3000\nevent: ready\ndata: connected\n\n"); err != nil {
		return fmt.Errorf("write ready event: %w", err)
	}
	if err := controller.Flush(); err != nil {
		return fmt.Errorf("flush ready event: %w", err)
	}

	heartbeat := time.NewTicker(timelineStreamHeartbeatInterval)
	defer heartbeat.Stop()

	untilExpiry := time.Until(expiresAt)
	if untilExpiry <= 0 {
		return nil
	}
	expiry := time.NewTimer(untilExpiry)
	defer expiry.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-expiry.C:
			return nil
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return fmt.Errorf("write heartbeat: %w", err)
			}
		case <-changes:
			if _, err := fmt.Fprint(w, "event: timeline_changed\ndata: refresh\n\n"); err != nil {
				return fmt.Errorf("write timeline change: %w", err)
			}
		}

		if err := controller.Flush(); err != nil {
			return fmt.Errorf("flush timeline stream: %w", err)
		}
	}
}
