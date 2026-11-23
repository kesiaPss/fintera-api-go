# Reduce Memory Spikes

## Why

The API is deployed on Railway with a 1 GB container limit, where memory spikes
from three sources risk OOM kills (exit 137) and restart loops, and restarts
themselves trigger a second spike because all scheduled jobs fire at once:

1. **Uploads**: multipart parsing buffers up to 64/32 MiB per request across
   media, contract-document, receipt, logo, and avatar endpoints, and profile
   images are decoded into uncompressed bitmaps without any pixel-dimension
   limit (a 10 MB PNG can become 100+ MB of RAM). JSON bodies are read with
   `io.ReadAll` and no global size cap.
2. **Report exports**: the commissions and overdue exports load **every**
   matching contract/payment with full association preloads (`PerPage = 0` and
   unscoped queries), filter in Go, copy slices, and then buffer the whole
   XLSX/CSV/PDF in memory before writing it to the response.
3. **Scheduled jobs**: all `ScheduleEveryImmediate` jobs run simultaneously on
   every start/deploy, and the overdue scans (`FindOverdue`) load all overdue
   payments plus per-contract next payments in single unbounded queries.

`GOMEMLIMIT` is set on Railway, but it is a soft limit: single large
allocations can still cross the container cap faster than the GC can react.
This change removes the spikes at the source.

## What Changes

- Lower multipart in-memory buffering to 8–16 MiB everywhere it is set, and
  make `router.MaxMultipartMemory` actually apply to the receipt, branding
  logo, and avatar handlers by switching `c.Request.FormFile` to Gin's
  `c.FormFile`.
- Add a global JSON request body limit middleware (default 2 MiB,
  configurable via `MAX_JSON_BODY_BYTES`) registered after the gzip middleware
  so the limit applies to decompressed bytes; oversized JSON bodies are
  rejected.
- Validate image dimensions (via `image.DecodeConfig`) before full decoding in
  the profile-picture service; reject images beyond a fixed pixel budget.
- Push report project/seller scoping into SQL (payment `List` gains
  `project_id`/`seller_id` filters; the overdue detailed query gains seller
  scoping) and remove the duplicate Go-side filtered slices in the income-flow
  and overdue report builders.
- Add batched report reads (`FindInBatches` / keyset pagination, batch 500)
  for the unbounded report paths, including a lean streaming query for the
  commissions export that skips `TotalPaid` and `NextPayment` loading.
- Stream report renders to the HTTP response: new
  `RenderReportTo(io.Writer, ...)`; CSV writes through `csv.Writer`, XLSX via
  excelize `StreamWriter` (with merges/widths/panes applied after flush), PDF
  via `pdf.Output(w)`. Export handlers set headers and stream instead of
  buffering `[]byte` + `c.Data`.
- Add `ScheduleEveryImmediateJitter` and use it to stagger the startup runs of
  the heavy jobs (0–90 s randomized delay, respecting shutdown context).
- Batch the overdue job scans (`FindOverdueInBatches`) and flush interest
  ledger/payment updates per batch instead of accumulating all entries.
- Change `loadNextPayments` to fetch at most one payment per contract using
  PostgreSQL `DISTINCT ON`.

No database migrations. No new routes. Exported file bytes stay identical in
content for CSV/PDF; XLSX must remain visually equivalent.

## Capabilities

### New Capabilities

- `http-request-body-limits`: global JSON body cap (with 413 rejection of
  oversized bodies) and bounded multipart in-memory buffering for every upload
  endpoint.
- `upload-media-safety`: image dimension validation before decoding in profile
  picture processing.
- `report-export-memory-efficiency`: SQL-side scoping, bounded batched reads,
  and streaming rendering for report exports.
- `background-job-memory-efficiency`: staggered startup job runs and batched
  overdue scans with bounded memory per batch.
- `contract-listing-next-payment`: next-collectable-payment attachment loads at
  most one payment per contract.

### Modified Capabilities

_None. `openspec/specs/` has no main specs yet; all behavior above is
introduced as new capability specs._

## Impact

- **Routes/wiring**: `cmd/api/main.go` (engine `MaxMultipartMemory`, body-limit
  middleware on the v1 group, `scheduleJobs` jitter), `internal/config` (new
  `MAX_JSON_BODY_BYTES`), new `internal/middleware` body-limit file.
- **Handlers**: `media_handler.go`, `contract_handler.go` (lower
  `ParseMultipartForm` values), `payment_handler.go`, `branding_handler.go`,
  `user_handler.go` (`c.FormFile` switch), `report_export_handler.go`
  (streaming serve), `binding.go` + `contract_handler.go:629` (413 mapping).
- **Services**: `image_service.go` (dimension gate), `report_export_service.go`
  (`RenderReportTo`), `income_flow_report.go`, `overdue_report.go`,
  `report_service.go` (`GenerateCommissions` batching),
  `report_render_csv.go` / `report_render_pdf.go` / `report_render_xlsx.go` /
  `xlsx_style.go` (streaming renderers), `payment_service.go` (batched
  interest/overdue jobs).
- **Repositories**: `contract_repository.go` (payment `List` filters,
  `FindOverdueInBatches`, scoped overdue detailed query, `DISTINCT ON`
  `loadNextPayments`), interfaces in `repository`.
- **Worker**: `internal/jobs/worker.go` (`ScheduleEveryImmediateJitter`).
- **Tests**: service mocks gain new repository methods; PostgreSQL-specific
  SQL (`DISTINCT ON`, `FindInBatches`) verified with live-`DATABASE_URL` tests
  in the existing skip-when-unset pattern.
- **Docs**: `docs/architecture/api-and-auth.md` (body limit / 413),
  `docs/architecture/domain-flows.md` (job stagger + batching),
  `docs/guides/operations.md` (multipart memory, `MAX_JSON_BODY_BYTES`).
- **Out of scope**: `ReportDocument` row buffering (residual memory ceiling),
  true two-pass-free streaming, moving jobs to a dedicated worker service.
