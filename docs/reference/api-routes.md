# API Endpoint Structure (current implementation)

This section documents how the code is actually laid out today, so the
pattern is easy to follow when adding the next event type. See
[docs/data-model.md](../data-model.md) for the Event Model, the long-term
vision (feed/nappy/sleep/pump/etc. as a generalized model) that this
section's backend-api + frontend wiring implements.

## Public discovery routes

The frontend serves three static discovery files at the site root:

* `GET /robots.txt` allows the public landing page, excludes private
  family-timeline and token-bearing workflows from crawling, and points
  crawlers to the sitemap.
* `GET /sitemap.xml` lists only the canonical public landing page at
  `https://getyauli.com/`.
* `GET /llms.txt` provides a concise, factual product overview and links to
  the public landing page and source repository. It explicitly distinguishes
  current web-app functionality from the planned ChatGPT/MCP experience.

Authenticated and authentication-related HTML pages also emit a `noindex`
robots meta tag. The public sign-in page remains crawlable so crawlers can see
and honour that tag. `robots.txt` is crawler guidance, not an access-control
boundary; sessions and authorization continue to protect all private data.

## backend-api routes

Authenticated app routes are mounted under `/api/v1/users` and
`/api/v1/babies` in `backend-api/cmd/server/main.go`, behind
`authctx.Middleware` (verifies the `Authorization: Bearer` JWT's
signature/expiry and decodes the caller's identity into context — see
`internal/authctx`):

* `GET /healthz` — unauthenticated.
* `GET /api/v1/users/me` → `GetCurrentUser`; returns the authenticated
  user's id, email, and display name for account/settings UI.
* `PATCH /api/v1/users/me` → `UpdateCurrentUser`; updates optional account
  profile fields such as `display_name`.
* `POST /api/v1/babies` → `CreateBaby`. A caller with no existing family
  membership gets a family created implicitly (auto-named, never shown to
  the user) and becomes its owner in the same call; a caller who already
  belongs to a family just gets a sibling baby added to it.
* `GET /api/v1/babies/current` → `GetCurrentBaby`, family-scoped (the
  caller's family's first-created baby, or 404 meaning "no baby yet").
  Includes `has_pending_invite` so the frontend can warn one-active-timeline
  users that they must archive the current baby before joining another
  timeline.
* `PATCH /api/v1/babies/current` → `UpdateCurrentBaby`, owner-only; updates
  current baby profile fields such as name, timezone, birth date, birth
  weight, birth length, and sex.
* `DELETE /api/v1/babies/current` → `ArchiveCurrentBaby`, owner-only; requires
  the caller to confirm the exact baby name and soft-deletes the active baby
  by setting `babies.archived_at`. Archiving also removes the owner's
  membership for that family and revokes their sessions, keeping the current
  product model to one active timeline per user.
* `POST /api/v1/babies/{id}/invite` → `InviteHelper`, baby-scoped and
  owner-only; creates a pending helper invite for the supplied email.
* `GET /api/v1/babies/current/members` → `ListTimelineMembers`, owner-only;
  returns active and invited users with access to the current baby's
  timeline.
* `PATCH /api/v1/babies/current/members/{userID}` →
  `UpdateTimelineMember`, owner-only; updates the member's relationship label
  (profile context such as "Dad", not an authorization role).
* `PATCH /api/v1/babies/current/members/{userID}/report-preferences` →
  `UpdateTimelineMemberReportPreferences`, owner-only; enables or disables
  scheduled report email delivery for an active member of the current baby's
  family timeline.
* `DELETE /api/v1/babies/current/members/{userID}` →
  `RemoveTimelineMember`, owner-only; cancels pending invites or removes
  active non-owner access. Active removal first asks auth-service to revoke
  the member's still-valid sessions for the family, then deletes the
  `family_members` row.
