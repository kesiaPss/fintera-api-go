## Why

The analytics cache refresh is the heaviest recurring job and the logs from
2026-11-18 20:00 UTC show it behaving badly:

- A single refresh starts at `20:00:28` and is still running past `20:01:09`
  for only 15 projects, because `RefreshCache` calls `GetOverview` and
  `GetDistribution` once per project and each call fans out into ~10 separate
  aggregate queries. Cost grows linearly with the number of projects.
- `Slow SQL` entries report `SELECT * FROM "projects"` at 212–363 ms for 15
  rows and `SELECT * FROM "lots" WHERE project_id IN (...)` at 287 ms for 823
  rows. `payments.payment_date`, the column behind almost every period filter,
  has no index.
- Every cache lookup keeps logging `SQL Error ... error="record not found"` at
  ERROR level. A cache miss is the normal path that immediately inserts the
  row; it is not a failure and it drowns real errors.
- The deployed log lines are text (`time=... level=ERROR msg=...`) wrapped by a
  collector that stamps `severity=info` on everything, so real `ERROR`/`WARN`
  entries are reported as informational.
- Every cached overview contains `previous_revenue: 0`, `previous_contracts: 0`,
  `previous_average_payment: 0`, `previous_occupancy_rate: 0` and
  `*_change_percentage: 100`, because the "previous period" defaults to an
  arbitrary 30-day window unrelated to the current all-time totals and previous
  occupancy is only computed when an end date is set.
- Revenue trend labels come back out of order (`Nov, Dec, Jan, Feb, Mar, Apr,
  Jun, Oct, May, Jul, Aug, Sep`) and are grouped only by month abbreviation, so
  November 2025 and November 2026 would collapse into one bucket.
- `weekly_performance` is the constant `[65,59,80,81]` for every project.

## What Changes

- Replace the per-project refresh loop with bounded, aggregated queries: one
  pass per metric across all projects instead of one pass per project. The
  scheduled refresh cost must not grow linearly with project count.
- Add an index migration for `payments.payment_date` and the composite indexes
  the analytics aggregates need; stop loading full `projects`/`lots` rows when
  only counts are required.
- Make cache writes a single atomic upsert so concurrent refreshes cannot race
  between the read and the insert.
- Normalize the previous-period calculation so it mirrors the selected window,
  computes previous occupancy for the default dashboard, and never derives a
  comparison from an unrelated default range.
- Group revenue trend points by calendar year and month and return them in
  chronological order.
- Derive project weekly performance from real data, or omit it instead of
  emitting a placeholder series.
- Treat `record not found` on cache lookups as a cache miss (debug/info), not
  an error.
- Emit structured logs whose severity matches the level in deployed
  environments, and derive the GORM log level from the same environment
  configuration as the rest of the app.

No API route, request shape, or response shape is removed. The overview,
distribution and performance payloads keep their current fields; the values and
ordering become correct.

## Capabilities

### New Capabilities

- `analytics-refresh-performance`: bounded scheduled refresh cost, indexed
  aggregation queries, and atomic cache writes.
- `analytics-metric-accuracy`: correct previous-period comparisons, chronological
  year-aware revenue trends, and real (or omitted) weekly performance.
- `diagnostic-logging`: cache misses are not errors, slow-query warnings, and
  structured severity in deployed logs.

### Modified Capabilities

None. This repository has no synced base specs; the new capabilities capture
the required behavior.

## Impact

- `internal/services/analytics_service.go` (`RefreshCache`, `computeOverview`,
  `getPreviousPeriod`)
- `internal/repository/analytics_repository.go` (aggregates, cache upsert,
  revenue trend, project performance)
- `internal/repository/contract_repository.go` (`projectRepository.FindAll`)
- `internal/database/database.go` (GORM log level environment source)
- `pkg/logger/gorm.go` (record-not-found handling)
- `pkg/logger/logger.go` (handler selection and severity)
- `internal/database/migrations/000041_analytics_indexes.{up,down}.sql`
- Analytics service/repository tests
- No consumer change required for FinteraUI or FinteraWeb; the response shapes
  are unchanged.
