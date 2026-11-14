package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// operationsFakeAnalyticsRepository is a small analytics fake that embeds the
// repository interface, so only the methods used by the report builders need
// an implementation. It is shared by the operations and income flow builder
// tests.
type operationsFakeAnalyticsRepository struct {
	repository.AnalyticsRepository

	metrics      models.ContractPeriodMetrics
	sales        models.FinancingSales
	collections  models.CollectionsSummary
	ledger       []models.LedgerTypeTotal
	portfolio    models.PortfolioSnapshot
	distribution *models.LotDistribution
	flow         []models.IncomeFlowMonth
	byProject    []models.ProjectCollections
	err          error

	metricsSellerID   *uint
	collectionsSeller *uint
	flowSellerID      *uint
	projectSellerID   *uint
}

func (f *operationsFakeAnalyticsRepository) GetContractPeriodMetrics(_ context.Context, _ *uint, sellerID *uint, _, _ *time.Time) (models.ContractPeriodMetrics, error) {
	f.metricsSellerID = sellerID
	if f.err != nil {
		return models.ContractPeriodMetrics{}, f.err
	}
	return f.metrics, nil
}

func (f *operationsFakeAnalyticsRepository) GetFinancingSales(_ context.Context, _ *uint, _ *uint, _, _ *time.Time) (models.FinancingSales, error) {
	if f.err != nil {
		return models.FinancingSales{}, f.err
	}
	return f.sales, nil
}

func (f *operationsFakeAnalyticsRepository) GetCollectionsSummary(_ context.Context, _ *uint, sellerID *uint, _, _ *time.Time) (models.CollectionsSummary, error) {
	f.collectionsSeller = sellerID
	if f.err != nil {
		return models.CollectionsSummary{}, f.err
	}
	return f.collections, nil
}

func (f *operationsFakeAnalyticsRepository) GetLedgerTotalsByEntryType(_ context.Context, _ *uint, _ *uint, _, _ *time.Time) ([]models.LedgerTypeTotal, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ledger, nil
}

func (f *operationsFakeAnalyticsRepository) GetPortfolioSnapshot(_ context.Context, _ *uint, _ *uint) (models.PortfolioSnapshot, error) {
	if f.err != nil {
		return models.PortfolioSnapshot{}, f.err
	}
	return f.portfolio, nil
}

func (f *operationsFakeAnalyticsRepository) GetLotDistribution(_ context.Context, _ *uint) (*models.LotDistribution, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.distribution, nil
}

func (f *operationsFakeAnalyticsRepository) GetIncomeFlowMonthly(_ context.Context, _ *uint, sellerID *uint, _, _ *time.Time) ([]models.IncomeFlowMonth, error) {
	f.flowSellerID = sellerID
	if f.err != nil {
		return nil, f.err
	}
	return f.flow, nil
}

func (f *operationsFakeAnalyticsRepository) GetCollectionsByProject(_ context.Context, sellerID *uint, _, _ *time.Time) ([]models.ProjectCollections, error) {
	f.projectSellerID = sellerID
	if f.err != nil {
		return nil, f.err
	}
	return f.byProject, nil
}

// operationsReportSection returns the section with the given title, failing the
// test when it is absent.
func operationsReportSection(t *testing.T, document *ReportDocument, title string) ReportSection {
	t.Helper()
	for _, section := range document.Sections {
		if section.Title == title {
			return section
		}
	}
	require.FailNow(t, "report section not found", title)
	return ReportSection{}
}

// operationsMoneyValue unwraps the money value of a cell, failing the test when
// the cell is not monetary.
func operationsMoneyValue(t *testing.T, cell ReportCell) float64 {
	t.Helper()
	require.NotNil(t, cell.Money, "expected a money cell")
	return *cell.Money
}

