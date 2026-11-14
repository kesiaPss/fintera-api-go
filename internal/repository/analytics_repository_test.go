package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildIncomeFlowMonths(t *testing.T) {
	january := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	february := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	months := []time.Time{january, february, march}
	actual := []monthlyCollectionRow{
		{Month: january, PaymentType: models.PaymentTypeDownPayment, Count: 1, Amount: 500},
		{Month: january, PaymentType: models.PaymentTypeInstallment, Count: 2, Amount: 1000},
		{Month: march, PaymentType: models.PaymentTypeInstallment, Count: 1, Amount: 300},
	}
	projected := []monthlyProjectionRow{
		{Month: january, Total: 3000},
		{Month: january, Total: 500},
		{Month: march, Total: 300},
	}

	flow := buildIncomeFlowMonths(months, actual, projected)

	require.Len(t, flow, 3)

	// January: 1500 collected against 3500 projected.
	assert.Equal(t, january, flow[0].Month)
	assert.Equal(t, 1500.0, flow[0].Actual)
	assert.Equal(t, 3500.0, flow[0].Projected)
	assert.Equal(t, -2000.0, flow[0].Deviation)
	assert.InDelta(t, 42.86, flow[0].Effectiveness, 0.001)
	require.Len(t, flow[0].ByType, 2)
	assert.Equal(t, models.PaymentTypeDownPayment, flow[0].ByType[0].PaymentType)
	assert.Equal(t, 1, flow[0].ByType[0].Count)
	assert.Equal(t, 500.0, flow[0].ByType[0].Amount)

	// February has no data but stays in the flow with zero values.
	assert.Equal(t, february, flow[1].Month)
	assert.Zero(t, flow[1].Actual)
	assert.Zero(t, flow[1].Projected)
	assert.Zero(t, flow[1].Deviation)
	assert.Zero(t, flow[1].Effectiveness)
	assert.Empty(t, flow[1].ByType)

	// March is fully collected.
	assert.Equal(t, march, flow[2].Month)
	assert.Equal(t, 300.0, flow[2].Actual)
	assert.Equal(t, 300.0, flow[2].Projected)
	assert.Zero(t, flow[2].Deviation)
	assert.Equal(t, 100.0, flow[2].Effectiveness)
}

func TestIncomeFlowMonthRange(t *testing.T) {
	january := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	t.Run("uses the requested period", func(t *testing.T) {
		start := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

		months := incomeFlowMonthRange(&start, &end, nil, nil)

		require.Len(t, months, 3)
		assert.Equal(t, january, months[0])
		assert.Equal(t, march, months[2])
	})

	t.Run("falls back to the observed data range", func(t *testing.T) {
		actual := []monthlyCollectionRow{
			{Month: march, PaymentType: models.PaymentTypeInstallment, Amount: 10},
			{Month: january, PaymentType: models.PaymentTypeInstallment, Amount: 20},
		}

		months := incomeFlowMonthRange(nil, nil, actual, nil)

		require.Len(t, months, 3)
		assert.Equal(t, january, months[0])
		assert.Equal(t, march, months[2])
	})

	t.Run("no period and no data returns nothing", func(t *testing.T) {
		assert.Empty(t, incomeFlowMonthRange(nil, nil, nil, nil))
	})
}

