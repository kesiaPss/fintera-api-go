## 1. Data model

- [x] 1.1 Add migration `000044_link_historical_clients_to_users`
- [x] 1.2 Add `users.historical_client` and `historical_sales.client_user_id`

## 2. Linker

- [x] 2.1 Add `HistoricalClientService` with identity/email resolution, placeholder emails and staff conflict handling
- [x] 2.2 Link on create, update and CSV import
- [x] 2.3 Sync contact data on historical updates
- [x] 2.4 Add service tests for create, reuse, placeholder, staff conflict, update relink and backfill

## 3. Backfill and deployment

- [x] 3.1 Add `cmd/backfill-historical-clients` and the Makefile target
- [x] 3.2 Run the backfill in the Docker start command
- [x] 3.3 Document the deployment behavior

## 4. API surface

- [x] 4.1 Return `historical_client` on users and `client_user_id` on historical sales
- [x] 4.2 Add `GET /users?historical=1` and the `historical` stats count
- [x] 4.3 Add handler tests
- [x] 4.4 Regenerate Swagger

## 5. Verification

- [x] 5.1 Run `go build ./...`
- [x] 5.2 Run `go test ./...`
- [x] 5.3 Run `make swagger`