* `GET /api/v1/babies/current/events` → `ListAllEvents`, the combined feed
  behind the frontend timeline: every event type, merged and ordered
  newest-first (`store.ListAllEvents`, capped at `allEventsLimit`). Supports
  `?date=YYYY-MM-DD`; an omitted date defaults to today. Dates select a
  single calendar day in the baby's timezone. Today's response also carries
  unfinished feed, pump, and sleep events from the preceding day so an
  overnight event can be finished without navigating back. Those carryovers
  retain their original `occurred_at` and do not enter today's daily-report
  totals.
* `GET /api/v1/babies/current/events/stream` →
  `StreamTimelineEvents`, an authenticated Server-Sent Events invalidation
  stream for the current baby. It emits `ready` and `timeline_changed`
  events, plus heartbeat comments. Messages contain no event or family data;
  consumers re-fetch the combined events/report APIs after a change. The
  stream closes when its short-lived bearer token expires so callers
  reconnect and revalidate the session.
* `GET /api/v1/babies/current/reports/daily` → `GetDailyReport`, a
  deterministic calendar-day report for the current baby. Supports
  `?date=YYYY-MM-DD`; an omitted date defaults to today. Past dates cover the
  full local calendar day, while today's report runs from midnight to now in
  the baby's timezone. The first version lives in
  `backend-api/internal/handlers/report.go` and summarizes the merged event
  stream into a structured response (`title`, legacy `summary` and
  `highlights`, deterministic `card`, `generated_at`, `range_start`, and
  `range_end`). `card.metrics` always contains feed count, total recorded feed
  volume, and total recorded feed duration; sleep count and duration; pump
  count, recorded volume, and recorded duration; and nappy count.
  The title uses the baby's name and selected day. The card contains no AI
  prose or model-dependent fields.
