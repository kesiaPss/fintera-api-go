## Purpose

Make the financial report exports (operations, commissions, overdue, income
flow) load only the rows in scope, in bounded chunks, and deliver the rendered
file without buffering the whole document in memory, producing byte-identical
CSV/PDF output and a visually equivalent XLSX.

## ADDED Requirements

### Requirement: Report rows are scoped at the data source

Report exports SHALL select rows for the requested seller and project scope in
the data query itself; rows outside the caller's scope SHALL NOT be transferred
from the database. The exported content SHALL be identical to what the previous
Go-side filtering produced.

#### Scenario: Seller export only contains own rows

- **WHEN** a seller exports the income flow or overdue report
- **THEN** only payments belonging to contracts of that seller (as seller or
  creator) are read and rendered

#### Scenario: Project filter applied in the dataset

- **WHEN** an admin exports the income flow report with `project_id`
- **THEN** only payments of lots in that project are read and rendered

### Requirement: Exports over large datasets complete with bounded reads

The commissions and overdue exports, and the income-flow collection detail,
SHALL read matching rows in fixed-size batches (about 500 rows) instead of
loading the whole dataset at once, so that per-batch memory does not grow with
total row count. Exports SHALL keep default periods and filters unchanged.

#### Scenario: Commissions export with no date filter

- **WHEN** an admin exports the commissions report for all history
- **THEN** the export completes and contains every approved/closed contract in
  scope, read in batches

#### Scenario: Wide date range income flow

- **WHEN** an admin exports the income flow report across several years
- **THEN** the export completes without loading every payment's associations at
  once, and the detail rows match the payments in scope

### Requirement: Export files are streamed to the response

Report export endpoints SHALL write the rendered file directly to the HTTP
response after setting the same status, `Content-Type`, and
`Content-Disposition` headers as before. If building the document fails, the
endpoint SHALL still respond with the Spanish JSON error before any file bytes
are written.

#### Scenario: Successful export response unchanged

- **WHEN** a report export succeeds in any supported format
- **THEN** the client receives the same file content, filename, and content
  type as before this change

#### Scenario: Builder failure before streaming

- **WHEN** a report build fails (e.g. invalid data)
- **THEN** the response is a JSON error with status 500 and no file bytes

### Requirement: Rendered file compatibility

CSV and PDF exports SHALL remain byte-identical to the current renderers for
the same document. XLSX exports SHALL preserve the current visual structure:
merged title banner, metadata rows, section titles, styled table headers,
zebra rows, totals, column widths, and the frozen header pane.

#### Scenario: XLSX visual parity

- **WHEN** the same report document is rendered before and after this change
- **THEN** the XLSX opens with identical sheets, styles, merges, widths, and
  frozen panes
