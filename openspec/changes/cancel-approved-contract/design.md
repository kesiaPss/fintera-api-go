## Context

See `proposal.md`. Current behavior:

- `Contract.MayCancel()` allows only `pending`, `submitted`, `rejected`; the FSM
  `cancel` event mirrors that list.
- `ContractService.Cancel` sets the status, deletes every ledger entry, deletes
  `pending`/`submitted` payments, releases the lot unconditionally, and logs the
  audit with `contract.ApplicantUserID` (the customer) instead of the admin.
- The commission report filters `status IN (approved, closed)`, so cancelling
  drops the contract (and any paid commission) from the report.
- `ContractHandler.Reopen` is a no-op stub; the UI offers "Reabrir".

## Goals / Non-Goals

**Goals:**

- Cancel an `approved`/`signed` contract while preserving the financial history.
- Stop all collection emails for the cancelled schedule.
- Reflect the refund and the write-off in the ledger.
- Release the lot safely.
- Recover a paid commission as an admin-applied credit.
- Remove the reopen action.

**Non-Goals:**

- Automating refunds to the customer (refund is recorded, not wired to payments
  rails).
- Automatically offsetting commission credits against future payouts (the admin
  applies them).

## Decisions

### States

`MayCancel()` returns true for `pending`, `submitted`, `rejected`, `approved`,
`signed`. The FSM `cancel` event adds `approved` and `signed` as sources. The
`reopen` event, `MayReopen` and `ContractHandler.Reopen` are removed.

### Cancellation flow (`ContractService.Cancel(ctx, id, note, actorID)`)

1. Load the contract with payments and the lot.
2. Validate with the FSM and store `status = cancelled`, `active = false`, note.
3. Payments: update every `pending`/`submitted` payment to `cancelled` (batch).
   Paid payments are left untouched.
4. Ledger (preserve history, settle to zero):
   - Keep the `initial` entry and the paid `payment` entries.
   - `refund` entry for `-paidTotal` (reverses the payment credits).
   - `adjustment` write-off for `+amount` (the contract amount) so the final
     balance is `0`.
5. Lot: release to `available` only if `FindActiveByLot` returns no other active
   contract (excluding this one).
6. Commission: if `commission_paid_at != nil`, create a
   `seller_commission_credits` row (seller, contract, amount, `pending`).
7. Audit: `CANCEL` with `actorID`, note, refunded amount and generated credit.

`ReleaseUnpaidReservations` keeps using `Cancel` with a system actor and no
refund (nothing paid yet); pending/submitted reservations still clean up.

### Commission credit

New table `seller_commission_credits`:

- `id`, `seller_id`, `contract_id`, `amount`, `status` (`pending|settled`),
  `created_at`, `settled_at`, `settled_by`.

The admin commissions report exposes pending credits and the net payable. An
admin endpoint applies a credit (marks it `settled`, audited). The seller
commissions page shows the credit read-only.

## Risks / Trade-offs

- [Refund is recorded but not paid out] → Documented as an accounting entry;
  the actual refund to the customer stays a manual/offline step.
- [Lot has another active contract] → Guard with `FindActiveByLot`; do not
  release in that case.
- [Paid commission history disappears] → The credit row preserves the reversal;
  the original contract keeps `commission_amount`/`commission_paid_at`.
- [Collection jobs] → They filter `pending`/`submitted`, so `cancelled` payments
  are automatically excluded; covered by tests.

## Migration Plan

- Additive migration for `seller_commission_credits`.
- No data migration for existing contracts; the new status/entry types only
  apply to future cancellations.
- Rollback: revert the migration; the cancel endpoint stays backward compatible
  for pending/submitted/rejected.

## Open Questions

None.
