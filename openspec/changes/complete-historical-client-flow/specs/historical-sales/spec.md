## ADDED Requirements

### Requirement: Single historical sale read

The system SHALL expose `GET /api/v1/historical_sales/:id` to load one
historical sale for the edit form. Sellers SHALL only read records they created;
admins SHALL read any record.

#### Scenario: Seller reads an own record

- **WHEN** a seller requests one of their historical sales
- **THEN** the API returns the record with its client, sale, commission and validation data

#### Scenario: Seller reads another seller's record

- **WHEN** a seller requests a historical sale created by someone else
- **THEN** the API responds with `403 Forbidden`

#### Scenario: Admin reads any record

- **WHEN** an admin requests a historical sale
- **THEN** the API returns the record regardless of its creator

#### Scenario: Record not found

- **WHEN** the requested historical sale does not exist
- **THEN** the API responds with `404 Not Found`

### Requirement: Rate-preserving historical update

The system SHALL preserve the stored commission rate when an update payload does
not include `rate`. An explicit `commission_amount` without a rate SHALL update
the amount while keeping the existing rate.

#### Scenario: Amount-only update keeps the rate

- **WHEN** a seller updates the commission amount of a pending historical record without sending a rate
- **THEN** the stored rate is unchanged
- **AND** the new amount is stored

#### Scenario: Seller updates a validated record

- **WHEN** a seller tries to update a validated historical record
- **THEN** the API responds with `403 Forbidden`
