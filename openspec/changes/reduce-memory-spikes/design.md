# Design: Reduce Memory Spikes

## Context

See `proposal.md` — Why for motivation. The constraints that shape this
approach:

- Railway container with a 1 GB RAM cap; `GOMEMLIMIT=800MiB` is set in the
  platform dashboard (outside the repo) and is a soft limit only.
- Production deployment runs every `ScheduleEveryImmediate` job once at each
  start (`cmd/api/main.go:540`), so restarts are themselves spike events.
- Gin's `router.MaxMultipartMemory` only affects `c.FormFile`/`c.MultipartForm`;
  `c.Request.FormFile` uses net/http's own 32 MiB default, and explicit
  `ParseMultipartForm(N)` calls bypass the engine setting entirely. All three
  patterns exist today.
- The gzip middleware (`main.go:215`) decompresses request bodies before inner
  middleware sees them.
- Report builders currently amplify memory ~4–5×: unbounded `List` with
  preloads → Go-side filter copy → row-cell copy → full file buffered in a
  `[]byte` → copied again by `c.Data`.
- Repository tests run against `go-sqlmock` (postgres dialector), which matches
  SQL strings but does not execute them. Postgres-specific semantics
  (`DISTINCT ON`, `FindInBatches` + preloads) can only be verified against a
  live `DATABASE_URL`, following the existing skip pattern in
  `repository/analytics_refresh_test.go`.

## Goals / Non-Goals

**Goals:**

- Keep peak per-request/per-job working memory within a fixed envelope
  independent of dataset size.
- Zero visible change to response bodies, file contents (CSV/PDF byte-identical,
  XLSX visually equivalent), default periods, filters, or job semantics.
- Only reject requests that are pathological (bodies > 2 MiB JSON, images
  > 16 MP) or already capped (multipart bodies keep their 500/200 MB limits).

**Non-Goals:**

- Streaming *document building*: `ReportDocument.Sections[].Rows` still holds
  all rendered rows (needed for column-width passes). This is a documented
  residual ceiling, not solved here.
- True two-pass-free XLSX streaming, or fixed-width columns.
- Moving jobs to a dedicated worker service or external scheduler (AGENTS.md
  lists this as the future path at higher scale).
- Changing upload semantics, per-file size limits, or accepted MIME types.
- Adding pprof/metrics endpoints (separate follow-up).

## Decisions

### D1. Multipart memory: engine setting + explicit lower args + `c.FormFile`

- `router.MaxMultipartMemory = 8 << 20` in `main.go`.
- Rewrite the three `c.Request.FormFile` call sites (`payment_handler.go:275,
  329`, `branding_handler.go:83`, `user_handler.go:1052`) to Gin's
  `c.FormFile` so the engine bound actually applies. *Alternative considered*:
  pre-`ParseMultipartForm(8<<20)` in each handler — rejected as more code with
  the same result.
- Lower the explicit `ParseMultipartForm` args: drawings `64<<20 → 16<<20`
  (single files up to 50 MB benefit from a bigger in-memory first chunk),
  photos and contract documents `32<<20 → 8<<20`.
- Add `defer form.RemoveAll()` where the parsed form is no longer needed so
  multipart temp files are removed deterministically instead of waiting on the
  runtime finalizer.

### D2. JSON body limit as middleware registered on the v1 group (not globally)

New `middleware.BodyLimit(maxBytes)`: for every non-multipart request with a
body, wrap `c.Request.Body` in `http.MaxBytesReader`; multipart is skipped
because upload endpoints own their much larger per-endpoint caps. Registered
on the `v1` group so it executes innermost — any global middleware that wraps
the body runs first and the cap then applies to the bytes handlers actually
read (correct placement if request decompression is ever enabled; today the
gzip middleware does not decompress requests).

Status-code semantics: the two sites that `io.ReadAll` the body
(`handlers/binding.go`, `contract_handler.go:629`) detect
`*http.MaxBytesError` and return **413** with a Spanish error; `ShouldBindJSON`
routes surface a **400** binding error. Uniform 413 everywhere would require a
shared bind helper across all handlers — deliberately out of scope (the
protective behavior is identical).

Config: `MAX_JSON_BODY_BYTES` in `internal/config` (default `2097152`),
passed into the middleware factory.

### D3. Dimension gate via `image.DecodeConfig` before full decode

