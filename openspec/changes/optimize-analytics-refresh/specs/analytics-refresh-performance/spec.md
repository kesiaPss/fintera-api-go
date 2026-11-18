## Purpose

Keep the scheduled analytics cache refresh accurate, bounded and safe to run
every 15 minutes as the number of projects grows.

## ADDED Requirements

### Requirement: Refresh cost is independent of project count

The analytics cache refresh SHALL compute global and per-project metrics from a
bounded set of aggregate queries. Adding projects SHALL NOT add a proportional
number of round trips, and a full refresh SHALL complete within its 15-minute
schedule for the current project volume.

#### Scenario: Many projects, one refresh

- **WHEN** the scheduled refresh runs with 15 active projects
- **THEN** the number of analytics queries does not grow in proportion to the
  project count
- **AND** the refresh completes before the next scheduled run begins

#### Scenario: Project with no activity

- **WHEN** a project has no payments or contracts
- **THEN** its per-project cache entry still exists with zero totals
- **AND** it does not require a separate per-project query to be produced

### Requirement: Aggregations use supporting indexes

Period-filtered payment aggregates SHALL use an index on the filtered date
column, and analytics reads SHALL NOT scan full `projects` or `lots` rows when
only identifiers or counts are needed.

#### Scenario: Period-filtered revenue

- **WHEN** an overview or trend is requested for a date range
- **THEN** the query plan for the paid-payment aggregate uses the payment-date
  index instead of a sequential scan

#### Scenario: Project performance

- **WHEN** per-project performance is computed
- **THEN** only aggregated lot counts are read, not every lot row

### Requirement: Cache writes are atomic

Writing an analytics cache entry SHALL be a single atomic upsert keyed by cache
key and project scope, so concurrent refreshes cannot insert duplicates or fail
on a uniqueness constraint.

#### Scenario: Concurrent refresh

- **WHEN** two refreshes write the same cache key at the same time
- **THEN** exactly one row exists for that key and project scope
- **AND** no duplicate-key error is returned

#### Scenario: Global and per-project entries coexist

- **WHEN** the global overview and a project overview share the same cache key
- **THEN** both entries are stored and read independently
