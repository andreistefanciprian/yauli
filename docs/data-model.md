# Data Model

## Database

Yauli uses PostgreSQL. Domain and authentication ownership remains split even
when services share one database deployment.

Current backend-api tables:

* `users`
* `families`
* `family_members`
* `babies`
* `events`
* `baby_latest_growth`
* `ai_report_cache`
* `ai_report_email_deliveries`

Current auth-service tables:

* `magic_links`
* `sessions`
* `audit_logs`

Planned OAuth 2.1 + PKCE work will add OAuth client, authorization-code,
access-token, and refresh-token storage. Those tables do not exist yet.

---

## Event Model

All timeline records share one `events` table with an `event_type`,
`occurred_at`, and JSONB `attributes`.

Implemented event types:

* Feed
* Nappy
* Sleep
* Pump
* Bath
* Observation
* Temperature
* Growth measurement

Medication and vaccination are planned event types, not current handlers or
routes.

Events can be created, corrected, completed, and deleted. Business validation
lives in `backend-api`; the generic store owns persistence, while per-event
handlers normalise each type's attributes.

For current routes, storage methods, and the pattern for adding an event type,
see [API Endpoint Structure](reference/api-routes.md).

An `AFTER INSERT OR UPDATE OR DELETE` trigger on `events` publishes the
affected `baby_id` to PostgreSQL's `timeline_events_changed` notification
channel after commit. The notification is a transient invalidation hint for
the authenticated timeline SSE stream, not another event store; consumers
always re-read canonical event data.
