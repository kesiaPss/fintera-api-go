## Purpose

Provides an overdue-payments export with 30/60/90+ aging buckets, summary
totals by bucket, and client fiscal/contact detail, limited to active
contracts, so collections teams and accounting can prioritize recovery.

## ADDED Requirements

### Requirement: Overdue export endpoint
The system SHALL expose `GET /api/v1/reports/overdue_export` to authenticated
admins and sellers with required query parameter `format` (`xlsx`, `csv`, or
`pdf`).

#### Scenario: Successful export in each format
- **WHEN** an authenticated admin calls the endpoint with `format=xlsx`
- **THEN** the system returns an XLSX file containing the aging summary and
  the overdue detail

#### Scenario: Seller scope
- **WHEN** a seller calls the endpoint
- **THEN** the export only contains overdue payments from contracts owned by
  that seller

### Requirement: Aging buckets
The export SHALL classify each overdue payment into exactly one bucket based
on days past due: 1–30, 31–60, 61–90, and more than 90 days.

#### Scenario: Payment 45 days overdue
- **WHEN** a payment is 45 days past its due date
- **THEN** it is counted and summed in the 31–60 bucket

#### Scenario: Payment exactly 90 days overdue
- **WHEN** a payment is exactly 90 days past its due date
- **THEN** it is counted in the 61–90 bucket and not in the 90+ bucket

### Requirement: Bucket summary
The export SHALL contain a summary table with one row per aging bucket showing
number of payments, principal amount, accumulated interest, and total amount,
plus a grand total row.

#### Scenario: Summary matches detail
- **WHEN** the export is generated
- **THEN** the sum of each bucket row equals the total of the corresponding
  detail lines and the grand total equals the sum of all buckets

### Requirement: Cancelled contracts excluded
The export SHALL exclude overdue payments belonging to cancelled contracts.

#### Scenario: Cancelled contract with overdue payment
- **WHEN** a contract is cancelled and has a payment past its due date
- **THEN** that payment does not appear in the export nor in bucket totals

### Requirement: Overdue detail lines
Each detail line SHALL include payment identifier, contract identifier,
client name, identity, RTN, phone, due date, days past due, principal amount,
accumulated interest, contract balance, and payment type, using Spanish
labels. Empty fiscal or bank values SHALL be rendered as an empty value rather
than an error.

#### Scenario: Client without RTN
- **WHEN** the client has no RTN registered
- **THEN** the RTN column is left empty and the rest of the line is exported
  normally

#### Scenario: Severity styling
- **WHEN** lines belong to different aging buckets
- **THEN** each line is rendered with the severity style of its bucket, with
  the 90+ bucket using the most severe style
