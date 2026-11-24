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

// incomeFlowFakePaymentRepository is a small payment fake that embeds the
// repository interface and records the scope received by the report builder's
// batched paid-payment stream. It emulates the SQL scope/date predicates in
// Go so fixtures stay declarative.
type incomeFlowFakePaymentRepository struct {
	repository.PaymentRepository

	payments []models.Payment
	err      error

	streamed             bool
	capturedStart        *time.Time
	capturedEnd          *time.Time
	capturedProjectID    *uint
	capturedSellerID     uint
	capturedFilterSeller bool
	capturedBatchSize    int
}

func (f *incomeFlowFakePaymentRepository) StreamPaidForReport(
	_ context.Context, start, end *time.Time, projectID *uint,
	sellerID uint, filterBySeller bool, batchSize int,
	fn func(batch []models.Payment) error,
) error {
	f.streamed = true
	f.capturedStart, f.capturedEnd, f.capturedProjectID = start, end, projectID
	f.capturedSellerID, f.capturedFilterSeller, f.capturedBatchSize = sellerID, filterBySeller, batchSize

	if f.err != nil {
		return f.err
	}

	selected := make([]models.Payment, 0, len(f.payments))
	for _, payment := range f.payments {
		if projectID != nil && payment.Contract.Lot.ProjectID != *projectID {
			continue
		}
		if filterBySeller && sellerID > 0 {
			sellerMatch := payment.Contract.SellerID != nil && *payment.Contract.SellerID == sellerID
			creatorMatch := payment.Contract.CreatorID != nil && *payment.Contract.CreatorID == sellerID
			if !sellerMatch && !creatorMatch {
				continue
			}
		}
		if start != nil || end != nil {
			date := payment.DueDate
			if payment.PaymentDate != nil {
				date = *payment.PaymentDate
			}
			if start != nil && date.Before(*start) {
				continue
			}
			if end != nil && date.After(*end) {
				continue
			}
		}
		selected = append(selected, payment)
	}

	if batchSize <= 0 {
		batchSize = 1
	}
	for begin := 0; begin < len(selected); begin += batchSize {
		until := begin + batchSize
		if until > len(selected) {
			until = len(selected)
		}
		if err := fn(selected[begin:until]); err != nil {
			return err
		}
	}
	return nil
}

// incomeFlowTestPayment builds a paid payment with the preloaded associations
// the detail section reads.
func incomeFlowTestPayment(contractGUID string, paymentDate time.Time, paymentType string, amount float64) models.Payment {
	sellerID := uint(7)
	paidAmount := amount
	return models.Payment{
		PaymentType: paymentType,
		PaidAmount:  &paidAmount,
		PaymentDate: &paymentDate,
		DueDate:     paymentDate,
		Contract: models.Contract{
			GUID:          contractGUID,
			Currency:      "HNL",
			SellerID:      &sellerID,
			ApplicantUser: models.User{FullName: "Cliente Ejemplo", Identity: "0801199012345", RTN: "08011990123456"},
			Lot: models.Lot{
				Name:      "Lote 5",
				ProjectID: 3,
				Project:   models.Project{ID: 3, Name: "Proyecto Norte"},
			},
		},
	}
}