* `GET /api/v1/babies/current/reports/data` → `GetReportData`, the canonical
  factual report-data payload for one to 31 local calendar days. Supports
  inclusive `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; omitting both
  dates defaults to today. The response includes minimal baby context, range
  metadata, factual totals and baby analytics for the whole range, including
  selected-range comparison against previous-7-day baseline daily averages.
  Medication totals include overall, medicine-item, vaccine-item, and
  other-item counts; nested items are counted without summing mixed medication
  units.
  It returns one deterministic daily report plus factual totals, baby
  analytics, and normalized oldest-first events per day. It also includes
  previous-7-day baseline range metadata, totals, and baby analytics. It
  intentionally does not include AI output yet.
* `POST /api/v1/babies/current/reports/ai` → `CreateAIReport`, the existing
  cached AI generation path for selected daily and weekly range reports and
  scheduled email. It returns `ai_report_output.v2`: up to three ordered
  `insights` and one optional `caveat`. Every report type and
  locale may occasionally use at most one subtle, everyday Australian English
  expression when it fits naturally; locale still controls terminology and
  units.
* Insights event attribution and ongoing-session behavior follow
  [ADR 0007](../decisions/0007-insights-event-attribution.md).
* `GET /api/v1/babies/current/insights/sleep` → `GetSleepInsights`, a
  deterministic sleep-insights payload for the current baby. Supports
  `?range=7|30|90`; an omitted range defaults to 30. Each selection covers
  that many completed local calendar days ending yesterday in the baby's
  timezone, so today never contributes partial data. When the baby was born
  inside the selected period, the range and chart begin on the birth date
  instead of showing pre-birth days. Completed sleeps that cross midnight
  contribute only their overlapping minutes to each day.
  Ongoing sleeps remain recorded activity and keep the range out of the
  no-sleep empty state, but have no duration and never contribute to totals,
  averages, longest-sleep values, or nap/night percentages. Duration-based
  aggregates disclose how many sleep periods supplied recorded durations.
  Daily sleep and completed-period averages use only `recorded_days`:
  calendar days with positive completed recorded sleep duration. Empty days
  after the first recorded day remain visible chart gaps but are treated as
  unknown rather than zero and do not lower averages. A completed sleep is
  counted once, on its start day,
  even when its duration is split across multiple calendar days. The response
  includes chart-ready daily totals and periods, a `carryover_note` for days
  whose period list includes sleep that started the previous day, range-level
  aggregates, and deterministic factual observations. Period clock labels use
  the baby's timezone. Periods crossing a calendar boundary expose
  `started_previous_day` or `continues_next_day`; their labels use "Previous
  day" or "Next day" instead of showing the clipped midnight boundary as an
  actual start or stop time.
* `GET /api/v1/babies/current/insights/nappies` → `GetNappyInsights`, a
  deterministic nappy-insights payload for the current baby. Supports
  `?range=7|30|90`; an omitted range defaults to 30. Like Sleep Insights,
  each selection covers completed local calendar days ending yesterday in
  the baby's timezone and begins on the birth date when it falls inside the
  selected period. Each day includes total, wee, poo, and mixed counts plus
  the recorded changes in chronological order. Each change includes `kind`,
  `time_label`, and, for poo or mixed changes with a valid stored size, `size`
  (`smear`, `small`, `medium`, `large`, or `blowout`). Stored `wet`, `poo`,
  and `both` kinds are presented as wee, poo, and mixed. Range aggregates
  include total changes, `blowout_count`, `large_count`, the average per
  recorded day, average recorded time between changes when at least two
  exist, and a percentage breakdown whose integer values are non-negative
  and sum to 100.
* `GET /api/v1/babies/current/insights/feeds` → `GetFeedInsights`, a
  deterministic feed-insights payload for the current baby. Supports
  `?range=7|30|90`; an omitted range defaults to 30. Like Sleep and Nappy
  Insights, each selection covers completed local calendar days ending
  yesterday in the baby's timezone and begins on the birth date when it falls
  inside the selected period. Each day includes breast-feed duration,
  formula and expressed volume, feed counts, and the recorded feeds in
  chronological order. A feed's count, volume, and full recorded duration are
  attributed to the local calendar day on which it started; feeds are not
  split at midnight, and feeds that started outside the selected range do not
  contribute. A missing `duration_minutes` value marks any feed type as
  ongoing. Ongoing feeds remain in event and frequency counts without an
  invented duration. Breast Insights use only recorded breast-feed durations;
  formula and expressed Insights use recorded volume, including volume already
  known for an ongoing bottle feed, and intentionally ignore bottle-feed
  duration. Breast-duration totals disclose how many recorded breast feeds
  supplied durations. Range aggregates include total and per-type feed counts,
  duration and volume totals, the average per recorded day, average recorded
  time between feed starts when at least two exist, and a percentage breakdown
  by feed type whose integer values sum to 100.
* `GET /api/v1/babies/current/insights/pump` → `GetPumpInsights`, a
  deterministic pump-insights payload for the current baby. Supports
  `?range=7|30|90`; an omitted range defaults to 30. Like Sleep, Nappy, and
  Feed Insights, each selection covers completed local calendar days ending
  yesterday in the baby's timezone and begins on the birth date when it falls
  inside the selected period. Each day includes session count, total
  expressed volume, total pumping duration, and the recorded sessions in
  chronological order. A pump session's count, volume, and full recorded
  duration are attributed to the local calendar day on which it started;
  sessions are not split at midnight, and sessions that started outside the
  selected range do not contribute. Sessions without `duration_minutes`
  remain counted; an ongoing session is labelled `Ongoing` and contributes no
  invented duration. Pump-duration totals and averages disclose how many
  sessions supplied durations. Range aggregates include session count, volume
  and duration totals, the average per recorded day, average volume per
  session, average recorded duration per session, and average recorded time
  between sessions when at least two exist.
* `GET /api/v1/babies/current/insights/growth` → `GetGrowthInsights`, a
  deterministic growth-insights payload for the current baby. Supports
  `?metric=weight|length|head_circumference` and
  `?range=90|180|365|0`; omitted values default to weight over 180 days, and
  `0` means all recorded history. The displayed range and chart begin on the
  birth date when it falls inside the selected history; otherwise they begin
  on the first recorded measurement in that history. Fixed ranges with no
  measurements retain their selected-period start. Each point represents one
  recorded growth measurement event; the birth date does not add a synthetic
  measurement. Points include their stable event ID, exact timestamp, local
  calendar date, formatted value, and change from the true previous
  measurement for that metric, including when that previous measurement falls
  outside the selected range. The response also includes deterministic
  range-level counts, intervals, changes, and factual observations without
  medical interpretation.
* `GET /api/v1/babies/current/insights/overview` → `GetOverviewInsights`, a
  deterministic payload for the Insights Overview tab's recorded-stats card:
  one aggregate per category rather than the day-by-day detail the endpoints
  above return. Supports `?range=7|30|90` (omitted defaults to 30), applied
  to sleep, feeds, nappies, and pump exactly as their own endpoints above;
  growth always reports against the baby's whole recorded history regardless
  of range, since "since birth" has no meaning scoped to a window. Each of
  the five categories is computed independently: a failure or lack of data
  in one never blanks the others. Each category reports `available: true`
  when its lookup succeeded, even when `has_any_data` is false; a failed
  category reports `available: false`, allowing clients to distinguish a
  temporary outage from an empty recorded history. Growth additionally
  reports `has_birth_weight: false` when the baby's profile has no recorded
  birth weight — distinct from having no growth measurements at all — and in
  that case omits `change_since_birth_label` rather than computing a change
  against a missing baseline.
* `PATCH /api/v1/babies/current/events/{id}` → `UpdateEvent`, type-checked
  generic edit for an existing current-baby event.
* `DELETE /api/v1/babies/current/events/{id}` → `DeleteEvent`, removes one
  current-baby event regardless of type.
* Per event type, nested under its plural resource name (`/nappies`,
  `/feeds`, `/pumps`, `/baths`, `/sleeps`, `/observations`,
  `/temperatures`, `/medications`, `/growth-measurements`):
  * `POST /api/v1/babies/current/<resource>` → `Create<Type>`
  * Feed, pump, and sleep events without `duration_minutes` are ongoing.
    Adding a duration completes the event.
  * Medication events contain a non-empty `items` array plus optional `notes`
    and `occurred_at`. Each item has `kind` (`medicine`, `vaccine`, or `other`)
    and a required free-text `name`. Any item may carry an optional free-text
    `description`. Medicines may carry `dose_value` plus `dose_unit`; vaccines
    may carry `series_dose`.
    The backend validates every item, then stores the array in one event through
    the generic event store.
  * Sleep `type` may be omitted on create or generic update. The backend then
    classifies the sleep from its start time: starts from 18:00 through 05:59
    are `night`, and starts from 06:00 through 17:59 are `nap`. Explicit
    `nap` or `night` values remain supported for corrections and older
    clients.

## The generic event store

There is one `events` table (`event_type TEXT`, `attributes JSONB`,
`occurred_at`, plus id/baby_id/created_at). `store.PostgresStore` exposes
generic event methods, shared by every event type:

* `CreateEvent(ctx, eventType string, attributes map[string]any, occurredAt time.Time) (Event, error)`
* `UpdateEvent(ctx, familyID, babyID, id uuid.UUID, eventType string, attributes map[string]any, occurredAt time.Time) (Event, error)`
* `DeleteEvent(ctx, familyID, babyID, id uuid.UUID) error`
* `ListAllEvents(ctx, familyID, babyID uuid.UUID, from, to time.Time, limit int) ([]Event, error)`
* `ListEventsByType(ctx, familyID, babyID uuid.UUID, eventType string, from, to time.Time, limit int) ([]Event, error)`

No event-type-specific SQL exists anywhere — a new event type never touches
`store/postgres.go`.

## Per-event-type handler file (backend-api)

Each event type is one file in `backend-api/internal/handlers/` (`nappy.go`,
`feed.go`, `pump.go`, `bath.go`, `sleep.go`, `observation.go`,
`temperature.go`, `medication.go`, `growth_measurement.go`) containing, and
nothing else:

1. A `const eventType<X> = "<x>"` string.
2. Any enum-like type for constrained fields (e.g. `NappyKind`, `FeedType`)
   with a `Valid()` method — only where the field genuinely has a fixed set
   of values. Free-text fields (like observation's `category`) skip this.
3. `create<X>Request` — the JSON body shape.
4. `<x>Response` — the JSON response shape.
5. `<x>FromEvent(ev store.Event) <x>Response` — maps `ev.Attributes` back to
   the typed response, doing any type coercion (see `attributeInt` in
   `feed.go`, reused by `bath.go`, for the JSONB int/float64 quirk).
6. `Create<X>` handler: decode request → validate/trim → build
   `map[string]any` attributes → call the shared
   `createAndRespond(w, r, h, eventType<X>, attributes, occurredAt, <x>FromEvent)`.
7. Combined reads go through `ListAllEvents`; per-type files do not define
   list handlers.

`createAndRespond` (a generic helper in `handlers.go`) owns the actual
`Store.CreateEvent` call, error logging, and JSON response — every per-type
create handler uses this same single-event path.

**To add a new event type on the backend:** create the new handler file
following the steps above, register its create route in `cmd/server/main.go`,
and add a migration only if the event needs schema changes beyond
`attributes` (usually it does not — JSONB absorbs new fields without a
migration).

## Frontend wiring

`frontend/internal/backendclient` has no per-event-type methods — just
generic `ListEvents(ctx, resource string, date string, out any)`,
`CreateEvent(ctx, resource string, payload map[string]any)`, and
`UpdateEvent(ctx, id string, payload map[string]any)` against
`/api/v1/babies/current/<resource>`. Reads go through the combined
`ListEvents(ctx, "events", selectedDate, &events)` (backend-api's `/events`
endpoint, already merged, date-filtered, and sorted newest-first across
every event type); creates still go through `CreateEvent(ctx, "<resource>", payload)` per type
(`"nappies"`, `"feeds"`, `"pumps"`, `"baths"`, `"sleeps"`,
`"observations"`, `"temperatures"`, `"medications"`,
`"growth-measurements"`), while edits go
through the combined `UpdateEvent` route. The only shape
`backendclient.go` decodes is the generic `Event` struct (`EventType` plus
an `Attributes map[string]any`) — no per-event-type typed view structs.

The UI is a single merged, chronological timeline (not one list per event
type) fed by a single "Add Event" dialog (not one form per event type).
`frontend/internal/handlers/handlers.go`:

* Every event type is flattened into one presentation shape,
  `TimelineEvent` (`CSSClass`, `TypeLabel`, `Kind`, `Detail`, `Time`). Event
  icons are shared inline SVG templates selected by `EventType` in the
  frontend templates. Nappy and sleep discriminators are rendered directly
  as specific `TypeLabel` values (for example `Wee Poo` and `Nap`). `Kind` is
  used for feed/bath's type and observation's category; pump intentionally
  leaves it empty. It is rendered next to `TypeLabel`.
  A `<x>TimelineEvent(ev, loc, now)` function builds one from a generic
  `backendclient.Event`, reading its `Attributes` map — this is where
  per-type display text (e.g. feed's "70 ml · 10 min") is decided.
  `timelineEvent(ev, loc, now)` dispatches to the right builder by
  `ev.EventType`, skipping (and logging) any type the frontend doesn't
  recognize.
* `loadTimeline(ctx, loc, selectedDate)` makes one
  `ListEvents(ctx, "events", selectedDate, ...)` call and converts each item to
  a `TimelineEvent`. One Medication event becomes one standard timeline row;
  its nested items are summarized in that row and edited together through the
  standard event dialog. The backend owns event ordering.
* `Index` calls `loadTimeline` and renders the full page.
* `Index` calls `Backend.GetDailyReport` for the selected date, then renders
  `templates/timeline.html`'s `timeline-workspace` partial. That workspace
  always contains both the daily KPI card and `#timeline`, so HTMX event
  mutations can refresh both together and avoid stale report counts. The
  Timeline filter controls event types only; it does not hide the KPI card.
