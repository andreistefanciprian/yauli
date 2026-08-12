# Insights Event Attribution and Ongoing Sessions

## Context

Sleep, feed, and pump events can contain durations, but they do not represent
the same kind of activity. Sleep is a continuous state whose minutes naturally
belong to every local calendar day it overlaps. Feeding and pumping are short,
discrete activities that parents expect to find on the day they started.

Insights also need truthful behavior when a recorded event has started but has
no known end. Treating such an event as absent loses recorded activity, while
extending it to midnight or the end of a selected range invents duration.

The Insights pages use completed local calendar-day ranges ending yesterday in
the baby's timezone. A stale ongoing event can therefore appear in a selected
historical range even though today itself is excluded.

## Decision

Use the following attribution rules for deterministic Insights:

* Resolve calendar-day and range boundaries in the baby's timezone.
* Sleep is continuous. Split a completed sleep's duration at local midnight so
  each overlapping day receives only its minutes. Keep the sleep period count
  on its start day, even when duration contributes to more than one day.
* Feed and pump events are discrete. Attribute their count, volume, and full
  recorded duration to the local calendar day on which the event started. Do
  not split or carry these values across midnight.
* Apply the same start-time rule at Feed and Pump range boundaries. An event
  starting before the selected range does not contribute, even if its recorded
  duration overlaps the range. An event starting inside the range contributes
  in full, even if its recorded duration ends after the range boundary.
* Sleep, every feed type, and pump use `duration_minutes` to distinguish a
  completed event from an ongoing one. Treat a missing duration as ongoing
  recorded activity. Include it in event and frequency counts on its start
  day, but do not invent a duration or add it to duration totals, averages,
  longest values, duration-based percentage splits, or duration chart bars.
  Known non-duration measurements, such as bottle or pump volume, remain
  available to their volume metrics while the event is ongoing.
* Feed Insights use recorded duration for breast feeds and recorded volume for
  formula and expressed feeds. Formula and expressed feeds still retain their
  duration for completion state and timeline behavior, but their duration does
  not contribute to the bottle Insights metric.
* An ongoing-only range is not an empty-data range. Show the recorded event and
  use `Not yet available` for duration metrics that have no completed input.
* When only some events have recorded durations, disclose the number of events
  that supplied duration data. Frontends may describe totals as recorded time
  and must not imply that missing durations were zero.

These rules belong to Backend API. Frontend and future MCP consumers render or
expose the resulting values without independently recalculating attribution.

## Alternatives Considered

### Split every duration-bearing event at midnight

Rejected for feeds and pumping because it makes a single short activity appear
on two days and separates its duration from its count and volume.

### Attribute every event entirely to its start day

Rejected for sleep because a long overnight sleep would leave the next day's
sleep total artificially empty even though much of the sleep occurred then.

### Extend ongoing events to midnight, now, or the range end

Rejected because the end time is unknown. Doing so would turn missing data into
invented duration and make historical totals depend on when they are viewed.

### Exclude ongoing events completely

Rejected because the start is a real recorded event and remains useful for
frequency, chronology, and the selected day's activity state.

## Consequences

Daily Feed and Pump cards, charts, and range aggregates stay aligned around the
event start date. Sleep duration remains representative of the local days on
which it occurred, while sleep period counts remain stable. Duration metrics
can be unavailable or based on fewer events than frequency metrics, so API
responses expose duration-basis labels and counts for transparent rendering.

Changing an event's start timestamp can move all Feed or Pump values to another
day. Completing an ongoing event adds its recorded duration to its existing
start day without changing its event count.
