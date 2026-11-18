## 1. Repository cursor query

- [x] 1.1 Replace `NotificationRepository.FindByUser` with `FindByUserPage(ctx, userID, status, cursorID, limit) ([]models.Notification, int64, error)`, ordering by `id DESC`, filtering `id < cursorID` when a cursor is present, and fetching `limit + 1` rows
- [x] 1.2 Add repository tests for the first page, a cursor page, the `unread` filter, and the `limit + 1` fetch

## 2. Service and handler

- [x] 2.1 Add a `NotificationPage` result containing notifications, total, unread count, next cursor, and has-more
- [x] 2.2 Update `NotificationService` to build the page, trim the extra row, and compute the next cursor
- [x] 2.3 Update `NotificationHandler.Index` and `IndexByUser` to parse `cursor` and `limit`, return `next_cursor`, `has_more`, and `unread_count`, and keep `pagination.total`
- [x] 2.4 Add a `parseNotificationLimit` helper defaulting to 20 and clamping to 1..50
- [x] 2.5 Update the Swagger annotations and run `make swagger`
- [x] 2.6 Add handler tests for the first page, cursor request, and invalid cursor/limit values

## 3. Verification

- [x] 3.1 Run `go build ./...` and `go test ./...`
- [x] 3.2 Run `make lint`
