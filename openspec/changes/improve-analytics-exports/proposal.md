## Why

The current analytics export (`GET /api/v1/analytics/export`) only prints four
detached metrics and a lot distribution, so it does not reflect the operations
of a month nor serve accounting. The commissions, overdue-payments, and revenue
reports are flat CSV dumps with no seller grouping, no aging buckets, no
accounting breakdown, and inconsistent styling. Accounting needs fiscal
identifiers (identity, RTN, bank reference), month-period summaries, ledger
movements by entry type, and a proper income flow. Every export must be
delivered in Spanish with a consistent visual style.

## What Changes

- Replace `GET /analytics/export` with `GET /reports/operations_export`
  supporting `format=xlsx|csv|pdf`, filters `project_id`, `start_date`,
  `end_date`. Content becomes a monthly operations summary: contracts
  approved/closed/cancelled, sales by financing type, collections by payment
  type, interest collected, projected vs collected effectiveness, ledger
  movements by entry type, outstanding portfolio, and lot distribution.
- Replace `GET /reports/commissions_csv` with `GET /reports/commissions_export`
  (`format=xlsx|csv|pdf`): sections grouped by seller with subtotals
  (estimated/paid/pending), rows visually distinguished as paid vs pending,
  pending credits (`seller_commission_credits`) and net pending per seller.
- Replace `GET /reports/overdue_payments_csv` with
  `GET /reports/overdue_export` (`format=xlsx|csv|pdf`): aging buckets 1–30,
  31–60, 61–90, 90+ with count/amount/interest/total summary, color-coded
  severity detail, contract balance, identity/RTN/phone. **BREAKING**: overdue
  payments of cancelled contracts are excluded.
- Deprecate `GET /reports/total_revenue_csv` in favor of a new
  `GET /reports/income_flow_export` (`format=xlsx|csv|pdf`) with a monthly
  accounting flow: actual collected by payment type vs projected, deviation,
  effectiveness %, per-project view, and payments/interest/refunds/adjustments
  breakdown.
- **BREAKING** (API): the three previously CSV-only report endpoints and the
  analytics export endpoint are removed; clients must call the new
  `*_export` endpoints with an explicit `format`.
- Introduce a shared XLSX style helper and reuse the existing PDF design
  system (`pdf_renderer.go`), so all four exports (XLSX, CSV, PDF) share the
  same palette, section headers, money/date formats, zebra rows, and titles.
- All exported files (column headers, section labels, sheet names, filenames
  metadata, statuses) are written in Spanish.

## Capabilities

### New Capabilities

- `monthly-operations-report`: monthly operations summary export with
  period totals, collections, ledger movements by entry type, portfolio
  balances, and lot distribution, oriented to accounting.
- `commissions-report`: commissions export grouped by seller, distinguishing
  paid vs pending commissions, with subtotals, credits, and net pending.
- `overdue-payments-report`: overdue payments export with 30/60/90+ aging
  buckets, summary totals, and fiscal/contact detail, excluding cancelled
  contracts.
- `income-flow-report`: monthly income-flow export with actual vs projected
  collections, effectiveness, per-project breakdown, and accounting movement
  detail.
- `report-export-formatting`: unified cross-format export rules (Spanish
  output, XLSX styling, PDF layout reuse, CSV consistency, money/date
  formatting, fiscal columns).

### Modified Capabilities

None (no archived capability specs currently exist for reports/exports).

## Impact

- Handlers: `internal/handlers/analytics_handler.go` (export removed),
  `internal/handlers/resource_handlers.go` (`ReportHandler` export methods).
- Services: `internal/services/export_service.go` (replaced),
  `internal/services/report_service.go` (new report builders + fixes), new
  `internal/services/xlsx_style.go`, `internal/services/operations_export.go`,
  `internal/services/income_flow_export.go`.
- Repositories: `internal/repository/analytics_repository.go` (monthly
  aggregations), `internal/repository/contract_repository.go`
  (`FindOverdueForActiveContracts` reused), commission/historical repositories
  for grouped data.
- Routes: `cmd/api/main.go` under the `sellerAdmin` group; Swagger annotations
  updated and `make swagger` regenerated.
- Docs: `docs/architecture/api-and-auth.md`, `docs/architecture/domain-flows.md`.
- Dependencies: none added (`excelize` and `gofpdf` already present).
- Data: no schema changes; reads existing `payments`, `contracts`,
  `contract_ledger_entries`, `historical_sales`, `seller_commission_credits`,
  `users` (identity, RTN), and payment bank-reference columns.
