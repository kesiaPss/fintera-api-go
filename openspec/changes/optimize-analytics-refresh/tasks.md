## 1. Database indexes

- [x] 1.1 Add `internal/database/migrations/000041_analytics_indexes.up.sql` creating `idx_payments_payment_date`, `idx_payments_status_payment_date`, `idx_contracts_status_active_lot`, `idx_lots_project_status`, replacing `idx_analytics_cache_key_project` with a unique index, and adding the partial unique index `idx_analytics_cache_global_key` on `analytics_cache(cache_key) WHERE project_id IS NULL`; delete pre-existing duplicate cache rows before enforcing uniqueness
- [x] 1.2 Add the matching `000041_analytics_indexes.down.sql` dropping only those indexes
- [x] 1.3 Run `make migrate-up` and `make migrate-down` against a local database and confirm both directions succeed

## 2. Batched analytics refresh

- [x] 2.1 Add `projectRepository.FindAllSummaries` (or equivalent) returning `id, name` only, and use it in `AnalyticsService.RefreshCache`; verify with a repository test
- [x] 2.2 Add `AnalyticsRepository` methods that compute overview and lot distribution for all projects in one aggregate query (`GROUP BY lots.project_id`, `LEFT JOIN` so empty projects are present)
- [x] 2.3 Rewrite `RefreshCache` to fill global and per-project cache entries from the batched results without a per-project loop; add a service test asserting the number of repository calls does not grow with project count
- [x] 2.4 Replace `GetProjectPerformance`'s `Preload("Lots")` with `COUNT`/`SUM` aggregates so full lot rows are not selected; verify the SQL in a test or `EXPLAIN`

## 3. Atomic cache writes

- [x] 3.1 Replace the read-then-write in `AnalyticsRepository.SetCache` with a single `clause.OnConflict` upsert on `(cache_key, project_id)`
- [x] 3.2 Add a repository test that runs two concurrent `SetCache` calls and asserts one row per key with no duplicate-key error

## 4. Metric accuracy

- [x] 4.1 Rewrite `getPreviousPeriod` to mirror the selected window (both bounds, one bound, or the default 30-day window) and add table-driven tests for each case
- [x] 4.2 Always compute previous occupancy in `computeOverview` when a comparison window exists; assert the default dashboard no longer returns a hard-coded 100% occupancy change
- [x] 4.3 Change `GetRevenueTrend` to group by calendar month/year (`DATE_TRUNC('month', ...)` or `YYYY-MM`) and return merged points sorted chronologically; add a test with Nov 2025 and Nov 2026 data producing two ordered points
- [x] 4.4 Replace the `[65, 59, 80, 81]` placeholder with weekly aggregates derived from paid payments (or omit `weekly_performance` when no weeks exist); update the model/response and tests accordingly

## 5. Diagnostics

- [x] 5.1 In `pkg/logger/gorm.go`, detect `errors.Is(err, gorm.ErrRecordNotFound)` and log it at debug/info without the `SQL Error` label; add a unit test
- [x] 5.2 Make `logger.Setup` emit structured JSON in every deployed environment (text only when `LOG_FORMAT=text`), so the collector receives a true `level`/`severity`; add a test for the production environment
- [x] 5.3 Pass `cfg.Environment` into `database.Connect` and use it for the GORM log level instead of `os.Getenv("ENVIRONMENT")`; verify production does not emit per-query INFO logs

## 6. Verification and docs

- [x] 6.1 Run `go build ./...` and `go test ./...` and fix any failure
- [x] 6.2 Run `make lint` (or `golangci-lint run`) and fix any reported issue
- [x] 6.3 Update `docs/architecture/` with the analytics refresh strategy and the new indexes
- [ ] 6.4 Deploy to staging and confirm the scheduled refresh completes under the slow threshold with no cache-miss `SQL Error` entries and correctly leveled logs
