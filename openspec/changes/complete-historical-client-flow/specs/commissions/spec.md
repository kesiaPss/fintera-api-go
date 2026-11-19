## ADDED Requirements

### Requirement: Effective historical rate for sellers

`GET /api/v1/commissions/rules` SHALL include `seller_rate` for non-admin actors,
resolved as the per-seller exception when present, otherwise the global default.
Admin responses SHALL keep the global rules without the field.

#### Scenario: Seller with an exception

- **WHEN** a seller with a 6% exception requests the commission rules
- **THEN** the response includes `seller_rate = 6`
- **AND** the global default rate is still returned

#### Scenario: Seller without an exception

- **WHEN** a seller without an exception requests the commission rules
- **THEN** `seller_rate` equals the global default rate

#### Scenario: Admin requests the rules

- **WHEN** an admin requests the commission rules
- **THEN** `seller_rate` is omitted
