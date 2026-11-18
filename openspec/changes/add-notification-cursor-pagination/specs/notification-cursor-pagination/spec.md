## Purpose

Give notification clients a stable, ID-based way to load older notifications
without duplicates or skipped rows.

## ADDED Requirements

### Requirement: Notification list supports cursor pagination

The notification list endpoints SHALL accept a `cursor` (notification ID) and
`limit`, and SHALL return notifications older than the cursor ordered newest
first.

#### Scenario: First page

- **WHEN** a client requests notifications without a cursor
- **THEN** the response contains the newest `limit` notifications
- **AND** `has_more` reflects whether older notifications exist

#### Scenario: Load more

- **WHEN** a client requests notifications with `cursor` equal to the oldest
  loaded notification ID
- **THEN** the response contains only notifications with a smaller ID
- **AND** `next_cursor` points at the oldest notification in the returned page

#### Scenario: End of list

- **WHEN** no notifications remain older than the cursor
- **THEN** `has_more` is false
- **AND** `next_cursor` is null

### Requirement: Notification list reports total and unread count

Each notification list response SHALL include the total matching the current
`status` filter and the user's total unread count, independent of the status
filter.

#### Scenario: Unread count exceeds one page

- **WHEN** a user has 40 unread notifications and the page size is 20
- **THEN** the first response reports `unread_count` as 40

#### Scenario: Status filter

- **WHEN** a client requests `status=unread`
- **THEN** only unread notifications are returned
- **AND** `pagination.total` counts only unread notifications

### Requirement: Limit is bounded

The list endpoints SHALL default `limit` to 20 and clamp it between 1 and 50.

#### Scenario: Invalid limit

- **WHEN** a client sends `limit=0` or a non-numeric limit
- **THEN** the server uses 20

#### Scenario: Large limit

- **WHEN** a client sends `limit=500`
- **THEN** the server uses 50
