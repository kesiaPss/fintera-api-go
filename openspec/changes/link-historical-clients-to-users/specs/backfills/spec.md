## ADDED Requirements

### Requirement: Versioned backfill runner

The API SHALL provide a registry of one-off backfill tasks with a name, version
and run function. Every execution SHALL be recorded in `backfill_runs` with
status, rows affected, details and error. A task SHALL be skipped when a
completed run exists for its name and version. The API SHALL run pending tasks
asynchronously at startup, and operators SHALL be able to list or force a task
with the generic command.

#### Scenario: Pending task runs at startup

- **WHEN** the API starts and a registered task has no completed run
- **THEN** the background worker executes it
- **AND** a completed row is recorded

#### Scenario: Completed task is skipped

- **WHEN** the API restarts and the task already has a completed run
- **THEN** the task is not executed again
- **AND** the existing run is reported as already completed

#### Scenario: Force a task

- **WHEN** an operator runs `backfill -task=<name> -force`
- **THEN** the task executes again
- **AND** a new run row is recorded

#### Scenario: Failed task retries

- **WHEN** a task fails
- **THEN** the run is recorded as failed with the error
- **AND** the next startup retries it because there is no completed run
