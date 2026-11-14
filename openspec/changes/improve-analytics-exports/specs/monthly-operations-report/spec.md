## Purpose

Provides a monthly operations summary export that consolidates contract
activity, collections, ledger movements, portfolio balances, and lot
distribution into one accounting-ready report.

## ADDED Requirements

### Requirement: Monthly operations export endpoint
The system SHALL expose `GET /api/v1/reports/operations_export` to
authenticated admins and sellers with required query parameter `format`
(`xlsx`, `csv`, or `pdf`) and optional `project_id`, `start_date`, and
`end_date`.

#### Scenario: Successful export in each format
- **WHEN** an authenticated admin calls the endpoint with `format=xlsx`
- **THEN** the system returns an XLSX file whose filename contains the export
  date and whose content uses the requested date range and project filter

#### Scenario: Invalid format
- **WHEN** the caller passes a `format` value other than `xlsx`, `csv`, or
  `pdf`
- **THEN** the system responds with a 400 error and a Spanish message

#### Scenario: Seller scope
- **WHEN** a seller (non-admin) calls the endpoint
- **THEN** the export only includes operations and collections belonging to
  contracts where the seller is the assigned seller or creator

### Requirement: Period operations summary
The export SHALL include a summary of operations for the requested period:
contracts approved, closed, cancelled, and rejected; total sales value;
number and value of sales by financing type (direct, bank, cash); and
down-payment amount collected.

#### Scenario: Operations aggregated by financing type
- **WHEN** the period contains approved contracts with direct, bank, and cash
  financing
- **THEN** the export shows one row per financing type with contract count and
  contract value, plus a total row

#### Scenario: Empty period
- **WHEN** no contracts exist in the requested period
- **THEN** the export is generated with zeroed totals and the same section
  structure, without failing

### Requirement: Collections and effectiveness
The export SHALL include collections for the period broken down by payment
type (reservation, down payment, installment, advance to principal, full
payment), total interest collected, projected amount due in the period, and
the collected-versus-projected effectiveness percentage.

#### Scenario: Effectiveness computed
- **WHEN** the period has collected payments and payments due
- **THEN** the export shows collected total, projected total, and
  effectiveness as a percentage rounded to two decimals

### Requirement: Ledger movements by entry type
The export SHALL include the contract ledger movements contained in the
period, grouped by entry type (`initial`, `interest`, `payment`,
`prepayment`, `adjustment`, `refund`), with the signed amount per type and a
net total consistent with the ledger sign convention.

#### Scenario: Ledger totals reflect sign
- **WHEN** the period contains initial debt, interest, payments, and refunds
- **THEN** the export shows negative amounts for debt-increasing entries,
  positive amounts for debt-reducing entries, and a net total equal to the sum
  of all signed amounts

### Requirement: Portfolio and lot distribution
The export SHALL include outstanding portfolio balance for the filter scope,
the number of contracts with a zero-or-positive balance, and the lot
distribution by status (financed, fully paid, reserved, available, total).

#### Scenario: Portfolio shown with distribution
- **WHEN** the export is generated
- **THEN** it contains an outstanding balance total and a lot distribution
  section with one row per status and a total row