* The frontend-only `GET /timeline/events/stream` route proxies backend-api's
  private SSE response through the browser's cookie-authenticated frontend
  session. `frontend/static/app.js` opens that same-origin stream and treats
  `ready`/`timeline_changed` as invalidation signals. It re-fetches the
  selected date through the existing HTMX `/app` path, which refreshes the
  full `timeline-workspace` (daily KPI card plus events). Signals are
  debounced, hidden tabs defer their refresh until visible, and every
  successful SSE connection reconciles canonical state. Native `EventSource`
  reconnects automatically. Each workspace carries its selected date, and the
  browser discards an out-of-order workspace response when its date no longer
  matches the latest day-pill choice. A baby-timezone calendar-date check
  performs one full-page reconciliation across midnight: Today advances,
  explicitly selected history remains pinned, and open event editors defer
  navigation so unsaved input is preserved.
* PostgreSQL migration `0012_timeline_event_notifications.sql` installs the
  commit-aware event-table trigger. Each backend-api instance owns one
  dedicated `LISTEN timeline_events_changed` connection and fans opaque
  `baby_id` signals only to matching authenticated subscribers. Notifications
  are non-durable and coalescing; connection/reconnection refreshes reconcile
  canonical state. See
  [How Timeline SSE Updates Work](../sse-timeline-updates.md) for the complete
  connection and mutation sequence.