func TestBuildIncomeFlowReportMapsPaymentTypesByMonth(t *testing.T) {
	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	november := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	repo := &operationsFakeAnalyticsRepository{
		flow: []models.IncomeFlowMonth{
			{
				Month:         october,
				Actual:        1900,
				Projected:     2000,
				Deviation:     -100,
				Effectiveness: 95,
				ByType: []models.CollectionTypeRow{
					{PaymentType: models.PaymentTypeReservation, Count: 1, Amount: 100},
					{PaymentType: models.PaymentTypeDownPayment, Count: 1, Amount: 300},
					{PaymentType: models.PaymentTypeInstallment, Count: 1, Amount: 700},
					{PaymentType: models.PaymentTypeAdvance, Count: 1, Amount: 200},
					{PaymentType: models.PaymentTypeCapitalRepayment, Count: 1, Amount: 100},
					{PaymentType: models.PaymentTypeFull, Count: 1, Amount: 500},
				},
			},
			{
				Month:         november,
				Actual:        0,
				Projected:     0,
				Deviation:     0,
				Effectiveness: 0,
			},
		},
	}
	service := NewReportExportService(repo, nil, &incomeFlowFakePaymentRepository{}, nil, nil)

	document, err := service.BuildIncomeFlowReport(context.Background(), ReportExportFilters{})

	require.NoError(t, err)
	assert.Equal(t, ReportTitleIncomeFlow, document.Title)
	assert.Equal(t, ReportBaseIncomeFlow, document.BaseName)

	section := operationsReportSection(t, document, "Flujo de Ingresos Mensual")
	require.Len(t, section.Rows, 2)
	require.Len(t, section.Headers, 10)

	octoberRow := section.Rows[0].Cells
	assert.Equal(t, "Octubre 2026", octoberRow[0].Text)
	assert.Equal(t, 100.0, operationsMoneyValue(t, octoberRow[1]), "reservation column")
	assert.Equal(t, 300.0, operationsMoneyValue(t, octoberRow[2]), "down payment column")
	assert.Equal(t, 700.0, operationsMoneyValue(t, octoberRow[3]), "installment column")
	assert.Equal(t, 300.0, operationsMoneyValue(t, octoberRow[4]), "advance and capital repayment share one column")
	assert.Equal(t, 500.0, operationsMoneyValue(t, octoberRow[5]), "full payment column")
	assert.Equal(t, 1900.0, operationsMoneyValue(t, octoberRow[6]), "actual total equals the sum of the columns")
	assert.Equal(t, 2000.0, operationsMoneyValue(t, octoberRow[7]))
	assert.Equal(t, -100.0, operationsMoneyValue(t, octoberRow[8]))
	require.NotNil(t, octoberRow[9].Percent)
	assert.InDelta(t, 0.95, *octoberRow[9].Percent, 0.0001)

	novemberRow := section.Rows[1].Cells
	assert.Equal(t, "Noviembre 2026", novemberRow[0].Text)
	assert.Zero(t, operationsMoneyValue(t, novemberRow[6]))
	require.NotNil(t, novemberRow[9].Percent)
	assert.Zero(t, *novemberRow[9].Percent, "a month without projections has zero effectiveness")

	require.NotNil(t, section.Totals)
	assert.Equal(t, 1900.0, operationsMoneyValue(t, section.Totals.Cells[6]))
	assert.Equal(t, 2000.0, operationsMoneyValue(t, section.Totals.Cells[7]))
	require.NotNil(t, section.Totals.Cells[9].Percent)
	assert.InDelta(t, 0.95, *section.Totals.Cells[9].Percent, 0.0001)

	ledger := operationsReportSection(t, document, "Movimientos Contables del Período")
	require.Len(t, ledger.Rows, 6, "all six ledger entry types are always listed")
	assert.Equal(t, "Los montos negativos aumentan la deuda; los positivos la reducen.", ledger.Note)
}

func TestBuildIncomeFlowReportProjectTotals(t *testing.T) {
	repo := &operationsFakeAnalyticsRepository{
		byProject: []models.ProjectCollections{
			{ProjectID: 1, ProjectName: "Proyecto Uno", Amount: 1200, Count: 3},
			{ProjectID: 2, ProjectName: "Proyecto Dos", Amount: 800, Count: 2},
		},
		distribution: &models.LotDistribution{},
	}
	service := NewReportExportService(repo, nil, &incomeFlowFakePaymentRepository{}, nil, nil)

	document, err := service.BuildIncomeFlowReport(context.Background(), ReportExportFilters{})

	require.NoError(t, err)
	section := operationsReportSection(t, document, "Cobranzas por Proyecto")
	require.Len(t, section.Rows, 2)
	assert.Equal(t, "Proyecto Uno", section.Rows[0].Cells[0].Text)
	assert.Equal(t, 1200.0, operationsMoneyValue(t, section.Rows[0].Cells[2]))
	require.NotNil(t, section.Totals)
	assert.Equal(t, 5, *section.Totals.Cells[1].Int)
	assert.Equal(t, 2000.0, operationsMoneyValue(t, section.Totals.Cells[2]))
}

