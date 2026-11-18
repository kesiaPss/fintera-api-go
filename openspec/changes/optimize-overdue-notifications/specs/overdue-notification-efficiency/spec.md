## Purpose

Make the overnight overdue notification workload fast and avoid spamming a user
with identical notifications.

## ADDED Requirements

### Requirement: One in-app notification per user per overdue run

The overdue-payment check SHALL create at most one `payment_overdue`
notification per applicant user in a single run, regardless of how many overdue
payments that user has.

#### Scenario: Multiple overdue payments for one user

- **WHEN** a user has two overdue payments and the overdue check runs
- **THEN** exactly one `payment_overdue` notification row is inserted for that
  user

#### Scenario: Different users with overdue payments

- **WHEN** two different users each have overdue payments
- **THEN** each user receives one notification

### Requirement: Reminder queries use a single joined round trip

The daily overdue and upcoming reminder jobs SHALL fetch payments together with
their contract, applicant user, lot and project in one query, instead of
issuing separate per-association queries.

#### Scenario: Fetch overdue reminders

- **WHEN** the daily overdue reminder job runs
- **THEN** the repository returns the full reminder dataset with one joined
  query
- **AND** the number of round trips does not grow with the number of payments

#### Scenario: Fetch upcoming reminders

- **WHEN** the daily upcoming-reminder job runs
- **THEN** the repository returns the full reminder dataset with one joined
  query

### Requirement: Overdue lookups use a covering index

Queries that filter or sort payments by `contract_id` and `due_date` SHALL use
an index that covers both columns.

#### Scenario: Payments by contract ordered by due date

- **WHEN** a query requests payments for a contract ordered by due date
- **THEN** the query plan uses the `idx_payments_contract_due_date` index
