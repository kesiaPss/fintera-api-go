## Why

Historical sales store client data but not a customer account, so historical
clients are invisible in the admin Users list and cannot be selected in the
reservation search. The API must link each historical sale to a real `user`,
reuse existing customers by identity, and backfill existing rows at API startup.

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
- Add an idempotent, versioned backfill ledger (`backfill_runs`) and a generic
  runner. The API runs pending tasks at startup; `cmd/backfill` lists and
  re-runs them manually.
- Regenerate Swagger.

## Capabilities

### New Capabilities

- `backfills`: a registry, execution ledger and CLI for one-off data tasks.

### Modified Capabilities

- `historical-sales`: customer-account linkage and startup backfill.
- `users`: historical flag, filter and stats.

## Impact

- Migrations `000044_link_historical_clients_to_users` and `000045_backfill_runs`.
- `internal/models`, `internal/repository`, `internal/services`,
  `internal/handlers`, `internal/backfills`, `cmd/api/main.go` (wiring) and
  `cmd/backfill`.
- `Makefile`, `docs/guides/operations.md`, Swagger.
- No change to reservations, lots or scores until a real contract is created.
