## ADDED Requirements

### Requirement: Cancel an approved or signed contract

An admin SHALL be able to cancel an `approved` or `signed` contract. The
contract SHALL become `cancelled` and inactive, its pending and submitted
payments SHALL become `cancelled`, and its lot SHALL be released to `available`
only when no other active contract uses it.

#### Scenario: Cancel an approved contract

- **WHEN** an admin cancels an approved contract
- **THEN** the contract status is `cancelled`
- **AND** its `pending` and `submitted` payments are `cancelled`
- **AND** the lot is released when no other active contract uses it

#### Scenario: Lot with another active contract

- **WHEN** the lot has another active contract
- **THEN** the lot is not released

#### Scenario: Signed contract

- **WHEN** an admin cancels a signed contract
- **THEN** the contract status is `cancelled` with the same refund handling as
  an approved contract

### Requirement: Preserve the financial history with a refund and write-off

Cancelling an active contract SHALL keep the ledger and paid payments, add a
`refund` entry for the paid amount and a write-off adjustment so the final
balance is zero. Collection jobs SHALL not pick up cancelled payments.

#### Scenario: Refund is recorded

- **WHEN** an approved contract with paid installments is cancelled
- **THEN** the paid payment entries remain
- **AND** a `refund` entry is added for the paid amount
- **AND** a write-off adjustment leaves the balance at zero

#### Scenario: No collection reminders

- **WHEN** a contract has been cancelled
- **THEN** its `cancelled` payments are excluded from overdue and reminder
  queries

### Requirement: Cancellation audit

The cancellation SHALL be audited with the acting admin and the note.

#### Scenario: Admin actor recorded

- **WHEN** an admin cancels a contract
- **THEN** an audit entry records the admin, the note and the cancellation

### Requirement: No reopen action

The system SHALL NOT offer a reopen action for cancelled or closed contracts.

#### Scenario: Reopen removed

- **WHEN** an admin opens a cancelled or closed contract
- **THEN** no reopen action or endpoint is available
