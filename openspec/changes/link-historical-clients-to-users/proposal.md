## Why

Historical sales store client data but not a customer account, so historical
clients are invisible in the admin Users list and cannot be selected in the
reservation search. The API must link each historical sale to a real `user`,
reuse existing customers by identity, and backfill existing rows on deployment.

## What Changes

- Add `users.historical_client` and `historical_sales.client_user_id`.
- Resolve or create a customer when a historical sale is created, updated or
  imported:
  - identity first, then email;
  - reject staff identities;
  - no welcome email, random password, `must_change_password`, no RTN;
  - placeholder email when the record has no email.
- Expose `historical_client` on user responses and `client_user_id` on
  historical-sale responses.
- Filter `GET /users?historical=1` and add a `historical` count to
  `GET /users/stats`.
- Add an idempotent backfill command and run it from the Docker start command.
- Regenerate Swagger.

## Capabilities

### Modified Capabilities

- `historical-sales`: customer-account linkage and deployment backfill.
- `users`: historical flag, filter and stats.

## Impact

- Migration `000044_link_historical_clients_to_users`.
- `internal/models`, `internal/repository`, `internal/services`,
  `internal/handlers`, `cmd/api/main.go` (wiring), `cmd/backfill-historical-clients`.
- `Dockerfile`, `Makefile`, `docs/guides/operations.md`, Swagger.
- No change to reservations, lots or scores until a real contract is created.
