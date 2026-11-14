## 1. Foundations

- [x] 1.1 Create `internal/services/report_labels.go` with Spanish label maps for statuses, payment types, financing types, origins, aging buckets, and section titles; add unit tests asserting every constant has a Spanish label (`go test ./internal/services -run ReportLabels`).
- [x] 1.2 Create `internal/services/xlsx_style.go` with the shared builder (title banner, section title, table header, zebra/subtotal/total rows, money number formats for HNL/USD, dates, status colors, column widths, freeze panes); add tests asserting generated style IDs and number formats (`go test ./internal/services -run XLSXStyle`).
- [x] 1.3 Define the shared report document types (metadata, ordered sections, row kinds, totals) used by all reports in `internal/services/report_export_types.go`; verify `go build ./...` succeeds.
- [x] 1.4 Add a currency/date formatting helper reusing `formatMoneyCurrency` semantics, with CSV-safe numeric variants; add table-driven tests (`go test ./internal/services -run Format`).

## 2. Data aggregation

- [x] 2.1 Add analytics repository methods for period contract counts/values by financing type and status, collections by payment type, total interest collected, projected due, and ledger sums by entry type, with `project_id`/date filters; add repository tests with sqlmock or the project's existing pattern.
- [x] 2.2 Add repository/service support for overdue detail enrichment (`Contract.Balance`, payment type) and switch the overdue source to `FindOverdueForActiveContracts`, excluding cancelled contracts; add a test proving a cancelled contract's overdue payment is excluded (`go test ./internal/services -run Overdue`).
- [x] 2.3 Implement `internal/services/operations_report.go` building the monthly operations document (operations summary, collections/effectiveness, ledger movements by entry type, portfolio, lot distribution) with totals; add unit tests for empty period, effectiveness math, and ledger sign sums.
- [x] 2.4 Implement commissions report builder that groups `GenerateCommissions` items by seller with estimated/paid/pending subtotals, pending credits and net pending per seller, preserving seller scoping; add tests for grouping, grand totals, and credits (`go test ./internal/services -run CommissionsReport`).
- [x] 2.5 Implement overdue report builder with inclusive buckets (`<=30`, `<=60`, `<=90`, `>90`, minimum 1 day), bucket summary totals and detail lines; add boundary tests for 1, 30, 31, 60, 61, 90, 91 days (`go test ./internal/services -run OverdueBuckets`).
- [x] 2.6 Implement `internal/services/income_flow_report.go` with monthly actual vs projected rows, deviation and effectiveness, collections by payment type per month, per-project totals, and ledger movement breakdown, reusing `GetRevenueTrend`; add tests including a month with no projections (no division by zero).
- [x] 2.7 Add client fiscal data joins (identity, RTN) and bank reference fields (authorization number, payer account name) to all detail builders; add tests asserting empty values render as empty and are not masked (`go test ./internal/services -run Fiscal`).

## 3. Renderers

- [x] 3.1 Implement the XLSX renderer for all four report documents using `xlsx_style.go`: Spanish sheet names, title, sections, headers, number/date formats, freeze panes, and severity/status colors; add tests that open the generated workbook and assert sheet names, headers, and a money cell format.
- [x] 3.2 Implement the CSV renderer for all four report documents using `encoding/csv`: Spanish headers, identical sections/totals, numeric values without currency symbols, and a currency column where relevant; add tests asserting headers and totals parity with the builder.
- [x] 3.3 Implement the PDF renderer for all four report documents on `reportPDFRenderer` (title banner, section titles, tables, status badges, footer page numbers); remove the ad hoc gofpdf path from `export_service.go`; add tests that generate bytes and assert a non-empty PDF with expected title text.
- [x] 3.4 Delete or repurpose `internal/services/export_service.go` so no report renders outside the shared pipeline; verify `go build ./...` and `go test ./...` succeed.

## 4. HTTP layer and routes

- [x] 4.1 Add thin handlers and a shared `format` validator in `internal/handlers/resource_handlers.go` for `operations_export`, `commissions_export`, `overdue_export`, and `income_flow_export`, with Spanish error messages and Swagger annotations (`@Produce application/octet-stream`/`text/csv` per format parameter, `@Router` paths).
- [x] 4.2 Register the four routes in `cmd/api/main.go` inside the `sellerAdmin` group and remove `/analytics/export`, `/reports/commissions_csv`, `/reports/overdue_payments_csv`, and `/reports/total_revenue_csv`; verify `go build ./...` and route listing tests (if present) pass.
- [x] 4.3 Implement Spanish filenames containing report name and period (`reporte_operaciones_2026-10.xlsx`, `reporte_comisiones_...`, etc.) and correct content types; add handler tests asserting filename and `Content-Type` per format.
- [x] 4.4 Add handler authorization tests proving sellers only receive their own data and invalid `format` values return 400 with a Spanish message.

## 5. Verification and docs

- [x] 5.1 Run `make fmt`, `go build ./...`, `go test ./...`, and `make lint`; fix all failures.
- [x] 5.2 Run `make swagger` and confirm `docs/swagger.{yaml,json,go}` contain the four new routes and none of the removed ones.
- [x] 5.3 Update `docs/architecture/api-and-auth.md` with the new route table and the old-to-new endpoint mapping, and `docs/architecture/domain-flows.md` with the four report descriptions and accounting scope.
- [ ] 5.4 Smoke-test each endpoint manually (xlsx/csv/pdf) against a local database and confirm Spanish headers, sections, and totals; record the result in the change notes.
