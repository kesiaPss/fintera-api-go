## Context

Historical sales are migrated rows with free-text client data. `GET /users` and
the reservation search both query `users`, so without a linked account the same
person cannot be reused for a new sale and does not appear in admin.

## Decisions

### Link, do not duplicate

- `historical_sales.client_user_id` references `users`.
- `users.historical_client` identifies accounts created from the migration.
- Resolution order: identity, then email. Staff identities return a conflict.
- Existing customer accounts are reused and are not marked as historical.

### Placeholder accounts

- No welcome email is sent.
- Random password, `must_change_password = true`, `email_notifications = false`,
  and `RTN` is inserted as NULL.
- Missing email uses `historico+<normalized identity>@fintera.local`.
- A taken email from another identity is not stolen; a placeholder is used.
- Two identities that normalize to the same placeholder get a unique suffix.

### Backfill

- `ListWithoutClientUser(afterID, limit)` processes in id order.
- `HistoricalClientService.Backfill` is idempotent and reports linked, skipped
  and error counts.
- The Docker command runs the binary after migrations and seeds. Per-record
  conflicts are logged and retried on the next deployment.

### API surface

- `UserResponse.historical_client`.
- `HistoricalSaleResponse.client_user_id`.
- `GET /users?historical=1`.
- `GET /users/stats` returns `historical` separately; it is a subset of `user`
  and is not added to `total`.

## Risks

- Placeholder accounts increase the Users table; the flag and filter isolate
  them.
- A record whose identity belongs to staff stays unlinked and is reported.
- Rejected historical records keep the linked account; admins can deactivate it.
- The backfill runs on every start but only queries unlinked rows.
