# Medication Event Contract

## Context

A single care moment can include several vaccines and medicines. Yauli’s
existing model treats one timestamped timeline occurrence as one event, and
the generic event store already supports structured JSONB attributes.

## Decision

Represent the care moment as one `medication` event. Its attributes contain a
non-empty `items` array and optional event-level `notes`. Every item has a
`kind` of `medicine`, `vaccine`, or `other` and a required free-text `name`;
medicine items may include a positive `dose_value` and `dose_unit`, vaccine
items may include `series_dose`, and other items are name-only.

The Backend API validates every item before calling the existing single-event
`CreateEvent` store method. The timeline renders one standard event row, and
the standard edit and delete operations replace or remove the whole Medication
event.

## Alternatives Considered

Storing every item as an independent event linked by a session identifier was
rejected because it adds a second grouping lifecycle, a batch store method,
and special display/edit behavior for what parents understand as one care
occurrence. Separate medication and vaccine event types were rejected because
they share the same timeline role and core fields.

## Consequences

Medication uses the same create, update, delete, ordering, and timeline paths
as other event types. All items share one time and notes and are edited or
deleted together. Reports count medicine, vaccine, and other items within the
event, but the event itself contributes one row to the timeline and one record
to the generic event count. Correcting one item requires updating the
containing event.
