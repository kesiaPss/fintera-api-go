## Purpose

Attach the next collectable payment to contract listing responses at bounded
cost, reading at most one payment row per contract from the database.

## ADDED Requirements

### Requirement: One next-payment row per contract

When contract listings attach the next collectable payment (the earliest-due
pending or submitted installment without a receipt document), the data load
SHALL return at most one payment row per contract from the database, selecting
the same payment the previous full-scan logic produced (earliest due date,
then lowest id). Response shape and payment selection SHALL be unchanged.

#### Scenario: Contract with many installments

- **WHEN** a paginated contract list is rendered and a contract has twelve
  collectable installments
- **THEN** that contract's `next_payment` is the earliest-due installment
- **AND** the association load reads one row for that contract, not all twelve

#### Scenario: Contract with no collectable payment

- **WHEN** a contract in the page has no pending or submitted payment without a
  receipt
- **THEN** its `next_payment` is absent, as before
