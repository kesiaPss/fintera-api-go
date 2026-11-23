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

// incomeFlowReportCurrency is the aggregate currency of the income flow
// report. The analytics aggregations sum payments regardless of the currency
// of their contracts, so the aggregate sections are reported in HNL; the
// collection detail keeps the currency of each contract.
const incomeFlowReportCurrency = "HNL"

// incomeFlowSellerScope returns the seller id that scopes the income flow
// report, or nil for admin callers (and for sellers without an id).
func incomeFlowSellerScope(filters ReportExportFilters) *uint {
	if !filters.FilterBySeller || filters.SellerID == 0 {
		return nil
	}
	sellerID := filters.SellerID
	return &sellerID
}

// incomeFlowLedgerOrder is the fixed order of the ledger entry types in the
// income flow report.
var incomeFlowLedgerOrder = []string{
	models.EntryTypeInitial,
	models.EntryTypeInterest,
	models.EntryTypePayment,
	models.EntryTypePrepayment,
	models.EntryTypeAdjustment,
	models.EntryTypeRefund,
}

// incomeFlowLedgerLabels maps ledger entry types to their Spanish labels.
var incomeFlowLedgerLabels = map[string]string{
	models.EntryTypeInitial:    "Saldo inicial",
	models.EntryTypeInterest:   "Interés",
	models.EntryTypePayment:    "Pagos",
	models.EntryTypePrepayment: "Prepagos",
	models.EntryTypeAdjustment: "Ajustes",
	models.EntryTypeRefund:     "Devoluciones",
}

// incomeFlowLedgerLabel returns the Spanish label of an entry type, falling
// back to the raw value so unexpected types are never dropped silently.
func incomeFlowLedgerLabel(entryType string) string {
	if label, ok := incomeFlowLedgerLabels[entryType]; ok {
		return label
	}
	return entryType
}

// incomeFlowLedgerSection builds the signed accounting movement table. Amounts
// are printed exactly as the authoritative ledger stores them: negative
// entries increase debt and positive entries reduce it.
func incomeFlowLedgerSection(totals []models.LedgerTypeTotal) ReportSection {
	amounts := make(map[string]float64, len(totals))
	for _, total := range totals {
		amounts[total.EntryType] = total.Total
	}

	ordered := make([]string, 0, len(totals)+len(incomeFlowLedgerOrder))
	seen := make(map[string]bool, len(totals))
	for _, entryType := range incomeFlowLedgerOrder {
		ordered = append(ordered, entryType)
		seen[entryType] = true
	}
	for _, total := range totals {
		if !seen[total.EntryType] {
			ordered = append(ordered, total.EntryType)
			seen[total.EntryType] = true
		}
	}

	rows := make([]ReportRow, 0, len(ordered))
	net := 0.0
	for _, entryType := range ordered {
		amount := amounts[entryType]
		net += amount
		rows = append(rows, ReportRow{
			Cells: []ReportCell{
				TextCell(incomeFlowLedgerLabel(entryType)),
				MoneyCell(amount, incomeFlowReportCurrency),
			},
		})
	}

	return ReportSection{
		Title:   "Movimientos Contables del Período",
		Kind:    ReportSectionTable,
		Headers: []string{"Tipo de Movimiento", "Monto"},
		Rows:    rows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				MoneyCell(net, incomeFlowReportCurrency),
			},
			Kind: ReportRowTotal,
		},
		Note: "Los montos negativos aumentan la deuda; los positivos la reducen.",
	}
}

// incomeFlowMonthLabel renders a month as "Octubre 2026".
func incomeFlowMonthLabel(month time.Time) string {
	return fmt.Sprintf("%s %d", spanishMonthName(month.Month()), month.Year())
}

// incomeFlowTypeAmounts maps the payment-type breakdown of a month into the
// five fixed columns of the monthly table: reservation, down payment,
// installment, advance to principal and full payment. The two advance
// constants share one column because they share the same Spanish label.
func incomeFlowTypeAmounts(rows []models.CollectionTypeRow) [5]float64 {
	var amounts [5]float64
	for _, row := range rows {
		switch row.PaymentType {
		case models.PaymentTypeReservation:
			amounts[0] += row.Amount
		case models.PaymentTypeDownPayment:
			amounts[1] += row.Amount
		case models.PaymentTypeInstallment:
			amounts[2] += row.Amount
		case models.PaymentTypeAdvance, models.PaymentTypeCapitalRepayment:
			amounts[3] += row.Amount
		case models.PaymentTypeFull:
			amounts[4] += row.Amount
		}
	}
	return amounts
}

