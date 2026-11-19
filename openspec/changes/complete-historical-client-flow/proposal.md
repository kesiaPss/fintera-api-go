## Why

Sellers with a per-seller commission exception still see the global default rate
when they register a historical sale, because the rules endpoint only exposes
global values and the seller UI sends that default back. The historical record
also cannot be loaded for editing, so a wrong rate or amount cannot be corrected
before an admin validates it.

The UI change `complete-historical-client-flow` needs the API to expose the
seller's effective historical rate and a single-record read endpoint.

## What Changes

- `GET /api/v1/commissions/rules` SHALL include `seller_rate` for sellers,
  resolved as: per-seller exception → global default. Admins keep the global
  response without the field.
- Add `GET /api/v1/historical_sales/:id` for the edit form. Sellers can read
  their own records; admins can read any record.
- Keep `PUT /api/v1/historical_sales/:id` limited to pending records for sellers
  and preserve the stored rate when a partial update omits it.
- Add tests and regenerate Swagger.

## Capabilities

### Modified Capabilities

- `commissions`: the seller-facing rules endpoint exposes the effective
  historical rate.
- `historical-sales`: single-record read and rate-preserving updates.

## Impact

- `internal/models/commission_rate.go`
- `internal/handlers/commission_handler.go`
- `internal/services/historical_sale_service.go`
- `internal/handlers/historical_sale_handler.go`
- `cmd/api/main.go`
- `internal/services/historical_sale_service_test.go`
- `docs/swagger.{json,yaml}` and `docs/docs.go`
- No database migration.
