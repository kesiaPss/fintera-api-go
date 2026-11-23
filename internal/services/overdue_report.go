package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
)

// This file builds the overdue payments export document. The repository
// already excludes cancelled contracts, does not skip reminded rows and
// applies the seller/project scope in SQL; the builder buckets the streamed
// rows by days past due and labels them.

// overdueBucketOrder is the fixed summary order of the aging buckets.
var overdueBucketOrder = []string{
	AgingBucket1To30,
	AgingBucket31To60,
	AgingBucket61To90,
	AgingBucket90Plus,
}

// overdueBucketSummary accumulates one aging bucket while the detail lines are
// walked.
type overdueBucketSummary struct {
	Bucket   string
	Payments int
	Capital  float64
	Interest float64
}

// overdueDetailLine is one overdue payment with its computed aging data.
type overdueDetailLine struct {
	Payment models.Payment
	Days    int
	Bucket  string
}

// BuildOverdueReport builds the overdue payments report document: an aging
// bucket summary and the client detail of every overdue payment of an active
// contract, optionally scoped to a seller or a project. The dataset arrives in
// keyset batches with the scope applied in SQL; only bucket counters and
// rendered rows are kept, so peak memory is one batch of payments. Rows are
// re-sorted by due date at the end, matching the previous full-load path.
func (s *ReportExportService) BuildOverdueReport(ctx context.Context, filters ReportExportFilters) (*ReportDocument, error) {
	now := time.Now()

	summaries := make(map[string]*overdueBucketSummary, len(overdueBucketOrder))
	for _, bucket := range overdueBucketOrder {
		summaries[bucket] = &overdueBucketSummary{Bucket: bucket}
	}

	type detailEntry struct {
		dueDate time.Time
		id      uint
		row     ReportRow
	}
	var entries []detailEntry

	err := s.overdueRepo.StreamOverdueForActiveContractsDetailed(ctx, filters.ProjectID,
		filters.SellerID, filters.FilterBySeller, repository.PaymentScanBatchSize,
		func(payments []models.Payment) error {
			for i := range payments {
				payment := &payments[i]
				days := payment.OverdueDays()
				if days < 1 {
					days = 1
				}
				bucket := overdueBucketForDays(days)
				if summary, ok := summaries[bucket]; ok {
					summary.Payments++
					summary.Capital += payment.Amount
					summary.Interest += floatValue(payment.InterestAmount)
				}
				entries = append(entries, detailEntry{
					dueDate: payment.DueDate,
					id:      payment.ID,
					row:     overdueDetailRow(overdueDetailLine{Payment: *payment, Days: days, Bucket: bucket}),
				})
			}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to load overdue payments for the report: %w", err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].dueDate.Equal(entries[j].dueDate) {
			return entries[i].id < entries[j].id
		}
		return entries[i].dueDate.Before(entries[j].dueDate)
	})

	rows := make([]ReportRow, 0, len(entries))
	for i := range entries {
		rows = append(rows, entries[i].row)
	}

	period, periodKey := reportPeriodLabel(filters.StartDate, filters.EndDate, now)

	document := &ReportDocument{
		Title:       ReportTitleOverdue,
		Subtitle:    "Saldos vencidos de contratos activos por tramo de mora",
		Period:      period,
		PeriodKey:   periodKey,
		BaseName:    ReportBaseOverdue,
		GeneratedAt: now,
		Currency:    "HNL",
	}

	document.Sections = append(document.Sections,
		overdueSummarySection(summaries),
		overdueDetailSection(rows),
		ReportSection{
			Title: "Notas",
			Kind:  ReportSectionNote,
			Note:  "No se incluyen pagos de contratos cancelados.",
		},
	)

	return document, nil
}

// overdueBucketForDays maps days past due to an aging bucket. Boundaries are
// inclusive: 30 -> 1-30, 31 -> 31-60, 60 -> 31-60, 61 -> 61-90, 90 -> 61-90
// and 91 -> 90+.
func overdueBucketForDays(days int) string {
	switch {
	case days <= 30:
		return AgingBucket1To30
	case days <= 60:
		return AgingBucket31To60
	case days <= 90:
		return AgingBucket61To90
	default:
		return AgingBucket90Plus
	}
}