* Each `Create<X>` handler parses the HTML form, builds a `map[string]any`
  payload (plus `occurred_at` via `parseEventTime`), calls
  `Backend.CreateEvent(ctx, "<resource>", payload)`, then calls the shared
  `renderTimeline` (itself `loadTimeline` + selected-date daily-report load
  + render), so every form's htmx response is the same re-sorted, all-types
  `timeline-workspace` partial (`templates/timeline.html`) swapped over
  `#timeline-workspace` with `outerHTML` — never a per-type partial. The
  selected date is carried in each form/delete request so HTMX refreshes
  preserve the parent's current view.
* `UpdateEvent` uses the same `renderTimeline` tail after patching the
  combined `/events/{id}` route. Timeline quick actions for ongoing feeds,
  pumps, and sleeps post to frontend-only finish routes
  (`/events/{id}/finish-feed`, `/events/{id}/finish-pump`, and
  `/events/{id}/finish-sleep`), which preserve the original event start time
  and call the same backend update route with a calculated duration.

On the client, `frontend/static/app.js` drives the "Add Event" dialog: a
type-picker step (one button per event type) followed by a
form-fields step showing only the chosen type's `<form class="event-form"
data-type="...">` block from `templates/index.html`. Each form still posts
straight to its own existing endpoint (`/nappies`, `/feeds`, `/pumps`, ...); the
dialog only changes what's *shown*, not the request shape.

Timeline cards carry the event's editable values in `data-*` attributes and
open the shared edit dialog when clicked or activated with Enter/Space. The
edit dialog keeps Save disabled until its populated form differs from the
original event and contains the event's immediate Delete action; deletion does
not add a separate confirmation step.

Frontend templates load CSS and JavaScript through content-fingerprinted
`/static/...?...` URLs generated once at server startup. A changed asset gets a
new URL on deployment, so CDN or browser caching cannot combine fresh HTML with
stale client behavior.

**To add a new event type on the frontend:** add a `<x>TimelineEvent`
builder in `handlers.go` (reading from `ev.Attributes`) and a case for it
in `timelineEvent`'s switch; add a `Create<X>` handler ending in
`h.renderTimeline(...)`; add a `<x>` type-choice button and its `.event-form
data-type="<x>"` block in `index.html`; add a `<x>: "Log a <x>"` entry to
`typeLabels` in `app.js`; wire the create/list routes for the new resource
in `cmd/server/main.go` (`/events` already returns every type, no change
needed there); and give the new card colour a light/dark pair in
`style.css`.
