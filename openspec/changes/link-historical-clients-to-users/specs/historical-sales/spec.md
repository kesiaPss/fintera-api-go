## ADDED Requirements

### Requirement: Historical client customer account

Each historical sale SHALL be linked to a real `users` row with
`role = user` through `historical_sales.client_user_id`. The system SHALL reuse
an existing customer by identity before creating a new account. Created accounts
SHALL be marked `historical_client = true` and SHALL NOT receive a welcome email.

#### Scenario: Create a customer account

- **WHEN** a historical sale is created with an identity that has no user
- **THEN** a customer account is created and linked
- **AND** the response includes its `client_user_id`

#### Scenario: Reuse an existing customer

- **WHEN** the identity already belongs to a customer account
- **THEN** that account is linked
- **AND** no duplicate is created

#### Scenario: Missing email

- **WHEN** the historical record has no email
- **THEN** the account receives a non-deliverable placeholder address
- **AND** it is marked `historical_client = true`

#### Scenario: Staff identity

- **WHEN** the identity belongs to an admin or seller
- **THEN** the request is rejected with a conflict
- **AND** no client account is created

### Requirement: Historical client backfill

The API SHALL link historical sales created before the feature asynchronously at
startup. The backfill SHALL be idempotent and SHALL skip already linked records.

#### Scenario: API startup backfill

- **WHEN** the API starts with unlinked historical sales
- **THEN** the background worker links them to customer accounts
- **AND** a second run makes no changes
