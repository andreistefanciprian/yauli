# Deterministic Wake-Window Analytics

## Context

Parents and clinicians commonly ask for the baby's average wake window. Yauli
already stores completed sleep start times and durations, so the backend can
calculate this without asking the model to infer it.

## Decision

Add `wake_windows` beneath `analytics.intervals.sleeps`:

```json
{
  "count": 2,
  "average_minutes": 148,
  "longest_minutes": 150,
  "shortest_minutes": 145
}
```

A wake window is the time from one completed sleep ending to the next completed
sleep starting. Only boundaries present in the requested analytics range are
used. Ongoing sleeps break the calculation chain because their end is unknown.
Overlapping completed sleep records do not produce a wake window.

The AI report may call `average_minutes` the "average wake window" and express
it as a natural duration. It must present the value as recorded tracking data,
not compare it with age-based norms or provide medical or sleep advice.

## Alternatives Considered

Calculate wake windows in the AI prompt. This would make a deterministic metric
model-dependent and harder to verify.

Infer the first and last wake windows from the report's day boundaries. Those
windows may have started or ended outside the selected data and would be less
trustworthy.

Use "wake-up window" as the name. "Wake window" is shorter and describes the
whole period awake between sleeps rather than the moment of waking.

## Consequences

The analytics response gains an additive nested field. Consumers that ignore
unknown JSON fields remain compatible.

The average is limited to wake windows whose two sleep boundaries are present
and complete in the requested range. Sparse or incomplete sleep logging can
therefore produce no value or a value that does not represent the baby's full
day.
