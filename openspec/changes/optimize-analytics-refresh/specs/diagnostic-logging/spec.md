## Purpose

Make production logs trustworthy: normal control flow is not reported as a
failure, slow work is distinguishable, and every entry exposes a machine-readable
severity.

## ADDED Requirements

### Requirement: Cache misses are not errors

A lookup that finds no analytics cache row SHALL be treated as a cache miss. It
SHALL NOT be logged as an error and SHALL NOT emit an `SQL Error` entry.

#### Scenario: Missing cache entry

- **WHEN** the analytics service reads a cache key that does not exist
- **THEN** the refresh continues and inserts the computed value
- **AND** the logs contain no `SQL Error` entry with `record not found`

#### Scenario: Real database failure

- **WHEN** a query fails for a reason other than a missing row
- **THEN** it is still logged at error level with the error attached

### Requirement: Slow queries are reported as warnings

Queries that exceed the configured slow-query threshold SHALL be logged at warn
level with the statement, row count and elapsed time. Queries below the
threshold SHALL NOT be reported as warnings.

#### Scenario: Query over the threshold

- **WHEN** a query takes longer than the configured threshold
- **THEN** a `Slow SQL` warning includes the SQL, rows and elapsed fields

#### Scenario: Query under the threshold

- **WHEN** a query completes under the threshold
- **THEN** it is logged at info level, not as a warning

### Requirement: Log severity is machine-readable and accurate

In deployed environments the application SHALL emit structured log entries whose
severity field matches the level of the event. A warning or error SHALL NOT be
reported as informational and SHALL NOT depend on a collector to infer the
level.

#### Scenario: Error in production

- **WHEN** an error is logged in a deployed environment
- **THEN** the structured output reports error severity

#### Scenario: Warning in production

- **WHEN** a warning is logged in a deployed environment
- **THEN** the structured output reports warning severity

### Requirement: One environment source for logging

The database log level SHALL be derived from the same environment configuration
used to initialize the application logger, so the two cannot disagree.

#### Scenario: Production database logging

- **WHEN** the application runs with the production environment configured
- **THEN** per-query informational SQL logs are not emitted
- **AND** slow-query and error logs are still emitted
