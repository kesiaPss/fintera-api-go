## Context

See `proposal.md` for motivation and `specs/` for required behavior. Relevant
current state:

- `internal/services/export_service.go` renders the analytics export ad hoc
  (single-style XLSX, plain gofpdf PDF, string-formatted CSV).
- `internal/services/report_service.go` already builds commissions, revenue,
  and overdue data and owns `formatMoneyCurrency`; its CSV writers are flat
  dumps.
- `internal/services/pdf_renderer.go` provides the shared PDF design system
  (palette, title banner, section titles, table primitives, badges, footer)
  used by the existing contract/balance PDFs.
- `internal/services/analytics_service.go` exposes overview/distribution and a
  cached `revenue_trend` (actual vs projected by month) currently never
  exported.
- `internal/repository/contract_repository.go` has `FindOverdue` (includes
  cancelled contracts) and `FindOverdueForActiveContracts`.
- Payment models carry `authorization_number` and `payer_account_name`; users
  carry `identity` and `rtn`.
- No XLSX style helper exists and there is no schema migration involved.

## Goals / Non-Goals

**Goals:**

- One report-building layer per report that produces a neutral data structure,
  rendered by three format-specific renderers (XLSX, CSV, PDF) with identical
  sections and totals.
- A shared XLSX style helper that mirrors the existing PDF palette and layout
  primitives.
- Spanish labels centralized in one place, used by every renderer.
- Seller scope and fiscal columns handled consistently across reports.

**Non-Goals:**

- No schema changes, no new dependencies, no new background jobs.
- No change to the analytics JSON endpoints (`overview`, `distribution`,
  `performance`, `sellers`).
- No scheduled/emailed report delivery.
- No IVA/tax computation (not modelled).

## Decisions

### Report model + three renderers

Each report is built once into a `ReportDocument`-style value: metadata
(title, period, scope), ordered sections, each section with a kind (summary
table, grouped table, detail table, stat grid) and typed rows. A small
dispatcher selects the renderer by `format`.

Rationale: guarantees CSV/XLSX/PDF stay in sync and makes totals testable
without rendering. Alternative (three independent writers per report) was
rejected because it duplicates aggregation logic and drifts.

### Shared XLSX style helper

New `internal/services/xlsx_style.go` exposes a builder: title banner, section
title, table header, body/zebra rows, subtotal/total rows, money cells with
`#,##0.00`, currency-aware number format (`L`/`$`), date format, status
colors (paid green, pending amber, overdue red/severity), column widths, freeze
pane, and sheet naming in Spanish.

Rationale: mirrors `pdf_renderer.go` so both formats share one palette and one
mental model. Alternative (per-report styling inline) was rejected for
consistency and maintenance.

### PDF reuse instead of gofpdf ad hoc

All report PDFs use `reportPDFRenderer` (`titleBanner`, `sectionTitle`,
`tableHeader`, `tableRow`, `drawFooter`). The current analytics PDF built
directly with `gofpdf` is deleted.

### Aging buckets computed in service, not SQL

Overdue buckets (1–30, 31–60, 61–90, 90+) are computed from
`Payment.OverdueDays()` in the service after fetching active-contract overdue
payments. Rationale: the formula already exists in the model, avoids duplicated
date math across SQL dialects, and keeps bucket boundaries (inclusive)
testable. Volume is bounded by overdue payments, not history.

Note: `OverdueDays()` uses whole 24-hour periods from `time.Now()`. Boundaries
are defined as `days <= 30`, `<= 60`, `<= 90`, `> 90`, with `days` clamped to a
minimum of 1 for payments already past due.

### Data sources and new repository queries

- Operations report: new aggregation methods on the analytics repository for
  contract counts/values by financing type and by status within the period,
  collections by payment type, interest collected, projected due, and ledger
  sums by entry type; portfolio balance reuses the existing contract balance
  cache and ledger aggregate. Lot distribution reuses existing distribution
  data.
- Commissions: reuse `GenerateCommissions` item assembly (contracts +
  historical sales + scores), add grouping-by-seller and credits lookup; do
  not reimplement commission calculation.
- Overdue: reuse `FindOverdueForActiveContracts`; add `Contract.Balance` and
  payment type to the detail.
- Income flow: reuse `GetRevenueTrend` for monthly actual/projected; add
  collections-by-type per month, per-project totals, and ledger movement
  breakdown for the period.

### Fiscal data is read from raw models, not response DTOs

`ContractResponse.ToResponse()` and payment DTOs mask identities. Exports read
the raw `User.Identity`, `User.RTN`, and payment bank-reference fields,
because exports are restricted to admins and to the owning seller (existing
`sellerAdmin` group plus seller scoping). Masked DTOs are not used for export
rendering.

### Spanish labels centralized

A single `report_labels.go` (or equivalent map block in the report package)
holds translations for statuses, payment types, financing types, origins,
bucket names, and section titles, including accented Spanish. All renderers
consume it.

### Endpoint and route replacement

Old routes are removed:
`/analytics/export`, `/reports/commissions_csv`,
`/reports/overdue_payments_csv`, `/reports/total_revenue_csv`. New routes are
registered in the same `sellerAdmin` group with a shared `format` validation
helper (`xlsx` default is **not** applied; `format` is required to keep the
contract explicit). Handlers stay thin: parse filters, call the report
service, stream the bytes with the correct content type and Spanish filename
(e.g. `reporte_operaciones_2026-10.xlsx`).

Rationale: one route per report with a format switch avoids four near-identical
handlers and keeps Swagger readable. Alternative (separate
`_csv`/`_xlsx`/`_pdf` suffixes) was rejected as route bloat.

### CSV remains an unstyled data format

CSV emits the same sections with Spanish headers, numeric money values without
currency symbols, and a currency column where relevant. It is generated with
`encoding/csv` and streamed where practical.

## Risks / Trade-offs

- **Large exports (memory)** → Excelize holds the workbook in memory and PDF
  rendering accumulates pages. Mitigation: keep detail sections filtered by
  period, stream CSV, and document practical limits; no cross-replica impact.
- **Removing existing endpoints breaks clients** → Coordinate the frontend
  release; document the mapping in `docs/architecture/api-and-auth.md`; keep
  the same `sellerAdmin` authorization so behavior is otherwise stable.
- **Bucket boundary ambiguity with `time.Now()`** → Define inclusive bounds
  (`<=30`, `<=60`, `<=90`, `>90`), clamp past-due minimum to 1, and lock with
  unit tests using fixed reference dates.
- **Fiscal data exposure** → Identity/RTN are unmasked in exports; restrict by
  existing role middleware and seller scoping, and assert in handler tests
  that sellers cannot export other sellers' data.
- **Label drift between formats** → All labels come from one map; renderer
  tests assert a Spanish header set per report.
- **Totals divergence** → Report builders return totals; renderers only print
  them. Tests assert summary totals equal the sum of detail rows.

## Migration Plan

1. Implement report builders, style helper, renderers, handlers, and routes.
2. Register new routes and remove the old ones in the same change; regenerate
   Swagger.
3. Update `docs/architecture/api-and-auth.md` and `domain-flows.md`, including
   the endpoint mapping for clients.
4. Deploy. No database migration and no data backfill.
5. Rollback: redeploy the previous build. Old endpoints return with the
   previous behavior; no persisted state is affected.
