## Why

The latest logs show the overnight/overdue notification workload is now the
main source of slow queries:

- `CheckOverduePayments` creates one notification per overdue payment, so a user
  with several overdue payments receives multiple identical `Pago vencido`
  notifications within seconds.
- The reminder repository methods use `Preload("Contract.Lot.Project")` and
  `Preload("Contract.ApplicantUser")`; GORM resolves each preload as a separate
  per-table `SELECT *`, producing a flood of single-row round trips.
- `CreditScoreService` calls `FindByUser` (which preloads contracts, lots,
  projects, seller and payments) and then `FindByContract` for every contract,
  reloading the same data.
- Single-row primary-key lookups (`contracts.id`, `payments.contract_id`,
  `lots.id`, `users.id`) are now flagged as `Slow SQL` at 140–495 ms. That
  timing points to database load and too many round trips rather than missing
  indexes.
- The credit-score update calls `userRepo.Update`, which uses GORM `Save` and
  rewrites the entire `users` row — including `encrypted_password` — on every
  score change.

## What Changes

- Group overdue payments by applicant user in `CheckOverduePayments` and create
  at most one `payment_overdue` notification per user per run.
- Replace the reminder preload queries with a single joined query that returns
  payments with their contract, applicant user, lot and project in one round
  trip.
- Add migration `000042_payments_contract_due_date_index` with a covering index
  on `payments(contract_id, due_date)` to support the sorted overdue lookups.
- Change `CreditScoreService.UpdateScore` to update only `credit_score` and
  `updated_at` through a targeted repository method, instead of saving the full
  user row.
- Add tests for grouped notifications, the joined reminder query, and the
  minimal credit-score update.

## Capabilities

### New Capabilities

- `overdue-notification-efficiency`: one notification per user per run, joined
  reminder queries, and a covering index for overdue payment lookups.
- `credit-score-minimal-update`: credit-score updates write only the changed
  columns.

### Modified Capabilities

None.

## Impact

- `internal/services/payment_service.go` (`CheckOverduePayments` grouping)
- `internal/repository/contract_repository.go` (reminder query methods,
  `FindByContract`, `FindByUser`)
- `internal/services/credit_score_service.go` (targeted update)
- `internal/repository/user_repository.go` (new `UpdateCreditScore` method)
- `internal/database/migrations/000042_payments_contract_due_date_index.{up,down}.sql`
- Related service/repository tests
