## 1. Commission rules

- [x] 1.1 Add `seller_rate` to `models.CommissionRulesResponse`
- [x] 1.2 Resolve and attach the seller rate in `CommissionHandler.ShowRules` for non-admin actors
- [x] 1.3 Cover the optional field and admin omission in tests

## 2. Historical sale read

- [x] 2.1 Add `HistoricalSaleService.Get` with seller-own/admin-any authorization
- [x] 2.2 Add `HistoricalSaleHandler.Show` and `GET /api/v1/historical_sales/:id`
- [x] 2.3 Add service tests for own, forbidden and not-found reads

## 3. Rate-preserving update

- [x] 3.1 Keep the stored rate when `rate` is omitted from the update payload
- [x] 3.2 Add a service test for an explicit amount without a rate
- [x] 3.3 Include `tasa` and `comision` in the CSV import template

## 4. Verification

- [x] 4.1 Run `go test ./internal/services ./internal/handlers ./internal/models`
- [x] 4.2 Run `make swagger` and commit the generated docs
- [x] 4.3 Run `go build ./...`
- [x] 4.4 Run the full `go test ./...` suite
