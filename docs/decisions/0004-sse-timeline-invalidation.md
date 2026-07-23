# SSE Timeline Invalidation

## Context

An event created through MCP or another parent's device should appear quickly
on every open dashboard without requiring the user to reload the page. The
update mechanism must avoid repeating frontend authentication, backend API,
and database work when nothing has changed.

Yauli's backend API is private, while browsers authenticate to the public
frontend with an HttpOnly session cookie. Event mutations can originate from
the frontend or MCP, so the notification source must sit behind the Backend
API rather than in either thin client.

## Decision

PostgreSQL emits a baby-scoped invalidation notification after every committed
insert, update, or delete on `events`. The payload contains only `baby_id`;
event contents and family data are never placed on the notification channel.

Each backend-api instance holds one dedicated `LISTEN` connection and
coalesces notifications into baby-scoped in-process subscriptions. An
authenticated `GET /api/v1/babies/current/events/stream` endpoint exposes the
current baby's subscription as Server-Sent Events. It sends named `ready` and
`timeline_changed` events plus heartbeat comments.

The public frontend proxies that private stream at
`GET /timeline/events/stream`. The proxy mints the same short-lived backend
access token used by normal frontend requests; backend-api closes the stream
when that token expires so native `EventSource` reconnects through the
frontend and revalidates the durable session.

The browser treats SSE messages only as invalidation signals. It re-fetches
the selected date's canonical `timeline-workspace` HTML through the existing
HTMX `/app` path, refreshing both the deterministic daily KPI card and event
list. Signals are debounced, and hidden tabs defer work until visible. Native
`EventSource` reconnection plus a canonical refresh on each `ready` event
repair state after an interrupted connection.

PostgreSQL notifications are explicitly not a durable event log. The browser
reconciles whenever a stream connects, and the backend publishes a general
resync after its database listener reconnects.

## Alternatives Considered

Use WebSockets. Timeline delivery is one-way; existing HTTP and HTMX requests
already handle every client-to-server mutation. Bidirectional framing would
add capability Yauli does not need.

Add Redis or another message broker. PostgreSQL already participates in every
event transaction and can provide commit-aware invalidation at the expected
scale without another service.

Send full event payloads over SSE. This would create another externally
visible event contract, duplicate timeline mapping behavior, and make missed
messages correctness-sensitive. Re-fetching canonical data keeps clients thin.

Connect the frontend directly to PostgreSQL. That would violate the service
boundary: only backend-api owns event persistence and family/baby isolation.

## Consequences

Cross-device and MCP timeline updates normally appear within a few seconds.
No new external infrastructure or frontend framework is required.

Each backend-api instance consumes one PostgreSQL pool connection for
`LISTEN`. Each open dashboard holds one browser-to-frontend stream and one
frontend-to-backend stream. Slow clients cannot block notification delivery;
their pending signals coalesce because a single invalidation is sufficient.

SSE delivery may be duplicated or missed during a disconnect, so consumers
must remain idempotent and re-read canonical state. A local web mutation may
also cause both its normal HTMX response and an SSE-triggered refresh; this is
safe and can be optimized later only if measurements show it matters.

Railway or another HTTP intermediary must allow streaming responses and must
not buffer them. Heartbeats and no-buffer/no-transform response headers make
that behavior explicit, but deployment smoke testing remains required.
