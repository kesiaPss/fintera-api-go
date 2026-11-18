# Add Public Seller Picker and Admin Reassignment

## Why

Organic reservations reached the API without a seller, so commissions were
attributed to "Unknown". The customer portal needs a safe way to list active
advisors, and admins need to correct the seller of a contract after it has been
created — including contracts whose commission was already paid.

## What Changes

- Add `GET /api/v1/public/sellers`, exposing only the name and contact data of
  active sellers.
- Add admin-only assignment history and reassignment endpoints for contracts:
  - `GET /contracts/{guid}/assignments`
  - `POST /contracts/{guid}/assignments`
- Allow reassignment in any contract status.
- When a commission was already paid to the previous seller, create a pending
  recoverable commission credit for them before assigning the new seller.
- Add a `superseded` assignment status that does not affect seller scores.
- Return machine-readable codes (`SELLER_INACTIVE`, `LOT_UNAVAILABLE`) from the
  public reservation create endpoint.

## Capabilities

### New Capabilities

- `public-seller-picker`: public advisor list for the customer portal.
- `contract-reassignment`: admin seller reassignment and commission recovery.

## Impact

- `cmd/api/main.go`
- `internal/models/{user,assignment,contract}.go`
- `internal/repository/assignment_repository.go`
- `internal/services/{seller,assignment,contract,commission,report}_service.go`
- `internal/handlers/{seller,contract}_handler.go`
- Tests and Swagger docs.
