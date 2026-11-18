# Design

## Public seller picker

`GET /public/sellers` is part of the existing rate-limited public catalog group
(60 requests per minute per IP). It returns active sellers only, with `id`,
`full_name`, `phone` and `email`. Score, sales count, address and internal
identifiers stay private.

## Reassignment

`POST /contracts/{guid}/assignments` is admin-only.

Request:

```json
{
  "mode": "seller" | "automatic",
  "seller_id": 12,
  "accept": true,
  "exclude_current": true
}
```

Rules:

- `mode=seller` validates the seller is active.
- `mode=automatic` runs the existing fairness selection and can exclude the
  current seller.
- `accept=true` records an accepted assignment immediately. Open contracts can
  also receive a normal pending offer with the configured acceptance window.
- Contracts that are not `pending` or `submitted` are always assigned directly
  so the commission does not depend on a late acceptance.
- Any existing pending offer is marked `superseded` before the new offer. The
  new status is ignored by the score window and excluded from future automatic
  selection for the same contract.
- If `commission_paid_at` is set and the seller changes, a pending
  `seller_commission_credits` row is created for the previous seller and the
  paid markers are cleared. The new seller's commission is recalculated with
  the existing `CommissionService`.
- Every change writes an audit entry with the admin actor.

## Reporting

`GenerateCommissions` no longer falls back to the customer's name for
self-service reservations. The creator is only used when it is a seller or an
admin.

## Risks

- Reassignment is not wrapped in a single database transaction. The unique
  pending-offer index still prevents two open offers; a failed commission
  update can leave a credit and a new assignment, which the admin can inspect
  in the history.
- Public seller contact data becomes enumerable. This was explicitly requested
  and is limited to advisors' business contact details.
