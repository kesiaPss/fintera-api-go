## 1. Public sellers

- [x] 1.1 Add `PublicSellerSummary` and `SellerService.ListPublic`.
- [x] 1.2 Add `GET /public/sellers` and the handler test coverage at the
      service level.

## 2. Reassignment

- [x] 2.1 Add the `superseded` assignment status and repository queries.
- [x] 2.2 Add `AssignNow`, `SupersedePending` and `History` to the assignment
      service.
- [x] 2.3 Add `ContractService.Reassign` and `ContractService.AssignmentHistory`.
- [x] 2.4 Add admin routes for assignment history and reassignment.
- [x] 2.5 Recover paid commissions through a pending credit for the previous
      seller.
- [x] 2.6 Add `SELLER_INACTIVE` and `LOT_UNAVAILABLE` error codes.
- [x] 2.7 Keep the commissions report from using customer names as sellers.

## 3. Verification

- [x] 3.1 Add service and commission tests.
- [x] 3.2 Run `go build ./...`, `go test ./...` and regenerate Swagger.
