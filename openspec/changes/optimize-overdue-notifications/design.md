## Context

See `proposal.md` for motivation. Relevant current state:

- `PaymentService.CheckOverduePayments` calls `repo.FindOverdue`, loops every
  payment, and calls `notificationSvc.NotifyUser` once per payment. The message
  and title are identical for every row.
- `FindOverdueForActiveContracts` and `FindPaymentsDueTomorrowForActiveContracts`
  use `Preload("Contract.Lot.Project")` and `Preload("Contract.ApplicantUser")`.
  GORM implements preloads as separate `SELECT *` statements per association,
  so a batch of N payments costs several round trips plus full-row reads.
- `CreditScoreService.UpdateScore` loads the full user record, sets
  `user.CreditScore`, and calls `userRepo.Update`, which uses GORM `Save` and
  emits an `UPDATE` for every column in `users`.
- `ContractRepository.FindByUser` preloads `Lot.Project`, `Seller` and
  `Payments`; `CreditScoreService` then calls `PaymentRepository.FindByContract`
  for each contract, reloading the contract and its payments.

## Goals / Non-Goals

**Goals:**

- Reduce the number of database round trips during the notification and
  credit-score background jobs.
- Stop emitting duplicate identical `payment_overdue` notifications for the same
  user in a single run.
- Make credit-score updates a narrow, safe write.

**Non-Goals:**

- Changing notification copy, email templates, or the credit-score algorithm.
- Adding new notification channels or types.
- Refactoring unrelated payment/contract paths.

## Decisions

### One notification per user per overdue run

Change `CheckOverduePayments` to group the overdue payments by
`Contract.ApplicantUserID` in memory and call `NotifyUser` once per user. The
7-day throttle for email reminders stays independent.

Alternative: deduplicate in the repository with a partial unique index. Rejected
because the same user can legitimately have multiple historical overdue
notifications; the deduplication should apply only within one job run.

### Joined query for reminder data

Add repository methods that fetch the reminder dataset with one joined query:

```sql
SELECT payments.*, contracts.*, users.*, lots.*, projects.*
FROM payments
JOIN contracts ON contracts.id = payments.contract_id ...
JOIN users ON users.id = contracts.applicant_user_id ...
JOIN lots ON lots.id = contracts.lot_id
JOIN projects ON projects.id = lots.project_id
WHERE ...
```

The service receives `[]models.Payment` with the associations hydrated from the
join rather than from separate preload queries. This keeps the service interface
stable while removing the per-association round trips.

Alternative: keep preloads and rely on the new index. Rejected because the logs
show the preload round trips themselves are slow under load; a single query is
fewer round trips and easier to reason about.

### Covering index

Migration `000042_payments_contract_due_date_index` adds
`idx_payments_contract_due_date` on `payments(contract_id, due_date)`. This
supports `FindByContract`, the overdue lookups, and any report that filters by
contract and orders by due date.

### Minimal credit-score update

Add `UserRepository.UpdateCreditScore(ctx, userID uint, score int) error` that
executes:

```sql
UPDATE users SET credit_score = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
```

`CreditScoreService.UpdateScore` will use this instead of loading the full user
and calling `Save`. This removes the full-row rewrite and avoids the risk of
persisting stale user fields.

## Risks / Trade-offs

- [Joined query transfers more columns per row] → It still transfers fewer total
  rows than N separate queries and eliminates repeated per-table round trips.
- [One notification per user hides multiple overdue payments] → The in-app
  notification is a generic "You have an overdue payment"; the email already
  groups all overdue payments, so this is consistent.
- [Targeted update bypasses GORM hooks/validation] → There are no relevant hooks
  on the user model for this field, and the operation is explicitly scoped.

## Migration Plan

1. Deploy migration `000042` (additive index).
2. Deploy the service/repository changes.
3. Monitor the next notification and credit-score runs for reduced slow-query
   warnings.

## Open Questions

None.
