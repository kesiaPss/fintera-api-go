## 1. Covering index

- [x] 1.1 Add `internal/database/migrations/000042_payments_contract_due_date_index.up.sql` creating `idx_payments_contract_due_date` on `payments(contract_id, due_date)`
- [x] 1.2 Add the matching `000042_payments_contract_due_date_index.down.sql`
- [x] 1.3 Run `make migrate-up` and `make migrate-down` against a local database and confirm both directions succeed

## 2. Group overdue notifications by user

- [x] 2.1 Rewrite `PaymentService.CheckOverduePayments` to group overdue payments by `Contract.ApplicantUserID` and call `NotifyUser` at most once per user per run
- [x] 2.2 Add a service test asserting a user with two overdue payments receives exactly one `payment_overdue` notification

## 3. Joined reminder queries

- [x] 3.1 Add `PaymentRepository` methods that fetch overdue and upcoming reminders with a single joined query (`payments` + `contracts` + `users` + `lots` + `projects`) instead of GORM preloads
- [x] 3.2 Update `PaymentService.SendDailyPaymentReminderEmails` and `SendDailyUpcomingPaymentReminderEmails` to use the new joined methods
- [x] 3.3 Add a repository test that asserts the query does not produce separate preload-style `SELECT *` round trips (sqlmock or `EXPLAIN`)

## 4. Minimal credit-score update

- [x] 4.1 Add `UserRepository.UpdateCreditScore(ctx, userID, score) error` that updates only `credit_score` and `updated_at`
- [x] 4.2 Update `CreditScoreService.UpdateScore` to use the new method instead of loading and saving the full user row
- [x] 4.3 Add a test asserting the update succeeds without touching `encrypted_password` or other unrelated columns

## 5. Verification and docs

- [x] 5.1 Run `go build ./...` and `go test ./...` and fix any failure
- [x] 5.2 Run `make lint` (or `golangci-lint run`) and fix any reported issue
- [x] 5.3 Update `docs/architecture/data-model.md` with the new index and `docs/architecture/overview.md` with the notification/credit-score query strategy
