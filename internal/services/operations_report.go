package services

import (
	"context"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
)

// operationsReportCurrency is the currency used by the aggregates of the
// operations report. The analytics aggregations sum contracts and payments
// regardless of the currency of each row, so the export reports every
// aggregate in HNL. Splitting by currency would require currency-aware
// repository aggregations, which this change does not introduce.
const operationsReportCurrency = "HNL"

// operationsSellerScope returns the seller id that scopes the operations
// report, or nil for admin callers (and for sellers without an id).
func operationsSellerScope(filters ReportExportFilters) *uint {
	if !filters.FilterBySeller || filters.SellerID == 0 {
		return nil
	}
	sellerID := filters.SellerID
	return &sellerID
}

// operationsLedgerOrder is the fixed order of the ledger entry types in the
// operations report.
var operationsLedgerOrder = []string{
	models.EntryTypeInitial,
	models.EntryTypeInterest,
	models.EntryTypePayment,
	models.EntryTypePrepayment,
	models.EntryTypeAdjustment,
	models.EntryTypeRefund,
}

// operationsLedgerLabels maps ledger entry types to their Spanish labels.
var operationsLedgerLabels = map[string]string{
	models.EntryTypeInitial:    "Saldo inicial",
	models.EntryTypeInterest:   "Interés",
	models.EntryTypePayment:    "Pagos",
	models.EntryTypePrepayment: "Prepagos",
	models.EntryTypeAdjustment: "Ajustes",
	models.EntryTypeRefund:     "Devoluciones",
}

// operationsLedgerLabel returns the Spanish label of an entry type, falling
// back to the raw value so unexpected types are never dropped silently.
func operationsLedgerLabel(entryType string) string {
	if label, ok := operationsLedgerLabels[entryType]; ok {
		return label
	}
	return entryType
}

// operationsLedgerSection builds the signed ledger movement table. Amounts are
// printed exactly as the authoritative ledger stores them: negative entries
// increase debt and positive entries reduce it.
func operationsLedgerSection(totals []models.LedgerTypeTotal) ReportSection {
	amounts := make(map[string]float64, len(totals))
	for _, total := range totals {
		amounts[total.EntryType] = total.Total
	}

	ordered := make([]string, 0, len(totals)+len(operationsLedgerOrder))
	seen := make(map[string]bool, len(totals))
	for _, entryType := range operationsLedgerOrder {
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
				TextCell(operationsLedgerLabel(entryType)),
				MoneyCell(amount, operationsReportCurrency),
			},
		})
	}

	return ReportSection{
		Title:   "Movimientos del Libro (Ledger)",
		Kind:    ReportSectionTable,
		Headers: []string{"Tipo de Movimiento", "Monto"},
		Rows:    rows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				MoneyCell(net, operationsReportCurrency),
			},
			Kind: ReportRowTotal,
		},
		Note: "Los montos negativos aumentan la deuda; los positivos la reducen.",
	}
}

