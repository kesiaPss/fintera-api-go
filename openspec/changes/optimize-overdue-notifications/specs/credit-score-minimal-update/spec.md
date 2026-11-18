## Purpose

Keep credit-score updates narrow so they do not rewrite unrelated user data or
add unnecessary write load.

## ADDED Requirements

### Requirement: Credit-score updates touch only the changed columns

Updating a user's credit score SHALL write only `credit_score` and
`updated_at`; it SHALL NOT update `email`, `encrypted_password`, `role`, or any
other user column.

#### Scenario: Score changes after a payment

- **WHEN** the credit-score service updates a user's score
- **THEN** the generated SQL updates only `credit_score` and `updated_at`
- **AND** the user's other fields remain unchanged

#### Scenario: Concurrent unrelated user edits

- **WHEN** a credit-score update runs at the same time as an unrelated user edit
- **THEN** the credit-score update does not overwrite the unrelated edit