func TestGetCollectionsSummary_ZeroProjections(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	mock.ExpectQuery(`SELECT payments\.payment_type AS payment_type`).
		WithArgs(models.PaymentStatusPaid).
		WillReturnRows(sqlmock.NewRows([]string{"payment_type", "count", "amount", "interest"}))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(payments\.amount\), 0\) FROM "payments"`).
		WithArgs(models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(0))

	summary, err := repo.GetCollectionsSummary(context.Background(), nil, nil, nil, nil)

	require.NoError(t, err)
	assert.Empty(t, summary.ByType)
	assert.Zero(t, summary.Total)
	assert.Zero(t, summary.InterestCollected)
	assert.Zero(t, summary.ProjectedDue)
	assert.Zero(t, summary.Effectiveness)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionsSummary_Effectiveness(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	mock.ExpectQuery(`SELECT payments\.payment_type AS payment_type`).
		WithArgs(models.PaymentStatusPaid).
		WillReturnRows(sqlmock.NewRows([]string{"payment_type", "count", "amount", "interest"}).
			AddRow(models.PaymentTypeInstallment, 2, 500, 25))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(payments\.amount\), 0\) FROM "payments"`).
		WithArgs(models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(1000))

	summary, err := repo.GetCollectionsSummary(context.Background(), nil, nil, nil, nil)

	require.NoError(t, err)
	require.Len(t, summary.ByType, 1)
	assert.Equal(t, 500.0, summary.Total)
	assert.Equal(t, 25.0, summary.InterestCollected)
	assert.Equal(t, 1000.0, summary.ProjectedDue)
	assert.Equal(t, 50.0, summary.Effectiveness)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPortfolioSnapshot(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	expectedQuery := `SELECT COALESCE(SUM(CASE WHEN contracts.balance < 0 THEN contracts.balance ELSE 0 END), 0) AS outstanding_debt, ` +
		`COALESCE(SUM(CASE WHEN contracts.balance >= 0 THEN 1 ELSE 0 END), 0) AS settled_contracts ` +
		`FROM "contracts" WHERE contracts.status IN ($1,$2)`
	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(models.ContractStatusApproved, models.ContractStatusClosed).
		WillReturnRows(sqlmock.NewRows([]string{"outstanding_debt", "settled_contracts"}).
			AddRow(-1500000, 3))

	snapshot, err := repo.GetPortfolioSnapshot(context.Background(), nil, nil)

	require.NoError(t, err)
	// Debt is stored as a negative balance; the snapshot exposes the positive
	// outstanding magnitude.
	assert.Equal(t, 1500000.0, snapshot.OutstandingBalance)
	assert.Equal(t, 3, snapshot.SettledContracts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetContractPeriodMetrics(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	projectID := uint(4)

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(CASE WHEN contracts\.approved_at IS NOT NULL`).
		WithArgs(start, end, start, end, models.ContractStatusCancelled, start, end, start, end, start, end, projectID).
		WillReturnRows(sqlmock.NewRows([]string{"approved_count", "closed_count", "cancelled_count", "rejected_count", "sales_value"}).
			AddRow(5, 2, 1, 1, 750000))

	metrics, err := repo.GetContractPeriodMetrics(context.Background(), &projectID, nil, &start, &end)

	require.NoError(t, err)
	assert.Equal(t, 5, metrics.Approved)
	assert.Equal(t, 2, metrics.Closed)
	assert.Equal(t, 1, metrics.Cancelled)
	assert.Equal(t, 1, metrics.Rejected)
	assert.Equal(t, 750000.0, metrics.SalesValue)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetContractPeriodMetrics_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)
	sellerID := uint(9)

	mock.ExpectQuery(`SELECT COALESCE\(SUM\(CASE WHEN contracts\.approved_at IS NOT NULL`).
		WithArgs(start, end, start, end, models.ContractStatusCancelled, start, end, start, end, start, end, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"approved_count", "closed_count", "cancelled_count", "rejected_count", "sales_value"}).
			AddRow(2, 1, 0, 0, 250000))

	metrics, err := repo.GetContractPeriodMetrics(context.Background(), nil, &sellerID, &start, &end)

	require.NoError(t, err)
	assert.Equal(t, 2, metrics.Approved)
	assert.Equal(t, 250000.0, metrics.SalesValue)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetFinancingSales_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`SELECT contracts\.financing_type AS financing_type.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?.*GROUP BY .*contracts.*financing_type`).
		WithArgs(sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"financing_type", "count", "value"}).
			AddRow(models.FinancingTypeDirect, 2, 500000).
			AddRow(models.FinancingTypeBank, 1, 250000))

	sales, err := repo.GetFinancingSales(context.Background(), nil, &sellerID, nil, nil)

	require.NoError(t, err)
	require.Len(t, sales.Metrics, 2)
	assert.Equal(t, 3, sales.TotalCount)
	assert.Equal(t, 750000.0, sales.TotalValue)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionsSummary_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`SELECT payments\.payment_type AS payment_type.*JOIN contracts ON contracts\.id = payments\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?`).
		WithArgs(models.PaymentStatusPaid, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"payment_type", "count", "amount", "interest"}).
			AddRow(models.PaymentTypeInstallment, 1, 400, 20))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(payments\.amount\), 0\) FROM "payments".*JOIN contracts ON contracts\.id = payments\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?`).
		WithArgs(models.PaymentStatusPending, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(800))

	summary, err := repo.GetCollectionsSummary(context.Background(), nil, &sellerID, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, 400.0, summary.Total)
	assert.Equal(t, 800.0, summary.ProjectedDue)
	assert.Equal(t, 50.0, summary.Effectiveness)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetLedgerTotalsByEntryType_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`FROM "contract_ledger_entries".*JOIN contracts ON contracts\.id = contract_ledger_entries\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?.*GROUP BY .*contract_ledger_entries.*entry_type`).
		WithArgs(sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"entry_type", "total"}).
			AddRow(models.EntryTypePayment, 1200).
			AddRow(models.EntryTypeInitial, -5000))

	totals, err := repo.GetLedgerTotalsByEntryType(context.Background(), nil, &sellerID, nil, nil)

	require.NoError(t, err)
	require.Len(t, totals, len(ledgerEntryTypeOrder))
	assert.Equal(t, -5000.0, ledgerTotalFor(t, totals, models.EntryTypeInitial))
	assert.Equal(t, 1200.0, ledgerTotalFor(t, totals, models.EntryTypePayment))
	assert.Zero(t, ledgerTotalFor(t, totals, models.EntryTypeRefund))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPortfolioSnapshot_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`FROM "contracts" WHERE contracts\.status IN \(\$1,\$2\) AND \(\(contracts\.seller_id = \$3 OR contracts\.creator_id = \$4\)\)`).
		WithArgs(models.ContractStatusApproved, models.ContractStatusClosed, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"outstanding_debt", "settled_contracts"}).
			AddRow(-900000, 2))

	snapshot, err := repo.GetPortfolioSnapshot(context.Background(), nil, &sellerID)

	require.NoError(t, err)
	assert.Equal(t, 900000.0, snapshot.OutstandingBalance)
	assert.Equal(t, 2, snapshot.SettledContracts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetIncomeFlowMonthly_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`SELECT DATE_TRUNC\('month', payments\.payment_date\) AS month.*JOIN contracts ON contracts\.id = payments\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?`).
		WithArgs(models.PaymentStatusPaid, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"month", "payment_type", "count", "amount"}))
	mock.ExpectQuery(`SELECT DATE_TRUNC\('month', payments\.due_date\) AS month.*JOIN contracts ON contracts\.id = payments\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?`).
		WithArgs(models.PaymentStatusPending, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"month", "total"}))

	flow, err := repo.GetIncomeFlowMonthly(context.Background(), nil, &sellerID, nil, nil)

	require.NoError(t, err)
	assert.Empty(t, flow)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCollectionsByProject_SellerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	sellerID := uint(9)

	mock.ExpectQuery(`JOIN contracts ON contracts\.id = payments\.contract_id.*\(\(?contracts\.seller_id = \$\d+ OR contracts\.creator_id = \$\d+\)\)?.*GROUP BY lots\.project_id, projects\.name`).
		WithArgs(models.PaymentStatusPaid, sellerID, sellerID).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "project_name", "amount", "count"}).
			AddRow(3, "Proyecto Tres", 450000, 4))

	collections, err := repo.GetCollectionsByProject(context.Background(), &sellerID, nil, nil)

	require.NoError(t, err)
	require.Len(t, collections, 1)
	assert.Equal(t, uint(3), collections[0].ProjectID)
	assert.Equal(t, "Proyecto Tres", collections[0].ProjectName)
	assert.Equal(t, 450000.0, collections[0].Amount)
	assert.Equal(t, 4, collections[0].Count)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ledgerTotalFor returns the total of an entry type from a ledger breakdown.
func ledgerTotalFor(t *testing.T, totals []models.LedgerTypeTotal, entryType string) float64 {
	t.Helper()
	for _, total := range totals {
		if total.EntryType == entryType {
			return total.Total
		}
	}
	require.FailNow(t, "entry type not found in ledger breakdown", entryType)
	return 0
}

func TestFindOverdueForActiveContractsDetailed(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewOverdueReportRepository(db)

	mock.ExpectQuery(`JOIN contracts ON contracts\.id = payments\.contract_id AND contracts\.status IN \(\$1,\$2\) AND contracts\.active = \$3`).
		WithArgs(models.ContractStatusApproved, models.ContractStatusSigned, true, models.PaymentStatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))

	payments, err := repo.FindOverdueForActiveContractsDetailed(context.Background())

	require.NoError(t, err)
	assert.Empty(t, payments)
	require.NoError(t, mock.ExpectationsWereMet())
}
