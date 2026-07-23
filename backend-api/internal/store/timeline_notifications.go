package store

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	timelineNotificationsChannel = "timeline_events_changed"
	timelineListenerRetryDelay   = time.Second
)

// TimelineNotifications turns PostgreSQL LISTEN/NOTIFY messages into
// baby-scoped in-process subscriptions. Notifications are invalidation
// signals only: consumers must re-read canonical event data after receiving
// one.
type TimelineNotifications struct {
	pool *pgxpool.Pool

	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan struct{}]struct{}
}

func NewTimelineNotifications(pool *pgxpool.Pool) *TimelineNotifications {
	return &TimelineNotifications{
		pool:        pool,
		subscribers: make(map[uuid.UUID]map[chan struct{}]struct{}),
	}
}

// Subscribe registers one coalescing notification channel for babyID.
// unsubscribe must be called when the request ends.
func (n *TimelineNotifications) Subscribe(babyID uuid.UUID) (<-chan struct{}, func()) {
	changes := make(chan struct{}, 1)

	n.mu.Lock()
	if n.subscribers[babyID] == nil {
		n.subscribers[babyID] = make(map[chan struct{}]struct{})
	}
	n.subscribers[babyID][changes] = struct{}{}
	n.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			n.mu.Lock()
			delete(n.subscribers[babyID], changes)
			if len(n.subscribers[babyID]) == 0 {
				delete(n.subscribers, babyID)
			}
			n.mu.Unlock()
		})
	}

	return changes, unsubscribe
}

// Run holds a dedicated PostgreSQL LISTEN connection. LISTEN/NOTIFY is
// transient rather than durable, so after reconnecting it asks every
// subscriber to resync in case a notification was missed during the gap.
func (n *TimelineNotifications) Run(ctx context.Context) {
	for {
		err := n.listen(ctx)
		if ctx.Err() != nil {
			return
		}

		slog.Error("timeline notification listener disconnected", "error", err)
		if !waitForTimelineListenerRetry(ctx) {
			return
		}
	}
}

func (n *TimelineNotifications) listen(ctx context.Context) error {
	conn, err := n.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+timelineNotificationsChannel); err != nil {
		return err
	}
	slog.Info("timeline notification listener connected")
	n.publishAll()

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}

		babyID, err := uuid.Parse(notification.Payload)
		if err != nil {
			slog.Warn("ignoring malformed timeline notification", "payload", notification.Payload)
			continue
		}
		n.publish(babyID)
	}
}

func (n *TimelineNotifications) publish(babyID uuid.UUID) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for changes := range n.subscribers[babyID] {
		select {
		case changes <- struct{}{}:
		default:
			// A pending signal already tells this subscriber to re-read the
			// timeline, so another one would carry no additional information.
		}
	}
}

func (n *TimelineNotifications) publishAll() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, subscribers := range n.subscribers {
		for changes := range subscribers {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	}
}

func waitForTimelineListenerRetry(ctx context.Context) bool {
	timer := time.NewTimer(timelineListenerRetryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