func TestBuildIncomeFlowReportDetailFiscalData(t *testing.T) {
	october := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	september := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	withReference := incomeFlowTestPayment("CTR-002", october, models.PaymentTypeInstallment, 450)
	authorization := "AUTH-123"
	withReference.AuthorizationNumber = &authorization

	withPayer := incomeFlowTestPayment("CTR-001", september, models.PaymentTypeDownPayment, 300)
	payer := "María Pagadora"
	withPayer.PayerAccountName = &payer
	withPayer.PaidAmount = nil // falls back to Amount
	withPayer.Amount = 275

	withoutReference := incomeFlowTestPayment("CTR-003", october, models.PaymentTypeReservation, 100)

	paymentRepo := &incomeFlowFakePaymentRepository{
		payments: []models.Payment{withReference, withoutReference, withPayer},
	}
	service := NewReportExportService(&operationsFakeAnalyticsRepository{}, nil, paymentRepo, nil, nil)

	document, err := service.BuildIncomeFlowReport(context.Background(), ReportExportFilters{})

	require.NoError(t, err)
	require.True(t, paymentRepo.streamed, "the detail must come from the batched stream")
	assert.Nil(t, paymentRepo.capturedStart, "no date filter when the period is unfiltered")
	assert.Nil(t, paymentRepo.capturedEnd)
	assert.Equal(t, repository.PaymentScanBatchSize, paymentRepo.capturedBatchSize)

	section := operationsReportSection(t, document, "Detalle de Cobranzas")
	require.Len(t, section.Rows, 3, "rows are ordered by payment date ascending")

	first := section.Rows[0].Cells
	assert.Equal(t, "CTR-001", first[1].Text)
	assert.Equal(t, "Cliente Ejemplo", first[2].Text)
	assert.Equal(t, "0801199012345", first[3].Text)
	assert.Equal(t, "08011990123456", first[4].Text)
	assert.Equal(t, "María Pagadora", first[5].Text, "payer account name is the fallback bank reference")
	assert.Equal(t, "Prima", first[6].Text)
	assert.Equal(t, "Proyecto Norte", first[7].Text)
	assert.Equal(t, "Lote 5", first[8].Text)
	assert.Equal(t, 275.0, operationsMoneyValue(t, first[9]), "nil paid amount falls back to the scheduled amount")

	second := section.Rows[1].Cells
	assert.Equal(t, "CTR-002", second[1].Text)
	require.NotNil(t, second[0].Date)
	assert.Equal(t, october, *second[0].Date)
	assert.Equal(t, "AUTH-123", second[5].Text)
	assert.Equal(t, 450.0, operationsMoneyValue(t, second[9]))

	third := section.Rows[2].Cells
	assert.Equal(t, "CTR-003", third[1].Text)
	assert.Empty(t, third[5].Text, "a payment without bank reference keeps an empty column")
	assert.Equal(t, "Reservación", third[6].Text)
}

func TestBuildIncomeFlowReportDetailFiltersProjectAndSeller(t *testing.T) {
	date := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

	inside := incomeFlowTestPayment("CTR-100", date, models.PaymentTypeInstallment, 500)
	inside.Contract.Lot.ProjectID = 4
	inside.Contract.Lot.Project = models.Project{ID: 4, Name: "Proyecto Filtrado"}

	outsideProject := incomeFlowTestPayment("CTR-101", date, models.PaymentTypeInstallment, 600)
	outsideProject.Contract.Lot.ProjectID = 5
	outsideProject.Contract.Lot.Project = models.Project{ID: 5, Name: "Otro Proyecto"}

	otherSellerID := uint(8)
	outsideSeller := incomeFlowTestPayment("CTR-102", date, models.PaymentTypeInstallment, 700)
	outsideSeller.Contract.Lot.ProjectID = 4
	outsideSeller.Contract.Lot.Project = models.Project{ID: 4, Name: "Proyecto Filtrado"}
	outsideSeller.Contract.SellerID = &otherSellerID
	outsideSeller.Contract.CreatorID = &otherSellerID

	paymentRepo := &incomeFlowFakePaymentRepository{
		payments: []models.Payment{inside, outsideProject, outsideSeller},
	}
	service := NewReportExportService(&operationsFakeAnalyticsRepository{}, nil, paymentRepo, nil, nil)
	projectID := uint(4)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	document, err := service.BuildIncomeFlowReport(context.Background(), ReportExportFilters{
		ProjectID:      &projectID,
		StartDate:      &start,
		EndDate:        &end,
		SellerID:       7,
		FilterBySeller: true,
	})

	require.NoError(t, err)
	require.NotNil(t, document)
	require.True(t, paymentRepo.streamed)
	require.NotNil(t, paymentRepo.capturedStart)
	require.NotNil(t, paymentRepo.capturedEnd)
	assert.True(t, paymentRepo.capturedStart.Equal(start))
	assert.True(t, paymentRepo.capturedEnd.Equal(end))
	require.NotNil(t, paymentRepo.capturedProjectID)
	assert.Equal(t, projectID, *paymentRepo.capturedProjectID)
	assert.True(t, paymentRepo.capturedFilterSeller)
	assert.Equal(t, uint(7), paymentRepo.capturedSellerID)
	section := operationsReportSection(t, document, "Detalle de Cobranzas")
	require.Len(t, section.Rows, 1)
	assert.Equal(t, "CTR-100", section.Rows[0].Cells[1].Text)
}