func TestBuildOperationsReportEmptyPeriod(t *testing.T) {
	repo := &operationsFakeAnalyticsRepository{
		distribution: &models.LotDistribution{},
	}
	service := NewReportExportService(repo, nil, nil, nil, nil)

	document, err := service.BuildOperationsReport(context.Background(), ReportExportFilters{})

	require.NoError(t, err)
	require.NotNil(t, document)
	assert.Equal(t, ReportTitleOperations, document.Title)
	assert.Equal(t, ReportBaseOperations, document.BaseName)
	assert.Equal(t, "HNL", document.Currency)
	assert.NotEmpty(t, document.PeriodKey)
	require.Len(t, document.Sections, 7, "no seller note for an admin report")

	summary := operationsReportSection(t, document, "Resumen de Operaciones del Período")
	require.Len(t, summary.Rows, 5)
	assert.Equal(t, "Contratos aprobados", summary.Rows[0].Cells[0].Text)
	require.NotNil(t, summary.Rows[0].Cells[1].Int)
	assert.Zero(t, *summary.Rows[0].Cells[1].Int)
	assert.Zero(t, operationsMoneyValue(t, summary.Rows[4].Cells[1]))

	financing := operationsReportSection(t, document, "Ventas por Tipo de Financiamiento")
	assert.Empty(t, financing.Rows)
	require.NotNil(t, financing.Totals)
	require.NotNil(t, financing.Totals.Cells[1].Int)
	assert.Zero(t, *financing.Totals.Cells[1].Int)

	collections := operationsReportSection(t, document, "Cobranzas del Período")
	require.NotNil(t, collections.Rows[3].Cells[1].Percent)
	assert.Zero(t, *collections.Rows[3].Cells[1].Percent, "zero projections yield zero effectiveness")

	ledger := operationsReportSection(t, document, "Movimientos del Libro (Ledger)")
	require.Len(t, ledger.Rows, 6)
	require.NotNil(t, ledger.Totals)
	assert.Zero(t, operationsMoneyValue(t, ledger.Totals.Cells[1]))

	portfolio := operationsReportSection(t, document, "Estado de Cartera")
	assert.Zero(t, operationsMoneyValue(t, portfolio.Rows[0].Cells[1]))
	require.NotNil(t, portfolio.Rows[1].Cells[1].Int)
	assert.Zero(t, *portfolio.Rows[1].Cells[1].Int)

	distribution := operationsReportSection(t, document, "Distribución de Lotes")
	require.NotNil(t, distribution.Totals)
	assert.Zero(t, *distribution.Totals.Cells[1].Int)
}

