## Purpose

Provides a monthly income-flow export comparing actual collections against
projected collections, with per-project breakdown and accounting movement
detail, replacing the flat paid-payments CSV dump.

## ADDED Requirements

### Requirement: Income flow export endpoint
The system SHALL expose `GET /api/v1/reports/income_flow_export` to
authenticated admins and sellers with required query parameter `format`
(`xlsx`, `csv`, or `pdf`) and optional `project_id`, `start_date`, and
`end_date`.

#### Scenario: Successful export in each format
- **WHEN** an authenticated admin calls the endpoint with `format=csv`
- **THEN** the system returns a CSV with the same sections and totals as the
  XLSX and PDF versions

#### Scenario: Seller scope
- **WHEN** a seller calls the endpoint
- **THEN** the export only includes collections from that seller's contracts

### Requirement: Monthly actual versus projected flow
The export SHALL contain one row per month in the requested range with actual
collections, projected collections for that month, deviation, and
effectiveness percentage.

#### Scenario: Month fully collected
- **WHEN** a month's projected amount is fully collected
- **THEN** its deviation is zero and its effectiveness is 100.00%

#### Scenario: Month without projections
- **WHEN** a month has no projected payments
- **THEN** its effectiveness is shown as zero without a division error

### Requirement: Collections by payment type per month
Each month row SHALL break actual collections down by payment type in Spanish
(reservation, down payment, installment, advance to principal, full payment).

#### Scenario: Mixed payment types
- **WHEN** a month contains installments and a down payment
- **THEN** the month row shows separate amounts per type and a total that
  equals their sum

### Requirement: Per-project breakdown
The export SHALL include a section with collections per project for the
requested period, with one row per project and a total row.

#### Scenario: Two projects with collections
- **WHEN** the period has collections from two projects
- **THEN** the export shows two project rows and a grand total equal to their
  sum

### Requirement: Accounting movement breakdown
The export SHALL include the period's ledger movements grouped by entry type
(`payment`, `interest`, `prepayment`, `adjustment`, `refund`), with signed
totals and a net amount.

#### Scenario: Refund present
- **WHEN** the period contains a refund entry
- **THEN** the refund appears as a separate signed line and is included in
  the net amount

### Requirement: Collection detail with fiscal data
When the format includes detail lines, each line SHALL include payment date,
contract, client, identity, RTN, bank reference (authorization number or payer
account name when available), payment type, project, lot, and amount, using
Spanish labels.

#### Scenario: Payment without bank reference
- **WHEN** a payment has neither authorization number nor payer account name
- **THEN** the bank reference column is left empty and the line is exported
  normally
