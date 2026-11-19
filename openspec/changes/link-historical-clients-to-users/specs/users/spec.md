## ADDED Requirements

### Requirement: Historical clients in the users API

The users API SHALL expose `historical_client` on user responses. `GET /users`
SHALL accept `historical=1` to list only migrated customer accounts. The stats
endpoint SHALL return a separate `historical` count that is not added to the
total.

#### Scenario: Filter historical clients

- **WHEN** an admin requests `GET /users?historical=1`
- **THEN** only users with `historical_client = true` are returned

#### Scenario: Historical stats

- **WHEN** an admin requests `GET /users/stats`
- **THEN** the response includes the `historical` count
- **AND** the total remains the sum of the role counts
