# Health History in Insights Overview

## Context

Parents need a compact health summary and a way to review the vaccinations and
medicine doses already recorded in Yauli. These records describe the baby's
whole history rather than the selected 7, 30, or 90-day Insights range.

## Decision

Add a `health` category to `GET /api/v1/babies/current/insights/overview`.
Backend API owns the all-time vaccination count, most recent vaccination
details, recent medicine rows, complete newest-first vaccination and medicine
histories, recorded vaccine descriptions, and age-at-recording labels. The
frontend renders the summary and expands the histories with a URL-backed
`history=1` view state.

The histories are returned with every Overview response. This keeps the first
version on one existing request and avoids a second route or conditional API
contract while recorded medication histories remain small.

The frontend selects Overview when `/insights` has no valid `category` query.
Sleep and every other category remain available through explicit category
links, and their range or day links preserve that category.

## Alternatives Considered

A separate health-history endpoint or an `include_history` parameter would
avoid returning collapsed history rows. Both were rejected for now because
they add another request and contract for data the Overview response already
needs to summarize.

## Consequences

The Overview payload is no longer aggregate-only: its health category includes
the records used by the expandable history panel. Health is independent of the
range pills and degrades independently from the other categories. If history
sizes make the response materially slow, history can move behind an explicit
request without changing how medication events are stored.
