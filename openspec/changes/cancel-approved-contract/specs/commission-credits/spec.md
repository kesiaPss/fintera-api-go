## ADDED Requirements

### Requirement: Commission credit on cancellation

When a contract with a paid commission is cancelled, the system SHALL create a
commission credit for the seller for the paid amount. An admin SHALL be able to
apply the credit manually.

#### Scenario: Credit is created

- **WHEN** a contract with a paid commission is cancelled
- **THEN** a `pending` commission credit is created for the seller
- **AND** the credit amount equals the paid commission

#### Scenario: Unpaid commission

- **WHEN** a contract whose commission was not paid is cancelled
- **THEN** no commission credit is created
- **AND** the contract is excluded from the commissions report

### Requirement: Apply a commission credit

An admin SHALL be able to apply a commission credit, marking it `settled` and
recording the actor.

#### Scenario: Admin applies a credit

- **WHEN** an admin applies a pending commission credit
- **THEN** the credit becomes `settled`
- **AND** an audit entry records the actor

#### Scenario: Seller sees the credit

- **WHEN** a seller has a pending commission credit
- **THEN** their commissions view shows the credit read-only
