## Context

See `proposal.md` for motivation. Relevant current state:

- `AnalyticsService.RefreshCache` (`internal/services/analytics_service.go`)
  loads every project with `projectRepo.FindAll`, then calls `GetOverview` and
  `GetDistribution` per project. `computeOverview` issues five current-period
  queries, four previous-period queries and two trend queries; each project
  therefore costs ~11 statements.
- `AnalyticsRepository.GetCache` uses `First` and returns the raw GORM error.
  `SetCache` performs a `First` and then a separate `Create`/`Updates`, so two
  concurrent refreshes can both miss and both insert.
- `GetPreviousPeriod` returns a fixed 30-day window ending the day before a
  hard-coded 30-day "current" start when no dates are supplied, while the
  current metrics in that path are all-time totals. `computeOverview` sets
  previous occupancy to 0 unless `filters.EndDate` is set.
- `GetRevenueTrend` groups by `TO_CHAR(payment_date, 'Mon')` and merges rows by
  the month abbreviation. Projected-only months are appended from a map, so
  their order is non-deterministic.
- `GetProjectPerformance` uses `Preload("Lots")`, which loads full lot rows for
  every project (`SELECT * FROM lots WHERE project_id IN (...)`), and emits a
  hard-coded `WeeklyPerformance: []float64{65, 59, 80, 81}`.
- `pkg/logger/gorm.go` logs any `err != nil` as `Error("SQL Error", ...)`,
  including `gorm.ErrRecordNotFound`.
- `internal/database/database.go` chooses the GORM level from
  `os.Getenv("ENVIRONMENT")`, while `cmd/api/main.go` calls
  `logger.Setup(cfg.Environment)`. `logger.Setup` selects the JSON handler only
  when the environment string is exactly `production`; otherwise it emits text
  lines that a collector may wrap as `severity=info`.
- The only indexes on `payments` are `contract_id`, `due_date`, `status` and
  `approved_at`. `payments.payment_date` is unindexed even though every
  period-filtered revenue/collection aggregate filters on it.

## Goals / Non-Goals

**Goals:**

- Keep the 15-minute refresh well inside its interval as projects grow.
- Keep analytics totals and distributions identical while reducing query
  count and data transferred.
- Make previous-period comparisons and trend ordering correct and stable.
- Make production diagnostics trustworthy: no cache-miss noise, correct
  severity, actionable slow-query warnings.

**Non-Goals:**

- Redesigning the analytics endpoints or the cached payload shapes.
- Changing the export/report endpoints covered by `improve-analytics-exports`.
- Adding a new cache backend (Redis) or a queue.
- Changing the frontend analytics UI.

## Decisions

### Aggregate once per metric, not once per project

Add repository methods that return metrics for every project in one query
(e.g. `GetOverviewByProject`, `GetLotDistributionByProject`) using
`GROUP BY lots.project_id` and `LEFT JOIN` so projects with no activity still
appear. `RefreshCache` then loads the project list once, fills all global and
per-project cache entries from those result sets, and only falls back to the
per-project path for a project explicitly requested outside the job.

Alternative: memoize `FindAll` and leave the loop. Rejected: it keeps the
linear fan-out and does not help as projects grow.

### Index migration as the primary latency fix

Add `000041_analytics_indexes` with:

- `idx_payments_payment_date` on `payments(payment_date)`.
- `idx_payments_status_payment_date` on `payments(status, payment_date)` for
  paid/unpaid period aggregates.
- `idx_contracts_status_active_lot` on `contracts(status, active, lot_id)` for
  occupancy and active-contract counts.
- `idx_lots_project_status` on `lots(project_id, status)` for distribution and
  occupancy per project.

Keep the migration additive and reversible; the down migration drops only these
indexes.

### Replace `SELECT *` with bounded projections

`projectRepository.FindAll` is used by `RefreshCache` only to get IDs and
names. Change the refresh path to read `id, name` through a dedicated query, and
replace `GetProjectPerformance`'s `Preload("Lots")` with grouped counts
(`total_lots`, `financed_lots`) so full lot rows are never selected.

### Atomic cache upsert

Replace the `First` + `Create/Updates` sequence with a single
`Create` using `clause.OnConflict{Columns: cache_key, project_id, DoUpdates: ...}`.
The existing `idx_analytics_cache_key_project` index is not unique, so the
migration replaces it with a unique index. Postgres treats `NULL` project IDs
as distinct, so the same migration also adds a partial unique index on
`(cache_key) WHERE project_id IS NULL` for the global cache entry.

### Fix previous-period math

- When both bounds are provided, previous period = same duration immediately
  before `startDate`.
- When neither is provided, use the most recent complete 30-day window as the
  current comparison window and the 30 days before it as previous; this matches
  what the dashboard displays instead of comparing all-time totals to an
  unrelated window.
- When only one bound is provided, mirror it (same length) rather than falling
  back to 30 days.
- Always compute previous occupancy for the comparison window; do not require
  `EndDate`.

### Year-aware chronological trends

Group by `DATE_TRUNC('month', payment_date)` (or `TO_CHAR(..., 'YYYY-MM')`),
return a `month`/`year` key alongside the display label, and sort the merged
real/projected points by the truncated month. The display label stays `Mon` so
the response shape does not change; two Novembers become two distinct points.

### Real or omitted weekly performance

Compute the last four weeks of collections per project with a
`DATE_TRUNC('week', payment_date)` aggregate. If there is not enough data to
produce four points, either return the available weeks or omit
`weekly_performance`; never emit a constant placeholder.

### Cache miss classification

In `GormLogger.Trace`, treat `errors.Is(err, gorm.ErrRecordNotFound)` as a
non-error: log at debug (or info) without the `SQL Error` label. Keep real
database errors at error level.

### Structured, correctly leveled logs

- `logger.Setup` always emits structured JSON (or a JSON handler in every
  environment) so the `level`/`severity` field is machine-readable; keep the
  text handler only when an explicit `LOG_FORMAT=text` is set for local
  development.
- Derive the GORM level from `cfg.Environment` passed into `database.Connect`,
  not from `os.Getenv`, so the two configurations cannot diverge.
- Keep `200ms` as the slow threshold but do not log a query as both slow and
  info.

## Risks / Trade-offs

- [Aggregated refresh returns subtly different numbers] → Add repository tests
  comparing the batched query against the existing per-project query for a
  fixture dataset; totals and percentages are asserted to match.
- [New indexes slow writes] → The added indexes are read-path indexes on
  append-heavy tables; monitor write latency and keep the migration reversible.
- [Partial unique index fails on existing duplicate global cache rows] → Before
  creating the index, the up migration deletes duplicate `analytics_cache`
  rows, keeping the newest per `cache_key`.
- [Changing the default previous-period semantics changes dashboard deltas] →
  This is the intended fix; the frontend already guards "no previous data", and
  no field is removed.
- [Switching to JSON logs changes the contract with the log collector] → The
  collector already consumes key/value output; validate in staging before
  rollout.

## Migration Plan

1. Ship the index migration first (additive, no behavior change).
2. Deploy the service/repository changes.
3. Verify the next scheduled refresh completes well under 15 minutes and that
   the overview cache contains non-zero `previous_*` values where the prior
   window has data.
4. Rollback: revert the code and run the index down migration; cached rows are
   regenerated by the job.

## Open Questions

- Should `weekly_performance` be omitted entirely, or returned with the weeks
  that exist? The tasks default to "return available weeks, minimum zero rows"
  and confirm with the analytics UI owner.
