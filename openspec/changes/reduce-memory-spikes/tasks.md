# Tasks: Reduce Memory Spikes

Order follows the Migration Plan in design.md: Increment A (jobs) → B
(uploads) → C (reports). Each group is independently deployable.

## 1. Increment A — Scheduled jobs and contract listing loads

- [x] 1.1 Add `Worker.ScheduleEveryImmediateJitter(name, interval, maxJitter, job)` to `internal/jobs/worker.go`: randomized ctx-cancellable first-run delay (`select` on `stopCtx`/timer, `wg`-accounted), then the existing ticker loop; add unit tests in `internal/jobs/worker_test.go` proving (a) the first run fires after a delay, (b) `Shutdown` during the delay abandons the run without executing it, and `go test ./internal/jobs/` passes.

- [x] 1.2 Apply the jitter (90 s) in `scheduleJobs` (`cmd/api/main.go`) to the heavy jobs — `overdue_payments`, `credit_scores`, `release_unpaid_reservations`, `reservation_voucher_reminders`, `payment_reminder_emails`, `expire_overdue_seller_assignments`, `seller_score_recompute` — keep `analytics_cache_refresh` immediate, and extend the existing comment to state that first runs may now land up to 90 s late (deliberate; do not "fix"); verify with `go build ./...` and the `Incoming`/`Scheduled job` log lines on a local start.

- [x] 1.3 Add `FindOverdueInBatches(ctx, batchSize, fn)` to the `PaymentRepository` interface and `paymentRepository` (`internal/repository/contract_repository.go`) with the same filters/preloads as `FindOverdue`; add a sqlmock test asserting the batched SQL/preload shape and a live-`DATABASE_URL` test (skip-when-unset pattern from `analytics_refresh_test.go`) verifying batch boundaries return every matching row exactly once.

- [x] 1.4 Rework `CalculateOverdueInterest` and `CheckOverduePayments` (`internal/services/payment_service.go`) onto `FindOverdueInBatches`: flush `BatchUpsertInterest` + `BatchUpdateInterest` per batch instead of accumulating all entries, keep the notification dedupe map across batches, and keep the single admin summary; update the service mocks and verify `TestCalculateOverdueInterest_Logic` and `TestCheckOverduePayments_NotifiesOncePerUser` pass, plus a new test asserting one notification per user when the user's payments land in two different batches.

- [x] 1.5 Change `loadNextPayments` (`internal/repository/contract_repository.go:329`) to `SELECT DISTINCT ON (contract_id) payments.* ... ORDER BY contract_id, due_date ASC, id ASC`; add a sqlmock test asserting the emitted SQL ordering and a live-`DATABASE_URL` test with a contract holding multiple collectable installments asserting exactly one row is read and the earliest payment (due_date, then id) is attached; existing contract list tests keep passing.

- [x] 1.6 Update `docs/architecture/domain-flows.md` for the staggered startup and batched overdue scans, and run the increment-A gate: `gofmt` clean, `go build ./...`, `go test ./...`, `make lint`.

## 2. Increment B — Upload paths and request body limits

- [x] 2.1 Set `router.MaxMultipartMemory = 8 << 20` after `gin.New()` (`cmd/api/main.go`) and lower explicit `ParseMultipartForm` bounds: `media_handler.go:93` → `16 << 20`, `media_handler.go:202,269` → `8 << 20`, `contract_handler.go:280` → `8 << 20`; add `defer form.RemoveAll()` where a parsed form is no longer needed; verify with the existing media/contract handler tests (`go test ./internal/handlers/`).

- [x] 2.2 Switch `c.Request.FormFile` to Gin's `c.FormFile` in `payment_handler.go:275,329`, `branding_handler.go:83`, `user_handler.go:1052` so the engine bound applies to receipts, logo, and avatar; adjust the corresponding handler tests and verify `go test ./internal/handlers/` passes.

- [x] 2.3 Add `MAX_JSON_BODY_BYTES` to `internal/config` (`Config` struct + default `2097152`) and extend `internal/config/config_test.go` with default/override cases; verify `go test ./internal/config/`.

- [x] 2.4 Create `internal/middleware/body_limit.go` (`BodyLimit(maxBytes int64)`, wraps JSON-ish content types in `http.MaxBytesReader`, skips `multipart/form-data`) and register it on the `v1` group in `cmd/api/main.go` so it runs after the global gzip middleware (add a comment: must stay inner to gzip to bound decompressed bytes); middleware tests cover oversized JSON rejected, small JSON passes through, multipart untouched.

- [x] 2.5 Map `*http.MaxBytesError` to a Spanish **413** JSON response in `internal/handlers/binding.go` (`BindNestedOrFlat`) and `contract_handler.go:629`; add a handler test posting a > limit JSON body to a `BindNestedOrFlat` route asserting 413 and that no resource changed; verify `ShouldBindJSON` routes still return their existing 400 on oversized bodies with a test.