Originally planned for `image_service.ProcessAndSaveProfilePicture` only, on
the assumption that other media paths store bytes verbatim. The rebase onto
main revealed the WebP transcoding pipeline (PR #152) now decodes *every*
raster upload (`decodeRaster` → full RGBA) before resizing, so a pixel bomb
in any raster upload expands memory regardless of file size. Post-rebase
implementation: the gate reads the header config from the in-memory `data`
bytes (`image.DecodeConfig(bytes.NewReader(data))`) and enforces `max
4096 px per side` and `max 16M px total` before `ProfileToWebP` decodes the
full image. Extending the same gate to gallery/drawing/receipt uploads was
deliberately left out of this PR (it changes user-visible acceptance limits
that the WebP PR just set; a follow-up should decide the budgets per kind).

### D4. Report data path: SQL scoping first, then bounded reads

- Extend `ListQuery` filter keys handled by `paymentRepository.List` with
  `project_id` (join contracts → lots) and `seller_id` (contracts.seller_id =
  $1 OR contracts.creator_id = $1), mirroring the semantics of the Go-side
  `incomeFlowPaymentMatches` being removed. The existing count clone and
  `Select("payments.*")` pattern are extended accordingly.
- Give the overdue detailed query the same seller scoping variant (the overdue
  export offers seller scope only — no date/project params — so only
  `seller_id` is pushed down).
- Add callback-based batched reads (`~500` rows) for the three unbounded paths:
  commissions contracts (lean select: the columns `GenerateCommissions`
  actually uses; skips the `TotalPaid` aggregate and `loadNextPayments` extra
  queries), income-flow paid payments, overdue detailed payments.
- Prefer **keyset pagination** (`WHERE payments.id > ? ORDER BY payments.id
  LIMIT 500` with the same joins/preloads) over `FindInBatches` for these
  streams: explicit, stable, and its generated SQL is inspectable under
  sqlmock; `FindInBatches` remains acceptable for `CalculateOverdueInterest`
  where GORM's batching matches the existing single-query call shape.
  *Trade-off*: keyset requires the caller to drive the loop; `FindInBatches`
  hides it but interleaves `Count` and PK-ordering behavior we must not
  regress. Either way, per-batch preloads are mandatory so association cost is
  bounded too.
- Delete the Go-side copies (`filtered := make([]models.Payment, ...)`,
  `overdueDetailLine` model copies) — batch rows are converted directly into
  `ReportRow` cells and the models dropped when the batch ends.

### D5. Report rendering: `RenderReportTo(w io.Writer, ...)`

- CSV: `csv.NewWriter(w)` directly.
- PDF: gofpdf still buffers internally, but we remove the `bytes.Buffer` and
  the `c.Data` copy by calling `pdf.Output(w)`.
- XLSX: excelize `StreamWriter` with per-cell `excelize.Cell{StyleID, Value}`.
  Implementation finding (3.7): in stream mode, sheet-level operations called
  on `*excelize.File` (MergeCell, SetPanes, SetColWidth, SetRowHeight) are
  silently lost, because the stream serializes its own worksheet state at
  Flush. All properties therefore go through the StreamWriter API:
  `sw.MergeCell` during the row pass, `RowOpts{Height}` per row,
  `sw.SetColWidth` in a pre-pass (widths measured over `doc.Sections`, which
  are already in memory), and `sw.SetPanes` — which excelize requires before
  the first `SetRow`, so the freeze row is pre-computed from the document
  layout by `reportDocumentFreezeRow`, a deliberate mirror of the row
  counting in `renderDocument`, pinned by `TestRenderReportXLSXStreamStructure`
  against drift.
  *Alternative*: keep `excelize.File` and only replace
  `WriteToBuffer()+c.Data` with `file.Write(w)` — retained as the fallback if
  StreamWriter parity proves too costly; it still removes two full copies.
- PDF byte-parity caveat (3.6): gofpdf is not deterministic across calls
  (internal font-object ordering and the creation timestamp vary), so
  streamed-vs-buffered identity is structural (`%PDF` prefix, size, parsing)
  rather than byte-level; the streaming wrapper is the sole code path, so
  content equivalence holds by construction.
- Handler `serveReportExport`: build the document first (errors → existing JSON
  500), then set headers + `c.Status(200)`, then render into `c.Writer`.
  Mid-render errors after streaming starts can only be logged; the connection
  is left broken (acceptable: the failure modes are builder-side, which happen
  before headers).

### D6. Jobs: jittered first runs + batched overdue processing

- New `Worker.ScheduleEveryImmediateJitter(name, interval, maxJitter, job)`:
  first run after `rand.Int63n(maxJitter)` via a `select` on `stopCtx` and the
  timer (cancel-safe, `wg`-accounted), then the existing ticker loop. Applied
  in `scheduleJobs` to the heavy jobs with `maxJitter = 90s`; the analytics
  cache refresh keeps an immediate first run because it serves cached reads.
  *Trade-off*: after a deploy, heavy jobs now land up to 90 s late — acceptable
  versus the current same-second pileup and the original "must run soon after
  restart" intent; this must be written into the code comment so it is not
  "fixed" back later.
- `FindOverdue` gains a batched sibling used by `CalculateOverdueInterest` and
  `CheckOverduePayments`; the interest job flushes `BatchUpsertInterest` and
  `BatchUpdateInterest` per batch instead of accumulating one giant slice +
  map. Notification dedupe (`map[uint]struct{}`) persists across batches so
  the "one notification per user per run" semantics hold.
- The daily reminder emails keep their existing flat `reminderRow` projection
  queries (already bounded by join, no preloads) — unchanged.

### D7. `loadNextPayments` via `DISTINCT ON`

`SELECT DISTINCT ON (contract_id) payments.* ... WHERE contract_id IN (...)
AND status IN ('pending','submitted') AND (document_path IS NULL OR
document_path = '') ORDER BY contract_id, due_date ASC, id ASC` — the Postgres
rule that leftmost `ORDER BY` columns match the `DISTINCT ON` expression is
satisfied by construction. This replaces a full association scan on **paginated**
`List` pages; the unbounded commissions case is already removed by D4's lean
stream, so the `IN` list stays bounded by page size (prepared-statement cache
growth is limited to a few arities).

### D8. Test strategy split

- Unit behavior (413/400 mapping, dimension gate, streaming renderers writing
  to `bytes.Buffer` in tests, batch flush counts, dedupe across batches):
  service-level tests with existing testify mocks; repository-interface mocks
  gain the new batched/scoped methods.
- Postgres-specific SQL (`DISTINCT ON`, keyset/`FindInBatches` streams,
  scoped `paymentRepository.List` joins): live-DB tests guarded by
  `DATABASE_URL` + `t.Skip`, following `analytics_refresh_test.go`, covering
  multi-installment selection, batch boundaries, and scope pushdown.
- XLSX parity: golden checks on unzip-level structure (sheet XML merges/widths/
  styles present, cell values) for a fixed fixture document, before/after.

## Risks / Trade-offs

- **[StreamWriter XLSX regressions] →** fallback D5 path (plain `File` writing
  to `w`) keeps most of the win; parity test gates the switch.
- **[Mid-stream render errors can't return JSON] →** all document building and
  validation happens before the first header; render errors are logged and the
  truncated response fails client-side loudly.
- **[Legitimate JSON bodies > 2 MiB rejected] →** the largest payloads
  (contract creation with long schedules, CSV *multipart* imports) are either
  under 2 MiB or exempt (multipart skip); `MAX_JSON_BODY_BYTES` is the escape
  hatch and requires only a Railway variable change.
- **[Keyset/FindInBatches + Preload behavior differs from one-shot `List`] →**
  live-DB tests pin row-count and ordering semantics; service tests pin the
  rendered output against the old implementation's expectations.
- **[Jitter delays post-deploy job runs] →** bounded at 90 s; documented in the
  code comment and `domain-flows.md`.
- **[DISTINCT ON arity churn in the prepared-statement cache] →** IN-list size
  is now always page-bounded (≤ 100); negligible.
- **[Residual `ReportDocument` row buffering] →** explicitly accepted
  (Non-Goal); it is roughly 10× smaller than the preloaded models it replaces.
- **[Behavioral break for abusive clients] →** oversized JSON now 400/413;
  acceptable and documented in `api-and-auth.md`.

## Migration Plan

1. Land as three independently deployable increments (one PR each), in order:
   **A** jobs (D6, D7) → **B** uploads (D1, D2, D3) → **C** reports (D4, D5).
2. No migrations, no new routes, no data changes; each increment is reverted
   by redeploying the previous image.
3. Deploy on a low-traffic window; after each increment, watch Railway memory
   and CPU plus `Scheduled job failed` / `Incoming request` logs for one
   scheduled cycle (24 h covers the daily jobs).
4. Verify with a load smoke per increment: one gallery upload batch, one
   commissions export with no dates, one wide-range income flow export, one
   simulated restart (deploy) to observe staggered job starts.

## Open Questions

- Exact batch size (500 assumed): tunable constant in the repository layer,
  to be confirmed against production row counts after increment C.
- Whether drawings merit a higher multipart bound than 16 MiB (50 MB PDF/SVG
  plans): start at 16 MiB, measure.