// BuildOperationsReport assembles the monthly operations report: contract
// activity, sales by financing type, collections and effectiveness, signed
// ledger movements, portfolio balance and lot distribution.
func (s *ReportExportService) BuildOperationsReport(ctx context.Context, filters ReportExportFilters) (*ReportDocument, error) {
	generatedAt := time.Now()
	periodLabel, periodKey := reportPeriodLabel(filters.StartDate, filters.EndDate, generatedAt)
	sellerID := operationsSellerScope(filters)

	metrics, err := s.analyticsRepo.GetContractPeriodMetrics(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudo obtener el resumen de contratos: %w", err)
	}

	sales, err := s.analyticsRepo.GetFinancingSales(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las ventas por financiamiento: %w", err)
	}

	collections, err := s.analyticsRepo.GetCollectionsSummary(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener las cobranzas: %w", err)
	}

	ledgerTotals, err := s.analyticsRepo.GetLedgerTotalsByEntryType(ctx, filters.ProjectID, sellerID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron obtener los movimientos del libro: %w", err)
	}

	portfolio, err := s.analyticsRepo.GetPortfolioSnapshot(ctx, filters.ProjectID, sellerID)
	if err != nil {
		return nil, fmt.Errorf("no se pudo obtener el estado de cartera: %w", err)
	}

	distribution, err := s.analyticsRepo.GetLotDistribution(ctx, filters.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("no se pudo obtener la distribución de lotes: %w", err)
	}
	if distribution == nil {
		distribution = &models.LotDistribution{}
	}

	document := &ReportDocument{
		Title:       ReportTitleOperations,
		Subtitle:    "Resumen mensual de operaciones",
		Period:      periodLabel,
		PeriodKey:   periodKey,
		BaseName:    ReportBaseOperations,
		GeneratedAt: generatedAt,
		Currency:    operationsReportCurrency,
	}

	// 1. Period operations summary.
	document.Sections = append(document.Sections, ReportSection{
		Title: "Resumen de Operaciones del Período",
		Kind:  ReportSectionSummary,
		Rows: []ReportRow{
			{Cells: []ReportCell{TextCell("Contratos aprobados"), IntCell(metrics.Approved)}},
			{Cells: []ReportCell{TextCell("Contratos cerrados"), IntCell(metrics.Closed)}},
			{Cells: []ReportCell{TextCell("Contratos cancelados"), IntCell(metrics.Cancelled)}},
			{Cells: []ReportCell{TextCell("Contratos rechazados"), IntCell(metrics.Rejected)}},
			{Cells: []ReportCell{TextCell("Valor total de ventas"), MoneyCell(metrics.SalesValue, operationsReportCurrency)}},
		},
	})

	// 2. Sales by financing type.
	financingRows := make([]ReportRow, 0, len(sales.Metrics))
	for _, row := range sales.Metrics {
		financingRows = append(financingRows, ReportRow{
			Cells: []ReportCell{
				TextCell(TranslateFinancingType(row.FinancingType)),
				IntCell(row.Count),
				MoneyCell(row.Value, operationsReportCurrency),
			},
		})
	}
	document.Sections = append(document.Sections, ReportSection{
		Title:   "Ventas por Tipo de Financiamiento",
		Kind:    ReportSectionTable,
		Headers: []string{"Tipo de Financiamiento", "Contratos", "Valor"},
		Rows:    financingRows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				IntCell(sales.TotalCount),
				MoneyCell(sales.TotalValue, operationsReportCurrency),
			},
			Kind: ReportRowTotal,
		},
	})

	// 3. Period collections summary.
	document.Sections = append(document.Sections, ReportSection{
		Title: "Cobranzas del Período",
		Kind:  ReportSectionSummary,
		Rows: []ReportRow{
			{Cells: []ReportCell{TextCell("Total cobrado"), MoneyCell(collections.Total, operationsReportCurrency)}},
			{Cells: []ReportCell{TextCell("Intereses cobrados"), MoneyCell(collections.InterestCollected, operationsReportCurrency)}},
			{Cells: []ReportCell{TextCell("Proyectado del período"), MoneyCell(collections.ProjectedDue, operationsReportCurrency)}},
			{Cells: []ReportCell{TextCell("Efectividad"), PercentCell(collections.Effectiveness / 100)}},
		},
	})

	collectionRows := make([]ReportRow, 0, len(collections.ByType))
	collectionCount := 0
	for _, row := range collections.ByType {
		collectionCount += row.Count
		collectionRows = append(collectionRows, ReportRow{
			Cells: []ReportCell{
				TextCell(TranslatePaymentType(row.PaymentType)),
				IntCell(row.Count),
				MoneyCell(row.Amount, operationsReportCurrency),
			},
		})
	}
	document.Sections = append(document.Sections, ReportSection{
		Title:   "Cobranzas por Tipo de Pago",
		Kind:    ReportSectionTable,
		Headers: []string{"Tipo de Pago", "Pagos", "Monto"},
		Rows:    collectionRows,
		Totals: &ReportRow{
			Cells: []ReportCell{
				TextCell("Total"),
				IntCell(collectionCount),
				MoneyCell(collections.Total, operationsReportCurrency),
			},
			Kind: ReportRowTotal,
		},
	})

	// 4. Ledger movements.
	document.Sections = append(document.Sections, operationsLedgerSection(ledgerTotals))

	// 5. Portfolio snapshot: the repository already returns the outstanding
	// balance as a positive debt magnitude.
	document.Sections = append(document.Sections, ReportSection{
		Title: "Estado de Cartera",
		Kind:  ReportSectionSummary,
		Rows: []ReportRow{
			{Cells: []ReportCell{TextCell("Saldo pendiente total"), MoneyCell(portfolio.OutstandingBalance, operationsReportCurrency)}},
			{Cells: []ReportCell{TextCell("Contratos saldados"), IntCell(portfolio.SettledContracts)}},
		},
	})

	// 6. Lot distribution.
	document.Sections = append(document.Sections, ReportSection{
		Title:   "Distribución de Lotes",
		Kind:    ReportSectionTable,
		Headers: []string{"Estado", "Cantidad"},
		Rows: []ReportRow{
			{Cells: []ReportCell{TextCell("Financiado"), IntCell(distribution.Financed)}},
			{Cells: []ReportCell{TextCell("Pagado Totalmente"), IntCell(distribution.FullyPaid)}},
			{Cells: []ReportCell{TextCell("Reservado"), IntCell(distribution.Reserved)}},
			{Cells: []ReportCell{TextCell("Disponible"), IntCell(distribution.Available)}},
		},
		Totals: &ReportRow{
			Cells: []ReportCell{TextCell("Total"), IntCell(distribution.TotalLots)},
			Kind:  ReportRowTotal,
		},
	})

	if sellerID != nil {
		document.Sections = append(document.Sections, ReportSection{
			Title: "Nota",
			Kind:  ReportSectionNote,
			Note:  "Reporte filtrado a las operaciones del vendedor.",
		})
	}

	return document, nil
}
