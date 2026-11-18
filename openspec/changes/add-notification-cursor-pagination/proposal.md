## Why

The notification bell loads the first page of notifications and has no way to
request older items. The API currently exposes offset pagination
(`page`/`per_page`), which can duplicate or skip rows when new notifications
arrive while the user is paging, and it only returns `pagination.total`, so a
client cannot tell whether another page exists without comparing counts.

Cursor pagination by notification ID gives the UI a stable "load more"
contract: each request asks for the rows older than the last item the client
already has, and new notifications created in the meantime do not shift the
page window.

## What Changes

- Replace `NotificationRepository.FindByUser` with a cursor query that returns
  one page of notifications plus the matching total.
- Add a service-level `NotificationPage` result with the notifications, total,
  unread count, next cursor, and has-more flag.
- Update `GET /notifications` and `GET /users/:id/notifications` to accept
  `cursor` and `limit` (default 20, max 50) and return `next_cursor`,
  `has_more`, and `unread_count`.
- Keep the existing `status=unread|read` filter and return an accurate unread
  count so the bell badge does not depend on how many rows are loaded.
- Update Swagger annotations and add repository/handler tests.

## Capabilities

### New Capabilities

- `notification-cursor-pagination`: stable ID-based pagination for the
  notification list, with load-more metadata and an accurate unread count.

### Modified Capabilities

None.

## Impact

- `internal/repository/contract_repository.go` (`NotificationRepository`,
  `notificationRepository`)
- `internal/services/notification_service.go`
- `internal/handlers/resource_handlers.go` (`NotificationHandler`)
- `internal/handlers/helpers.go` (limit parsing helper)
- Repository and handler tests
- `docs/` Swagger output via `make swagger`