- [x] 2.6 Add the dimension gate to `image_service.ProcessAndSaveProfilePicture`: `image.DecodeConfig` first, reject if either side > 4096 px or total > 16M px with a Spanish error and no files written, then `Seek(0,0)` and proceed as today; test with a crafted small-but-large-IHDR image and a normal avatar image; verify `go test ./internal/services/` passes and the returned paths are unchanged for valid uploads.

- [x] 2.7 Update `docs/architecture/api-and-auth.md` (global JSON body limit, 413/400 behavior, `MAX_JSON_BODY_BYTES`) and `docs/guides/operations.md` (multipart memory bounds, new env var); run `make swagger` and confirm `git diff docs/` is empty or reflects only intended annotation changes; gate the increment: `gofmt`, `go build ./...`, `go test ./...`, `make lint`, `make security`.

## 3. Increment C — Report exports: scoped, batched, streamed

- [x] 3.1 Extend `paymentRepository.List` (`internal/repository/contract_repository.go:645`) with `project_id` (join contracts→lots) and `seller_id` (`contracts.seller_id = ? OR contracts.creator_id = ?`) filter keys, keeping the existing count-clone and `Select("payments.*")` behavior correct with the joins; add sqlmock tests for both filters and a live-`DATABASE_URL` test asserting out-of-scope rows never reach Go.

- [x] 3.2 Add a seller-scoped variant of `FindOverdueForActiveContractsDetailed` (same join/preloads, plus the seller predicate) to the `PaymentRepository` interface and implementation; sqlmock + live-DB tests; interface mocks in service tests gain the method.

- [x] 3.3 Rework `incomeFlowDetailSection` (`internal/services/income_flow_report.go`): pass `project_id`/`seller_id` through the query, delete `incomeFlowPaymentMatches` and the `filtered` copy, build rows directly per batch; verify existing `income_flow_report_test.go` expectations produce byte-identical sections (row order after SQL-side sorting preserved).

- [x] 3.4 Rework `BuildOverdueReport` (`internal/services/overdue_report.go`): use the scoped query, drop `overdueFilterPayments` and the `overdueDetailLine` model copies (compute bucket/days inline while streaming); verify `overdue_report_test.go` output is unchanged for the same fixture.

- [x] 3.5 Add the lean batched commissions contract stream (keyset pagination, batch 500, only the columns `GenerateCommissions` uses; no `TotalPaid` aggregate, no `loadNextPayments`) as a new repository method, and rework `ReportService.GenerateCommissions` (`internal/services/report_service.go:86`) and the overdue/income-flow builders to accumulate `ReportRow` cells per batch, releasing models between batches; add tests asserting identical item/row sets versus the old unbounded path for a fixture dataset and that per-batch callbacks bound peak row memory (row-count assertions suffice at unit level).

- [x] 3.6 Introduce `RenderReportTo(w io.Writer, doc, format, sheetName)` on `ReportExportService` and convert CSV (`csv.NewWriter(w)`) and PDF (`pdf.Output(w)`) renderers to writer targets (`report_render_csv.go`, `report_render_pdf.go`); tests render the same fixture document to buffers before/after and assert byte-identical CSV/PDF bytes; keep `RenderReport` as a thin `bytes.Buffer` wrapper for existing tests or migrate them.

- [x] 3.7 Convert `renderReportXLSX` + `writeReportDocumentToSheet` (`report_render_xlsx.go`, `xlsx_style.go`) to excelize `StreamWriter`: precompute styles, column-width pre-pass over `doc.Sections`, per-cell `excelize.Cell{StyleID, Value}` rows, `Flush()`, then apply `MergeCell`/`SetColWidth`/`SetRowHeight`/`SetPanes`, finally `file.Write(w)`; add an unzip-based golden test comparing sheet XML structure (merges, widths, panes, styles, cell values) against the current output; if parity cannot be met, fall back to design D5 (plain `File` + `file.Write(w)`) and record the decision here. Implementation note: file-level MergeCell/SetPanes are lost in stream mode, so all sheet properties go through the StreamWriter API during the pass (SetPanes pre-computed via reportDocumentFreezeRow); see design D5.

- [x] 3.8 Change `serveReportExport` (`internal/handlers/report_export_handler.go:307`) to: build document → set `Content-Disposition`/`Content-Type` → `c.Status(200)` → `RenderReportTo(c.Writer, ...)`; builder errors keep returning the Spanish JSON 500 before any bytes are written; handler tests assert unchanged headers and identical bytes for a stub document, and streaming starts only after a successful build.

- [x] 3.9 Update `docs/architecture/domain-flows.md` (report data flow: scoped/batched/streamed) and re-run the full gate: `gofmt`, `go build ./...`, `go test ./...` (plus the live-`DATABASE_URL` suite locally), `make lint`, `make security`.

## 4. Integration verification (spans all increments)

- [ ] 4.1 Deploy incrementally to Railway and verify on the platform: memory graph shows no >2× baseline spikes during a multi-photo gallery upload, an all-history commissions export, and a wide-range income-flow export; a redeploy shows staggered job starts in the logs and no exit-137 restarts; keep `GOMEMLIMIT=800MiB` (platform variable, outside repo) and watch GC/CPU for thrashing.