// incomeFlowDetailSection loads the paid payments of the period and renders
// the fiscal collection detail. The rows are read in keyset batches with the
// project/seller scope applied in SQL (the previous PerPage=0 query loaded
// every paid payment of the period and filtered in Go, duplicating the whole
// slice). Detail rows are the only data kept: the payment models are released
// after each batch.
func (s *ReportExportService) incomeFlowDetailSection(ctx context.Context, filters ReportExportFilters) (ReportSection, error) {
	type detailEntry struct {
		date time.Time
		row  ReportRow
	}
	var entries []detailEntry

	err := s.paymentRepo.StreamPaidForReport(ctx, filters.StartDate, filters.EndDate,
		filters.ProjectID, filters.SellerID, filters.FilterBySeller,
		repository.PaymentScanBatchSize,
		func(payments []models.Payment) error {
			for i := range payments {
				payment := &payments[i]

				dateCell := TextCell("")
				if payment.PaymentDate != nil {
					dateCell = DateCell(*payment.PaymentDate)
				}

				amount := payment.Amount
				if payment.PaidAmount != nil {
					amount = *payment.PaidAmount
				}

				entries = append(entries, detailEntry{
					date: incomeFlowPaymentDate(*payment),
					row: ReportRow{
						Cells: []ReportCell{
							dateCell,
							TextCell(payment.Contract.GUID),
							TextCell(payment.Contract.ApplicantUser.FullName),
							TextCell(payment.Contract.ApplicantUser.Identity),
							TextCell(payment.Contract.ApplicantUser.RTN),
							TextCell(incomeFlowBankReference(*payment)),
							TextCell(TranslatePaymentType(payment.PaymentType)),
							TextCell(payment.Contract.Lot.Project.Name),
							TextCell(payment.Contract.Lot.Name),
							MoneyCell(amount, payment.Contract.Currency),
						},
					},
				})
			}
			return nil
		})
	if err != nil {
		return ReportSection{}, fmt.Errorf("no se pudo obtener el detalle de cobranzas: %w", err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].date.Before(entries[j].date)
	})

	rows := make([]ReportRow, 0, len(entries))
	for i := range entries {
		rows = append(rows, entries[i].row)
	}

	return ReportSection{
		Title: "Detalle de Cobranzas",
		Kind:  ReportSectionTable,
		Headers: []string{
			"Fecha Pago", "Contrato", "Cliente", "Identidad", "RTN",
			"Referencia Bancaria", "Tipo de Pago", "Proyecto", "Lote", "Monto",
		},
		Rows: rows,
	}, nil
}

// incomeFlowBankReference returns the bank reference of a payment: the
// authorization number when present, otherwise the payer account name, and an
// empty string when neither is available.
func incomeFlowBankReference(payment models.Payment) string {
	if payment.AuthorizationNumber != nil && strings.TrimSpace(*payment.AuthorizationNumber) != "" {
		return *payment.AuthorizationNumber
	}
	if payment.PayerAccountName != nil && strings.TrimSpace(*payment.PayerAccountName) != "" {
		return *payment.PayerAccountName
	}
	return ""
}

// incomeFlowPaymentDate returns the date used to sort and render a detail
// line: the payment date when set, otherwise the due date.
func incomeFlowPaymentDate(payment models.Payment) time.Time {
	if payment.PaymentDate != nil {
		return *payment.PaymentDate
	}
	return payment.DueDate
}

