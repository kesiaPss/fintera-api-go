## Purpose

Keep scheduled background jobs from concentrating memory and CPU load at
process start, and bound the datasets they read by processing overdue payments
in batches.

## ADDED Requirements

### Requirement: Startup job runs are staggered

After a start or restart, the heavy recurring jobs (overdue interest and
notifications, credit scores, reservation release and reminders, daily emails,
assignment expiry, seller score recompute) SHALL begin their first run at a
randomized offset within 90 seconds of startup, and then continue on their
existing intervals. The scheduler SHALL cancel a delayed first run during
graceful shutdown without executing it.

#### Scenario: Deploy does not fire every job at once

- **WHEN** the process starts
- **THEN** the immediate jobs' first runs are spread over up to 90 seconds
  instead of all starting simultaneously

#### Scenario: Shutdown during the delay window

- **WHEN** the process shuts down while a first run is still waiting for its
  delay
- **THEN** the waiting run is abandoned without executing and shutdown
  completes within its budget

#### Scenario: Jobs still run after restart

- **WHEN** the process restarts mid-cycle
- **THEN** each job still performs a run shortly after startup (within the
  delay window) and then keeps its normal interval

### Requirement: Overdue payment scans are batched

The overdue interest calculation and overdue notification runs SHALL process
pending overdue payments of approved contracts in fixed-size batches rather
than loading all matching payments at once, applying ledger interest entries,
payment interest updates, and user notifications per batch. Observable
outcomes per run SHALL be unchanged: one interest ledger entry per overdue
installment (upsert semantics), the same accumulated interest totals, and at
most one overdue notification per user per run, with a single admin summary.

#### Scenario: Interest job with many overdue installments

- **WHEN** the overdue interest job runs with thousands of overdue
  installments
- **THEN** it processes them batch by batch and writes ledger entries and
  payment updates incrementally
- **AND** admins receive one summary notification for the run

#### Scenario: Notification dedupe preserved across batches

- **WHEN** one user has overdue payments in two different batches
- **THEN** the user receives exactly one `payment_overdue` notification for
  the run
