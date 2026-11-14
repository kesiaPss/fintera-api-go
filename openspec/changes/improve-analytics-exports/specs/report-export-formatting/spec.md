## Purpose

Defines the cross-format presentation rules shared by every financial export so
XLSX, CSV, and PDF outputs are consistent, legible, and fully written in
Spanish.

## ADDED Requirements

### Requirement: Spanish-only exported content
All exported files SHALL use Spanish for sheet names, section titles, column
headers, status labels, financing types, payment types, origins, messages, and
file names.

#### Scenario: Statuses translated
- **WHEN** an export contains paid and pending commissions
- **THEN** they are labelled in Spanish and no English status label appears in
  the file

### Requirement: Consistent visual style across formats
All financial exports SHALL share the same visual identity: a title banner,
section headers, alternating row shading, a single palette, and consistent
money and date formatting.

#### Scenario: XLSX styled
- **WHEN** an XLSX export is generated
- **THEN** the sheet has a title row, styled section headers, frozen header
  rows, sensible column widths, numeric money cells formatted with two
  decimals, and date cells formatted as dates

#### Scenario: PDF styled like existing reports
- **WHEN** a PDF export is generated
- **THEN** it uses the same layout system as the existing contract and balance
  PDFs, including title banner, section titles, table headers, and footer with
  page numbers

#### Scenario: CSV remains plain data
- **WHEN** a CSV export is generated
- **THEN** it contains the same sections and Spanish headers as the other
  formats without visual styles, and numbers are rendered without currency
  symbols

### Requirement: Currency-aware money formatting
Exports SHALL render money amounts using the currency of the underlying
record, with the Honduran lempira symbol for HNL and the dollar symbol for
USD, and two decimal places.

#### Scenario: USD record
- **WHEN** an exported record is denominated in USD
- **THEN** its amount is rendered with the dollar symbol and two decimals

#### Scenario: CSV money values
- **WHEN** a CSV export renders money values
- **THEN** values are numeric with two decimals and no currency symbol,
  preserving the currency in a separate column where relevant

### Requirement: Fiscal and bank reference columns
Exports that include client-level detail SHALL include the client identity,
the RTN, and the bank reference columns (authorization number and payer
account name) when the source record provides them, leaving them empty
otherwise.

#### Scenario: Client with fiscal data
- **WHEN** a client has identity and RTN registered
- **THEN** both values appear in their corresponding columns

### Requirement: Export response contract
Every export endpoint SHALL return a downloadable file with a filename that
identifies the report, the period, and the extension matching the requested
format, using `application/octet-stream` (XLSX/PDF) or `text/csv` (CSV).

#### Scenario: Filename identifies report and period
- **WHEN** an admin exports the operations report for October 2026 in XLSX
- **THEN** the downloaded filename contains a Spanish report name and the
  period, ending in `.xlsx`