func TestBuildOperationsReportEffectivenessAndLedgerTotals(t *testing.T) {
	repo := &operationsFakeAnalyticsRepository{
		metrics: models.ContractPeriodMetrics{
			Approved:   4,
			Closed:     2,
			Cancelled:  1,
			Rejected:   1,
			SalesValue: 900000,
		},
		sales: models.FinancingSales{
			Metrics: []models.FinancingSalesRow{
				{FinancingType: models.FinancingTypeDirect, Count: 2, Value: 500000},
				{FinancingType: models.FinancingTypeBank, Count: 1, Value: 300000},
				{FinancingType: models.FinancingTypeCash, Count: 1, Value: 100000},
			},
			TotalCount: 4,
			TotalValue: 900000,
		},
		collections: models.CollectionsSummary{
			ByType: []models.CollectionTypeRow{
				{PaymentType: models.PaymentTypeInstallment, Count: 3, Amount: 600},
				{PaymentType: models.PaymentTypeDownPayment, Count: 1, Amount: 150},
			},
			Total:             750,
			InterestCollected: 75,
			ProjectedDue:      1000,
			Effectiveness:     75,
		},
		ledger: []models.LedgerTypeTotal{
			{EntryType: models.EntryTypeInitial, Total: -1500},
			{EntryType: models.EntryTypeInterest, Total: -30},
			{EntryType: models.EntryTypePayment, Total: 400},
			{EntryType: models.EntryTypePrepayment, Total: 100},
			{EntryType: models.EntryTypeAdjustment, Total: 20},
			{EntryType: models.EntryTypeRefund, Total: -50},
		},
		portfolio: models.PortfolioSnapshot{OutstandingBalance: 760, SettledContracts: 2},
		distribution: &models.LotDistribution{
			TotalLots: 10, Financed: 4, FullyPaid: 2, Reserved: 1, Available: 3,
		},
	}
	service := NewReportExportService(repo, nil, nil, nil, nil)

	document, err := service.BuildOperationsReport(context.Background(), ReportExportFilters{})

	require.NoError(t, err)

	financing := operationsReportSection(t, document, "Ventas por Tipo de Financiamiento")
	require.Len(t, financing.Rows, 3)
	assert.Equal(t, "Directo", financing.Rows[0].Cells[0].Text)
	assert.Equal(t, "Bancario", financing.Rows[1].Cells[0].Text)
	assert.Equal(t, "Contado", financing.Rows[2].Cells[0].Text)
	require.NotNil(t, financing.Totals)
	assert.Equal(t, 4, *financing.Totals.Cells[1].Int)
	assert.Equal(t, 900000.0, operationsMoneyValue(t, financing.Totals.Cells[2]))

	collections := operationsReportSection(t, document, "Cobranzas del Período")
	require.NotNil(t, collections.Rows[3].Cells[1].Percent)
	assert.InDelta(t, 0.75, *collections.Rows[3].Cells[1].Percent, 0.0001)
	assert.Equal(t, 75.0, operationsMoneyValue(t, collections.Rows[1].Cells[1]))

	byType := operationsReportSection(t, document, "Cobranzas por Tipo de Pago")
	require.Len(t, byType.Rows, 2)
	assert.Equal(t, "Cuota", byType.Rows[0].Cells[0].Text)
	assert.Equal(t, "Prima", byType.Rows[1].Cells[0].Text)
	require.NotNil(t, byType.Totals)
	assert.Equal(t, 4, *byType.Totals.Cells[1].Int)
	assert.Equal(t, 750.0, operationsMoneyValue(t, byType.Totals.Cells[2]))

	ledger := operationsReportSection(t, document, "Movimientos del Libro (Ledger)")
	require.Len(t, ledger.Rows, 6)
	labels := make([]string, 0, len(ledger.Rows))
	signedSum := 0.0
	for _, row := range ledger.Rows {
		labels = append(labels, row.Cells[0].Text)
		signedSum += operationsMoneyValue(t, row.Cells[1])
	}
	assert.Equal(t, []string{
		"Saldo inicial", "Interés", "Pagos", "Prepagos", "Ajustes", "Devoluciones",
	}, labels)
	assert.Equal(t, -1060.0, signedSum)
	require.NotNil(t, ledger.Totals)
	assert.Equal(t, signedSum, operationsMoneyValue(t, ledger.Totals.Cells[1]))
	assert.Equal(t, "Los montos negativos aumentan la deuda; los positivos la reducen.", ledger.Note)
}

func TestBuildOperationsReportSellerScope(t *testing.T) {
	repo := &operationsFakeAnalyticsRepository{
		distribution: &models.LotDistribution{TotalLots: 2, Available: 2},
	}
	service := NewReportExportService(repo, nil, nil, nil, nil)

	document, err := service.BuildOperationsReport(context.Background(), ReportExportFilters{
		SellerID:       9,
		FilterBySeller: true,
	})

	require.NoError(t, err)
	require.NotNil(t, repo.metricsSellerID)
	assert.Equal(t, uint(9), *repo.metricsSellerID)
	require.NotNil(t, repo.collectionsSeller)
	assert.Equal(t, uint(9), *repo.collectionsSeller)

	require.Len(t, document.Sections, 8)
	note := document.Sections[len(document.Sections)-1]
	assert.Equal(t, ReportSectionNote, note.Kind)
	assert.Equal(t, "Reporte filtrado a las operaciones del vendedor.", note.Note)
}
