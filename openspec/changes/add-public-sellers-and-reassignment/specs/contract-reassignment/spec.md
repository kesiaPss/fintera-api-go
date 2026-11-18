## ADDED Requirements

### Requirement: Public seller picker

The API SHALL expose active sellers to the public portal without requiring a
session. Only the seller's name and contact data may be returned.

#### Scenario: Active sellers

- **WHEN** an anonymous client calls `GET /public/sellers`
- **THEN** the response contains active sellers with `id`, `full_name`,
  `phone` and `email`
- **AND** no score, sales count or internal identifiers are present

### Requirement: Admin reassignment

An admin SHALL be able to list the assignment history and reassign the seller
of a contract in any status.

#### Scenario: Reassign to a specific seller

- **WHEN** an admin posts `mode=seller` with an active `seller_id`
- **THEN** the contract points to that seller
- **AND** the new assignment is recorded with the requested origin
- **AND** an audit entry names the admin

#### Scenario: Automatic reassignment

- **WHEN** an admin posts `mode=automatic`
- **THEN** the API selects an eligible seller with the existing fairness rules
- **AND** can exclude the current seller

#### Scenario: Paid commission

- **WHEN** the contract commission was already paid to a different seller
- **THEN** a pending recoverable commission credit is created for that seller
- **AND** the new seller's commission is recalculated
- **AND** the contract no longer appears as paid

#### Scenario: Superseded offer

- **WHEN** a contract already has a pending offer
- **THEN** the previous offer is marked `superseded`
- **AND** it does not affect the seller's score window

### Requirement: Reservation error codes

The public reservation endpoint SHALL distinguish an unavailable lot from an
inactive advisor.

#### Scenario: Inactive advisor

- **WHEN** the selected seller is no longer active
- **THEN** the response is `422` with code `SELLER_INACTIVE`

#### Scenario: Unavailable lot

- **WHEN** the lot is no longer available
- **THEN** the response is `422` with code `LOT_UNAVAILABLE`
