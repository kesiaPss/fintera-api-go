## Purpose

Make the analytics statistics that clients display correct, stable and
comparable across periods.

## ADDED Requirements

### Requirement: Previous period mirrors the selected period

The previous-period comparison SHALL cover the immediately preceding window of
the same length as the selected period. When only one bound is supplied the
window SHALL be mirrored, and when no bounds are supplied the default comparison
SHALL be a complete 30-day window and the 30 days before it.

#### Scenario: Explicit month

- **WHEN** the overview is requested for 2026-10-01 through 2026-10-31
- **THEN** the previous window is 2026-08-31 through 2026-09-30

#### Scenario: Only an end date

- **WHEN** only an end date is supplied
- **THEN** the previous window ends the day before the derived start and has the
  same length as the current window

#### Scenario: No bounds

- **WHEN** no date bounds are supplied
- **THEN** the current and previous values come from two consecutive complete
  30-day windows, not from all-time totals compared to an unrelated window

### Requirement: Comparisons are computed for every metric

Every previous-period metric payload SHALL report the value computed for the
comparison window, including occupancy, and SHALL NOT report a hard-coded or
unknown previous value.

#### Scenario: Default dashboard occupancy

- **WHEN** the overview is requested without an end date
- **THEN** the previous occupancy rate is computed for the comparison window
- **AND** the occupancy change is not always 100%

#### Scenario: No prior activity

- **WHEN** the comparison window has no activity for a metric
- **THEN** the previous value is zero
- **AND** the change percentage does not claim an increase when the current
  value is also zero

### Requirement: Revenue trend is chronological and year-aware

Revenue trend points SHALL be grouped by calendar year and month and returned in
ascending chronological order. Payments from the same month in different years
SHALL NOT be merged into one point.

#### Scenario: Two Novembers

- **WHEN** paid payments exist in November 2025 and November 2026
- **THEN** the trend contains two distinct points in that order

#### Scenario: Mixed real and projected months

- **WHEN** some months contain only pending payments and others contain paid
  payments
- **THEN** the merged response is ordered by month, not by map iteration

### Requirement: Weekly performance reflects real data

Project weekly performance SHALL be derived from actual payment activity. The
response SHALL NOT contain a constant placeholder series shared by every
project.

#### Scenario: Weeks with collections

- **WHEN** a project has paid payments in the recent weeks
- **THEN** its weekly performance reflects those collections

#### Scenario: Weeks without collections

- **WHEN** a project has no recent collection data
- **THEN** the response returns the available weeks or omits the field instead
  of returning fabricated values
