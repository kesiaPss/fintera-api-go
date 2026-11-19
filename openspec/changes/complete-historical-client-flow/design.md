## Context

`CommissionService.ResolveHistoricalRate` already resolves the seller exception
and global default, but it is only used when a historical sale is created without
an explicit rate. The seller UI always sends the rate it sees, and the rules
endpoint only returns global values, so the exception is bypassed.

Historical sales can already be updated through `PUT`, but there is no single
record read and the update clears `rate` when the payload omits it.

## Decisions

### Seller rate in the rules response

- Add `seller_rate *float64` with `omitempty` to `CommissionRulesResponse`.
- In `CommissionHandler.ShowRules`, when the actor is not an admin, resolve the
  historical rate for the actor and attach it. Admin responses keep the global
  values unchanged.
- The UI treats `seller_rate` as the preloaded value and sends its (possibly
  edited) value back on create/update.

### Single-record read

- Add `HistoricalSaleService.Get(ctx, id, actorID, isAdmin)`.
- Reuse `FindByIDWithDetails` so `creator_name` and `validator` data are present.
- Sellers can read only records they created; admins can read any record.
- Expose it as `GET /api/v1/historical_sales/:id` in the seller/admin group.

### Rate-preserving updates

- `HistoricalSaleService.Update` only replaces `sale.Rate` when the input
  contains a rate. An explicit amount without a rate therefore keeps the existing
  rate instead of clearing it.

## Risks

- The rules response changes shape only by adding an optional field, so existing
  clients are unaffected.
- The new read endpoint uses the same seller/admin authorization as the list and
  update paths.
- No schema changes; rollback is a code revert and Swagger regeneration.
