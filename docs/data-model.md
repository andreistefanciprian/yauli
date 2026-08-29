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
* Medication, including vaccine records
* Growth measurement

Each Medication event stores one or more items in an `items` attribute. Every
item uses `kind` (`medicine`, `vaccine`, or `other`) and a required free-text
`name`.
Medicine items may include a positive `dose_value` with `dose_unit` (`ml`,
`mg`, `drops`, `dose`, or `other`); vaccine items may instead include
`series_dose` (`first`, `second`, `third`, or `booster`); other items are
name-only. The event owns the
shared occurrence time and optional `notes`. It is displayed, edited, and
deleted as one timeline event. The model records what was given and does not
provide dosage advice.
See [ADR 0008](decisions/0008-medication-event-contract.md) for the contract
trade-offs.

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
