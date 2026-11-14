## 1. States and models

- [ ] 1.1 Add `PaymentStatusCancelled` and `Payment.IsCancelled()` in `internal/models/payment.go`
- [ ] 1.2 Add `EntryTypeRefund` in `internal/models/contract_ledger_entry.go`
- [ ] 1.3 Update `Contract.MayCancel()` and remove `MayReopen()` in `internal/models/contract.go`
- [ ] 1.4 Update the FSM `cancel` event and remove `reopen` in `internal/statemachine/contract_fsm.go`

## 2. Cancellation service

- [ ] 2.1 Change `ContractService.Cancel` to accept `actorID` and preserve history (mark payments cancelled, keep paid, refund + write-off ledger entries, guarded lot release, credit when paid)
- [ ] 2.2 Update `ReleaseUnpaidReservations` and the `Cancel` handler to pass the acting admin ID
- [ ] 2.3 Remove `ContractHandler.Reopen` and its route in `cmd/api/main.go`

## 3. Commission credits

- [ ] 3.1 Add `seller_commission_credits` migration and model
- [ ] 3.2 Create a credit when cancelling a contract with `commission_paid_at != nil`
- [ ] 3.3 Expose credits and net payable in `GET /reports/commissions`
- [ ] 3.4 Add an admin endpoint to apply a credit (mark settled, audited)

## 4. Verification

- [ ] 4.1 Add service/repository/handler tests for the cancel flow, refund, write-off, lot guard, credit and actor audit
- [ ] 4.2 `go build ./...`, `go test ./...`, `go vet ./...`, `make lint`, `make swagger`
- [ ] 4.3 Update `docs/architecture/domain-flows.md` and `data-model.md`
