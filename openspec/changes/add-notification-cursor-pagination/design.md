## Context

`NotificationHandler.Index` and `IndexByUser` call
`NotificationService.FindByUser`, which delegates to
`NotificationRepository.FindByUser`. That method:

- counts all matching rows,
- orders by `created_at DESC`,
- applies `Offset((page - 1) * perPage).Limit(perPage)`.

The UI calls `notificationsApi.list({})`, receives the default first page, and
has no cursor or has-more metadata. `NotificationsBell` derives the unread badge
from the loaded rows, so the badge undercounts once notifications exceed one
page.

## Goals / Non-Goals

**Goals:**

- Provide stable load-more pagination keyed by notification ID.
- Return enough metadata for the bell to render "Load more" and an accurate
  unread badge.
- Preserve the `status=unread|read` filter.

**Non-Goals:**

- Changing notification creation, delivery, or read semantics.
- Adding real-time updates or polling.
- Reworking the seller assignment notification merge logic.

## Decisions

### ID cursor instead of offset

Notification IDs are serial primary keys assigned in creation order. Ordering
by `id DESC` and filtering `id < cursor` is stable even when new notifications
are inserted between requests, which is the main failure mode of offset
pagination for a live bell.

Alternative: keep `page`/`per_page` and accept occasional duplicates. Rejected
because the bell appends rows, so duplicates would be visible.

### Fetch `limit + 1` to detect more pages

The repository requests one extra row. The service trims it and sets
`has_more`. This avoids a second count query just to answer "is there another
page?", while the existing total count is still returned for compatibility.

### Response shape

```json
{
  "notifications": [...],
  "next_cursor": 123,
  "has_more": true,
  "unread_count": 7,
  "pagination": { "total": 57 }
}
```

`next_cursor` is `null` when `has_more` is false. `unread_count` ignores the
current `status` filter and always reports the user's unread total.

### Limit bounds

`limit` defaults to 20 and is clamped to 1..50. The UI uses 20.

## Risks / Trade-offs

- [Cursor assumes ID order matches creation order] → IDs are serial and
  `created_at` is set at insert; the bell only needs newest-first, so ID order is
  appropriate.
- [Total count adds a query per page] → It preserves the existing response and
  runs against `notifications.user_id`, which is indexed; the page query itself
  remains the dominant cost.
- [Unread count adds a query per page] → It is a single indexed `COUNT` and
  makes the badge correct with pagination.

## Migration Plan

No database migration. Deploy the backend before the UI that sends `cursor` and
`limit`; older clients that send `page` still receive the first page because
unknown query params are ignored.

## Open Questions

None.
