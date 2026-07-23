CREATE OR REPLACE FUNCTION notify_timeline_event_changed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM pg_notify('timeline_events_changed', OLD.baby_id::text);
    RETURN OLD;
  END IF;

  PERFORM pg_notify('timeline_events_changed', NEW.baby_id::text);
  RETURN NEW;
END;
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgname = 'events_notify_timeline_changed'
      AND tgrelid = 'events'::regclass
  ) THEN
    CREATE TRIGGER events_notify_timeline_changed
    AFTER INSERT OR UPDATE OR DELETE ON events
    FOR EACH ROW
    EXECUTE FUNCTION notify_timeline_event_changed();
  END IF;
EXCEPTION
  WHEN duplicate_object THEN
    NULL;
END;
$$;
