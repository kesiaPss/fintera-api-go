## Purpose

Provides a commissions export that groups commissions by seller, separates
paid from pending commissions, and reports credits and net pending amounts so
sellers and accounting can reconcile payouts.

## ADDED Requirements

### Requirement: Commissions export endpoint
The system SHALL expose `GET /api/v1/reports/commissions_export` to
authenticated admins and sellers with required query parameter `format`
(`xlsx`, `csv`, or `pdf`) and optional `start_date` and `end_date` filtering
by commission date.

#### Scenario: Successful export in each format
- **WHEN** an authenticated admin calls the endpoint with `format=pdf`
- **THEN** the system returns a PDF with the same sections and totals as the
  XLSX and CSV versions

#### Scenario: Seller scope
- **WHEN** a seller calls the endpoint
- **THEN** the export only contains commissions belonging to that seller

### Requirement: Grouping by seller with subtotals
The export SHALL group commission lines by seller and SHALL show, per seller,
the total estimated commission, total paid commission, and total pending
commission, plus a grand total at the end of the report.

#### Scenario: Multiple sellers
- **WHEN** commissions exist for three different sellers
- **THEN** the export contains three seller sections, each with its own
  subtotal block, followed by one grand-total block

#### Scenario: Seller without commissions in period
- **WHEN** a seller has no commissions in the requested period
- **THEN** that seller does not appear in the export

### Requirement: Paid and pending distinction
Each commission line SHALL display its state explicitly in Spanish as paid or
pending, and paid lines SHALL include the payment date.

#### Scenario: Paid commission line
- **WHEN** a contract commission has a payment date
- **THEN** the line shows the Spanish paid status together with that date and
  is rendered with the paid visual style

#### Scenario: Pending commission line
- **WHEN** a contract commission has no payment date
- **THEN** the line shows the Spanish pending status and is rendered with the
  pending visual style

### Requirement: Credits and net pending
The export SHALL include a final section with pending seller commission
credits and the net pending amount per seller, computed as pending commissions
minus pending credits.

#### Scenario: Seller with pending credits
- **WHEN** a seller has pending credits and pending commissions
- **THEN** the export lists the pending credit amounts and shows the net
  pending amount for that seller

#### Scenario: No pending credits
- **WHEN** no pending credits exist
- **THEN** the net pending amount equals the pending commission total

### Requirement: Commission line detail
Each commission line SHALL include contract identifier, client name, lot,
project, financing type, contract value, sale date, origin, and commission
amount, using Spanish labels.

#### Scenario: Historical sale included
- **WHEN** a historical sale falls inside the period
- **THEN** it appears as a commission line with origin shown in Spanish as
  historical
