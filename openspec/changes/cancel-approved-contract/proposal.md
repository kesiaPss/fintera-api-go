## Why

Admins can cancel an `approved` or `signed` contract from the contract detail
action menu, but the API rejects it: the FSM `cancel` transition and
`Contract.MayCancel` only allow `pending`, `submitted` and `rejected`. When a
sale is cancelled after approval, the money must be accounted for: the schedule
must stop sending collection reminders, the paid amount must be refunded in the
ledger, the lot must be released, and a commission already paid to the seller
must be recoverable as a credit.

## What Changes

- Allow cancelling `approved` and `signed` contracts.
- On cancellation, mark `pending`/`submitted` payments `cancelled` (rows are
  preserved, so collection jobs stop picking them up), keep paid payments, add
  a `refund` ledger entry for the paid amount and a write-off `adjustment` that
  settles the contract balance to zero.
- Release the lot to `available` only when no other active contract uses it.
- When the commission was already paid, create a seller commission credit the
  admin can apply later.
- Record the cancellation audit with the acting admin.
- Remove the contract "reopen" action and endpoint.

**BREAKING**: cancelling no longer deletes the ledger or the schedule; the
history is preserved and the schedule rows are marked `cancelled`.

## Capabilities

### New Capabilities

- `contract-cancellation`: cancelling active contracts with refund and credit.
- `commission-credits`: recoverable credit when a paid commission is reversed.

## Impact

- `internal/statemachine/contract_fsm.go`, `internal/models/contract.go`,
  `internal/models/payment.go`, `internal/models/contract_ledger_entry.go`
- `internal/services/contract_service.go`, `internal/services/report_service.go`,
  `internal/services/commission_service.go`
- `internal/repository/*`, `internal/handlers/contract_handler.go`,
  `internal/handlers/commission_handler.go`
- Migration for `seller_commission_credits`
- `cmd/api/main.go` (remove the reopen route)
- `docs/architecture/domain-flows.md`, `docs/architecture/data-model.md`