// BuildIncomeFlowReport assembles the income flow report: monthly actual
// versus projected collections by payment type, collections per project,
// signed accounting movements and the fiscal collection detail.
func (s *ReportExportService) BuildIncomeFlowReport(ctx context.Context, filters ReportExportFilters) (*ReportDocument, error) {
	generatedAt := time.Now()
	periodLabel, periodKey := reportPeriodLabel(filters.StartDate, filters.EndDate, generatedAt)
	sellerID := incomeFlowSellerScope(filters)

	months, err := s.analyticsRepo.GetIncomeFlowMonthly(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudo obtener el flujo mensual: %w", err)
	}

	byProject, err := s.analyticsRepo.GetCollectionsByProject(ctx, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las cobranzas por proyecto: %w", err)
	}

	ledgerTotals, err := s.analyticsRepo.GetLedgerTotalsByEntryType(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener los movimientos contables: %w", err)
	}

	detail, err := s.incomeFlowDetailSection(ctx, filters)
	if err != nil {
		return nil, err
	}

	document := &ReportDocument{
		Title:       ReportTitleIncomeFlow,
		Subtitle:    "Cobranza real versus proyectada",
		Period:      periodLabel,
		PeriodKey:   periodKey,
		BaseName:    ReportBaseIncomeFlow,
		GeneratedAt: generatedAt,
		Currency:    incomeFlowReportCurrency,
	}

	// 1. Monthly actual versus projected flow.
	monthRows := make([]ReportRow, 0, len(months))
	var typeTotals [5]float64
	totalActual := 0.0
	totalProjected := 0.0
	totalDeviation := 0.0
	for _, month := range months {
		amounts := incomeFlowTypeAmounts(month.ByType)
		for index := range typeTotals {
			typeTotals[index] += amounts[index]
		}
		totalActual += month.Actual
		totalProjected += month.Projected
		totalDeviation += month.Deviation

		monthRows = append(monthRows, ReportRow{
			Cells: []ReportCell{
				TextCell(incomeFlowMonthLabel(month.Month)),
				MoneyCell(amounts[0], incomeFlowReportCurrency),
				MoneyCell(amounts[1], incomeFlowReportCurrency),
				MoneyCell(amounts[2], incomeFlowReportCurrency),
				MoneyCell(amounts[3], incomeFlowReportCurrency),
				MoneyCell(amounts[4], incomeFlowReportCurrency),
				MoneyCell(month.Actual, incomeFlowReportCurrency),
				MoneyCell(month.Projected, incomeFlowReportCurrency),
				MoneyCell(month.Deviation, incomeFlowReportCurrency),
				PercentCell(month.Effectiveness / 100),
			},
		})
	}
	document.Sections = append(document.Sections, ReportSection{
		Title: "Flujo de Ingresos Mensual",
		Kind:  ReportSectionTable,
		Headers: []string{
			"Mes", "Reservación", "Prima", "Cuota", "Abono a Capital",
			"Pago Total", "Total Cobrado", "Proyectado", "Desviación", "Efectividad",
		},
		Rows: monthRows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				MoneyCell(typeTotals[0], incomeFlowReportCurrency),
				MoneyCell(typeTotals[1], incomeFlowReportCurrency),
				MoneyCell(typeTotals[2], incomeFlowReportCurrency),
				MoneyCell(typeTotals[3], incomeFlowReportCurrency),
				MoneyCell(typeTotals[4], incomeFlowReportCurrency),
				MoneyCell(totalActual, incomeFlowReportCurrency),
				MoneyCell(totalProjected, incomeFlowReportCurrency),
				MoneyCell(totalDeviation, incomeFlowReportCurrency),
				PercentCell(models.Effectiveness(totalActual, totalProjected) / 100),
			},
			Kind: ReportRowTotal,
		},
	})

	// 2. Collections per project.
	projectRows := make([]ReportRow, 0, len(byProject))
	projectCount := 0
	projectTotal := 0.0
	for _, project := range byProject {
		projectCount += project.Count
		projectTotal += project.Amount
		projectRows = append(projectRows, ReportRow{
			Cells: []ReportCell{
				TextCell(project.ProjectName),
				IntCell(project.Count),
				MoneyCell(project.Amount, incomeFlowReportCurrency),
			},
		})
	}
	document.Sections = append(document.Sections, ReportSection{
		Title:   "Cobranzas por Proyecto",
		Kind:    ReportSectionTable,
		Headers: []string{"Proyecto", "Pagos", "Monto"},
		Rows:    projectRows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				IntCell(projectCount),
				MoneyCell(projectTotal, incomeFlowReportCurrency),
			},
			Kind: ReportRowTotal,
		},
	})

	// 3. Accounting movements.
	document.Sections = append(document.Sections, incomeFlowLedgerSection(ledgerTotals))

	// 4. Fiscal collection detail.
	document.Sections = append(document.Sections, detail)

	return document, nil
}
