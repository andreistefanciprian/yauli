package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTimelineNotificationsPublishesOnlyToMatchingBaby(t *testing.T) {
	notifications := NewTimelineNotifications(nil)
	firstBabyID := uuid.New()
	secondBabyID := uuid.New()

	first, unsubscribeFirst := notifications.Subscribe(firstBabyID)
	t.Cleanup(unsubscribeFirst)
	second, unsubscribeSecond := notifications.Subscribe(secondBabyID)
	t.Cleanup(unsubscribeSecond)

	notifications.publish(firstBabyID)

	select {
	case <-first:
	default:
		t.Fatal("matching subscriber did not receive notification")
	}
	select {
	case <-second:
		t.Fatal("different baby's subscriber received notification")
	default:
	}
}

func TestTimelineNotificationsCoalescesPendingSignals(t *testing.T) {
	notifications := NewTimelineNotifications(nil)
	babyID := uuid.New()
	changes, unsubscribe := notifications.Subscribe(babyID)
	t.Cleanup(unsubscribe)

	notifications.publish(babyID)
	notifications.publish(babyID)

	select {
	case <-changes:
	default:
		t.Fatal("subscriber did not receive notification")
	}
	select {
	case <-changes:
		t.Fatal("duplicate signal was not coalesced")
	default:
	}
}

func TestTimelineNotificationsReceivesCommittedDatabaseChange(t *testing.T) {
	s := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	notifications := s.TimelineNotifications()
	babyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	familyID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	changes, unsubscribe := notifications.Subscribe(babyID)

	done := make(chan struct{})
	go func() {
		notifications.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		unsubscribe()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("timeline notification listener did not stop")
		}
	})

	// A successful LISTEN publishes a resync to all subscribers. Receiving it
	// here also removes the race between starting the goroutine and creating
	// the event below.
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("timeline notification listener did not become ready")
	}

	event, err := s.CreateEvent(ctx, familyID, babyID, "observation", map[string]any{
		"text":     "listener integration test",
		"category": "general",
	}, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	t.Cleanup(func() {
		execCleanup(t, s, `DELETE FROM events WHERE id = $1`, event.ID)
	})

	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("listener did not publish committed event change")
	}
}

func TestEventMutationsNotifyAfterCommit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	familyID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	babyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	listener, err := s.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire listener connection: %v", err)
	}
	t.Cleanup(listener.Release)
	if _, err := listener.Exec(ctx, "LISTEN "+timelineNotificationsChannel); err != nil {
		t.Fatalf("listen for timeline notifications: %v", err)
	}

	event, err := s.CreateEvent(ctx, familyID, babyID, "observation", map[string]any{
		"text":     "notification test",
		"category": "general",
	}, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	t.Cleanup(func() {
		execCleanup(t, s, `DELETE FROM events WHERE id = $1`, event.ID)
	})
	assertTimelineNotification(t, listener.Conn().WaitForNotification, babyID)

	if _, err := s.UpdateEvent(ctx, familyID, babyID, event.ID, "observation", map[string]any{
		"text":     "updated notification test",
		"category": "general",
	}, event.OccurredAt); err != nil {
		t.Fatalf("update event: %v", err)
	}
	assertTimelineNotification(t, listener.Conn().WaitForNotification, babyID)

	if err := s.DeleteEvent(ctx, familyID, babyID, event.ID); err != nil {
		t.Fatalf("delete event: %v", err)
	}
	assertTimelineNotification(t, listener.Conn().WaitForNotification, babyID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rolled-back event transaction: %v", err)
	}
	rolledBackEventID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO events (id, family_id, baby_id, event_type, occurred_at, attributes, source)
		VALUES ($1, $2, $3, 'observation', $4, '{"text":"rolled back"}', 'web')
	`, rolledBackEventID, familyID, babyID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("insert rolled-back event: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback event transaction: %v", err)
	}

	noNotificationCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if notification, err := listener.Conn().WaitForNotification(noNotificationCtx); err == nil {
		t.Fatalf("rolled-back event emitted notification: %#v", notification)
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait after rollback: %v", err)
	}
}

func assertTimelineNotification(
	t *testing.T,
	wait func(context.Context) (*pgconn.Notification, error),
	wantBabyID uuid.UUID,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	notification, err := wait(ctx)
	if err != nil {
		t.Fatalf("wait for timeline notification: %v", err)
	}
	if notification.Payload != wantBabyID.String() {
		t.Fatalf("notification payload = %q, want %q", notification.Payload, wantBabyID)
	}
}
