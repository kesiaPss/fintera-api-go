package models

import (
	"math"
	"time"
)

// ContractPeriodMetrics summarizes contract activity for a report period.
//
// Counting rules:
//   - Approved counts every contract whose ApprovedAt falls inside the period,
//     regardless of the status it has today (a contract approved in the period
//     and closed afterwards still happened in the period).
//   - Closed uses ClosedAt and Rejected uses RejectedAt.
//   - Cancelled has no dedicated timestamp column, so UpdatedAt is used: the
//     cancellation is the last status change of the row.
//   - SalesValue is the sum of Amount for the contracts approved in the period,
//     so it reconciles with the FinancingSales total for the same period.
type ContractPeriodMetrics struct {
	Approved   int     `json:"approved"`
	Closed     int     `json:"closed"`
	Cancelled  int     `json:"cancelled"`
	Rejected   int     `json:"rejected"`
	SalesValue float64 `json:"sales_value"`
}

// FinancingSalesRow is one financing type inside the sales breakdown.
type FinancingSalesRow struct {
	FinancingType string  `json:"financing_type"`
	Count         int     `json:"count"`
	Value         float64 `json:"value"`
}

// FinancingSales groups approved contracts by financing type (direct, bank,
// cash). TotalValue and TotalCount are the sums over Metrics, so renderers can
// print a total row without recomputing.
type FinancingSales struct {
	Metrics    []FinancingSalesRow `json:"metrics"`
	TotalValue float64             `json:"total_value"`
	TotalCount int                 `json:"total_count"`
}

// CollectionTypeRow is one payment type inside a collections breakdown. Amount
// is the sum of paid_amount for paid payments, or the sum of amount for the
// projected rows, depending on the section.
type CollectionTypeRow struct {
	PaymentType string  `json:"payment_type"`
	Count       int     `json:"count"`
	Amount      float64 `json:"amount"`
}

// CollectionsSummary describes collections of a period compared against the
// amount that was due in the same period.
//
//   - ByType holds paid payments grouped by payment_type.
//   - Total is the sum of ByType amounts.
//   - InterestCollected is SUM(interest_amount) over the paid payments.
//   - ProjectedDue is SUM(amount) of pending payments due in the period.
//   - Effectiveness is Total/ProjectedDue*100, rounded to two decimals, and 0
//     when ProjectedDue is 0.
type CollectionsSummary struct {
	ByType            []CollectionTypeRow `json:"by_type"`
	Total             float64             `json:"total"`
	InterestCollected float64             `json:"interest_collected"`
	ProjectedDue      float64             `json:"projected_due"`
	Effectiveness     float64             `json:"effectiveness"`
}

// LedgerTypeTotal is the signed ledger total of one entry type. Amounts are
// stored signed by the authoritative ledger implementation: initial and
// interest entries are negative (debt increases), payment and prepayment
// entries are positive (debt decreases), and adjustment/refund keep the sign
// they were written with.
type LedgerTypeTotal struct {
	EntryType string  `json:"entry_type"`
	Total     float64 `json:"total"`
}

// PortfolioSnapshot classifies the cached contract balances.
//
// OutstandingBalance is the total debt as a positive magnitude (the ledger
// keeps debt as a negative balance, so it is -SUM(balance) over contracts with
// balance < 0). SettledContracts counts approved or closed contracts whose
// balance is zero or positive.
type PortfolioSnapshot struct {
	OutstandingBalance float64 `json:"outstanding_balance"`
	SettledContracts   int     `json:"settled_contracts"`
}

// IncomeFlowMonth is one row of the monthly income flow. Actual comes from paid
// payments by payment_date, Projected from pending payments by due_date,
// Deviation is Actual-Projected, and Effectiveness is Actual/Projected*100 (0
// when there is no projection). ByType breaks Actual down by payment type.
type IncomeFlowMonth struct {
	Month         time.Time           `json:"month"`
	Actual        float64             `json:"actual"`
	Projected     float64             `json:"projected"`
	Deviation     float64             `json:"deviation"`
	Effectiveness float64             `json:"effectiveness"`
	ByType        []CollectionTypeRow `json:"by_type"`
}

// ProjectCollections is the collected amount of one project for a period.
type ProjectCollections struct {
	ProjectID   uint    `json:"project_id"`
	ProjectName string  `json:"project_name"`
	Amount      float64 `json:"amount"`
	Count       int     `json:"count"`
}

// Effectiveness returns collected/projected*100 rounded to two decimals and 0
// when projected is zero (or negative), so report builders never divide by
// zero on an empty period.
func Effectiveness(collected, projected float64) float64 {
	if projected <= 0 {
		return 0
	}
	return math.Round(collected/projected*10000) / 100
}