// overdueSummarySection renders one row per aging bucket plus the grand total.
// Every bucket is present even when it has no payments.
func overdueSummarySection(summaries map[string]*overdueBucketSummary) ReportSection {
	rows := make([]ReportRow, 0, len(overdueBucketOrder))
	var totalPayments int
	var totalCapital, totalInterest float64

	for _, bucket := range overdueBucketOrder {
		summary := summaries[bucket]
		if summary == nil {
			summary = &overdueBucketSummary{Bucket: bucket}
		}
		totalPayments += summary.Payments
		totalCapital += summary.Capital
		totalInterest += summary.Interest

		rows = append(rows, ReportRow{
			Cells: []ReportCell{
				TextCell(TranslateAgingBucket(bucket)),
				IntCell(summary.Payments),
				MoneyCell(summary.Capital, "HNL"),
				MoneyCell(summary.Interest, "HNL"),
				MoneyCell(summary.Capital+summary.Interest, "HNL"),
			},
			Kind: ReportRowNormal,
		})
	}

	return ReportSection{
		Title:   "Resumen por Antigüedad",
		Kind:    ReportSectionTable,
		Headers: []string{"Tramo de Mora", "Pagos", "Capital", "Interés Acumulado", "Total"},
		Rows:    rows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				IntCell(totalPayments),
				MoneyCell(totalCapital, "HNL"),
				MoneyCell(totalInterest, "HNL"),
				MoneyCell(totalCapital+totalInterest, "HNL"),
			},
			Kind: ReportRowTotal,
		},
	}
}

// overdueDetailSection renders one line per overdue payment ordered by due
// date, with the fiscal data of the applicant and the contract bank reference.
func overdueDetailSection(rows []ReportRow) ReportSection {
	return ReportSection{
		Title: "Detalle de Pagos Vencidos",
		Kind:  ReportSectionTable,
		Headers: []string{
			"Contrato",
			"Cliente",
			"Identidad",
			"RTN",
			"Teléfono",
			"Fecha de Vencimiento",
			"Días de Mora",
			"Tipo de Pago",
			"Capital",
			"Interés Acumulado",
			"Saldo del Contrato",
			"Referencia Bancaria",
		},
		Rows: rows,
	}
}

// overdueDetailRow renders one overdue payment line. The row style escalates
// with the aging bucket: 1-30 normal, 31-90 warning, 90+ danger.
func overdueDetailRow(detail overdueDetailLine) ReportRow {
	payment := detail.Payment
	contract := payment.Contract
	applicant := contract.ApplicantUser
	currency := firstNonEmpty(contract.Currency, "HNL")

	return ReportRow{
		Cells: []ReportCell{
			TextCell(fmt.Sprintf("#%d", payment.ContractID)),
			TextCell(applicant.FullName),
			TextCell(applicant.Identity),
			TextCell(applicant.RTN),
			TextCell(applicant.Phone),
			DateCell(payment.DueDate),
			IntCell(detail.Days),
			TextCell(TranslatePaymentType(payment.PaymentType)),
			MoneyCell(payment.Amount, currency),
			MoneyCell(floatValue(payment.InterestAmount), currency),
			MoneyCell(overdueContractBalance(contract), currency),
			TextCell(overdueBankReference(payment)),
		},
		Kind: overdueRowKind(detail.Bucket),
	}
}

// overdueContractBalance returns the outstanding debt as a positive magnitude.
// The ledger keeps debt negative on the contract balance cache, while the
// export shows it as the amount owed.
func overdueContractBalance(contract models.Contract) float64 {
	if contract.Balance == nil || *contract.Balance >= 0 {
		return 0
	}
	return -*contract.Balance
}

// overdueBankReference returns the card authorization number, falling back to
// the payer account name, and stays empty when both are missing.
func overdueBankReference(payment models.Payment) string {
	return strings.TrimSpace(firstNonEmpty(
		pointerValue(payment.AuthorizationNumber),
		pointerValue(payment.PayerAccountName),
	))
}

// overdueRowKind maps an aging bucket to its severity style.
func overdueRowKind(bucket string) ReportRowKind {
	switch bucket {
	case AgingBucket31To60, AgingBucket61To90:
		return ReportRowWarning
	case AgingBucket90Plus:
		return ReportRowDanger
	default:
		return ReportRowNormal
	}
}
