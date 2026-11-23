package repository

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/sjperalta/fintera-api/internal/models"
)

// setupLiveDB opens the real database from DATABASE_URL and skips the test
// when it is unavailable, mirroring TestSetCacheConcurrentUpsert: sqlmock
// cannot validate Postgres-specific semantics (keyset scans, DISTINCT ON).
func setupLiveDB(t *testing.T) *gorm.DB {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping live repository test")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	return db
}

// seedOverdueContract creates the user/project/lot/contract chain plus the
// given number of pending, overdue installments (due dates one month apart in
// the past) and a single not-yet-due installment. Registered cleanup removes
// everything created. Returns the contract.
func seedOverdueContract(t *testing.T, db *gorm.DB, overdueCount int) *models.Contract {
	t.Helper()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	user := models.User{
		Email:             "live-" + suffix + "@example.com",
		EncryptedPassword: "not-a-hash",
		Identity:          "ID-" + suffix,
		RTN:               "RTN-" + suffix,
	}
	require.NoError(t, db.Create(&user).Error)

	project := models.Project{
		Name: "Live " + suffix, Description: "fixture", Address: "fixture",
		LotCount: 1, PricePerSquareUnit: 100, InterestRate: 5, GUID: "live-p-" + suffix,
	}
	require.NoError(t, db.Create(&project).Error)

	lot := models.Lot{
		GUID: "live-l-" + suffix, ProjectID: project.ID, Name: "L-1",
		Length: 10, Width: 10, Price: 100000,
	}
	require.NoError(t, db.Create(&lot).Error)

	contract := models.Contract{
		LotID: lot.ID, ApplicantUserID: user.ID, PaymentTerm: overdueCount + 1,
		GUID: "live-c-" + suffix, FinancingType: models.FinancingTypeDirect,
		Status: models.ContractStatusApproved, Active: true,
	}
	require.NoError(t, db.Create(&contract).Error)

	t.Cleanup(func() {
		db.Exec("DELETE FROM payments WHERE contract_id = ?", contract.ID)
		db.Exec("DELETE FROM contract_ledger_entries WHERE contract_id = ?", contract.ID)
		db.Exec("DELETE FROM contracts WHERE id = ?", contract.ID)
		db.Exec("DELETE FROM lots WHERE id = ?", lot.ID)
		db.Exec("DELETE FROM projects WHERE id = ?", project.ID)
		db.Exec("DELETE FROM users WHERE id = ?", user.ID)
	})

	base := time.Now().AddDate(-1, 0, 0)
	for i := 0; i < overdueCount; i++ {
		payment := models.Payment{
			ContractID:  contract.ID,
			Amount:      1000 + float64(i),
			DueDate:     base.AddDate(0, i, 0),
			Status:      models.PaymentStatusPending,
			PaymentType: models.PaymentTypeInstallment,
		}
		require.NoError(t, db.Create(&payment).Error)
	}

	// Not overdue: must never appear in the scans.
	future := models.Payment{
		ContractID:  contract.ID,
		Amount:      9999,
		DueDate:     time.Now().AddDate(0, 6, 0),
		Status:      models.PaymentStatusPending,
		PaymentType: models.PaymentTypeInstallment,
	}
	require.NoError(t, db.Create(&future).Error)

	return &contract
}

func TestPaymentRepository_FindOverdueInBatchesLiveReturnsEveryRowOnce(t *testing.T) {
	db := setupLiveDB(t)
	repo := NewPaymentRepository(db)

	const overdueCount = 7
	contract := seedOverdueContract(t, db, overdueCount)

	var ids []uint
	batches := 0
	err := repo.FindOverdueInBatches(context.Background(), 3, func(batch []models.Payment) error {
		batches++
		for i := range batch {
			if batch[i].Contract.ID != contract.ID {
				// Other tests may leave overdue rows in the database; only
				// our fixture is asserted below.
				continue
			}
			ids = append(ids, batch[i].ID)
		}
		return nil
	})
	require.NoError(t, err)

	require.Len(t, ids, overdueCount, "each overdue row exactly once")
	for i := 1; i < len(ids); i++ {
		require.NotEqual(t, ids[i-1], ids[i])
	}
	assert.GreaterOrEqual(t, batches, 3, "7 rows in batches of 3 must use at least 3 callbacks")
	assert.GreaterOrEqual(t, batches, (overdueCount+2)/3)
}

// TestContractRepository_LoadNextPaymentsLive verifies DISTINCT ON selects
// the earliest collectable payment per contract and ignores paid and
// in-review (receipt-bearing) payments, using real Postgres ordering.
func TestContractRepository_LoadNextPaymentsLive(t *testing.T) {
	db := setupLiveDB(t)
	repo := NewContractRepository(db).(*contractRepository)

	contract := seedOverdueContract(t, db, 3)

	// A paid payment with an even earlier due date must be ignored (status).
	earlier := time.Now().AddDate(-2, 0, 0)
	paid := models.Payment{
		ContractID: contract.ID, Amount: 500, DueDate: earlier,
		Status: models.PaymentStatusPaid, PaymentType: models.PaymentTypeInstallment,
	}
	require.NoError(t, db.Create(&paid).Error)

	// A submitted payment that already has a receipt document must be ignored.
	doc := "receipts/live-doc.pdf"
	receipt := models.Payment{
		ContractID: contract.ID, Amount: 600, DueDate: earlier.AddDate(0, 0, 1),
		Status: models.PaymentStatusSubmitted, PaymentType: models.PaymentTypeInstallment,
		DocumentPath: &doc,
	}
	require.NoError(t, db.Create(&receipt).Error)

	// Expected: the earliest pending installment without a document, i.e. the
	// first of the three seeded overdue payments (due date = -1 year).
	var expected models.Payment
	require.NoError(t, db.
		Where("contract_id = ? AND status IN ? AND (document_path IS NULL OR document_path = '')",
			contract.ID, []string{models.PaymentStatusPending, models.PaymentStatusSubmitted}).
		Order("due_date ASC, id ASC").
		First(&expected).Error)

	contracts := []models.Contract{{ID: contract.ID}}
	require.NoError(t, repo.loadNextPayments(context.Background(), contracts))

	require.NotNil(t, contracts[0].NextPayment)
	assert.Equal(t, expected.ID, contracts[0].NextPayment.ID)
}

// TestPaymentRepository_ListScopeFiltersLive verifies the report scoping
// filters push down to SQL: only rows in the requested project/seller scope
// reach Go.
func TestPaymentRepository_ListScopeFiltersLive(t *testing.T) {
	db := setupLiveDB(t)
	repo := NewPaymentRepository(db)

	contract := seedOverdueContract(t, db, 2)

	var lot models.Lot
	require.NoError(t, db.First(&lot, contract.LotID).Error)

	paidAmount := 700.0
	paid := models.Payment{
		ContractID: contract.ID, Amount: 700, PaidAmount: &paidAmount,
		DueDate: time.Now(), Status: models.PaymentStatusPaid,
		PaymentType: models.PaymentTypeInstallment,
	}
	require.NoError(t, db.Create(&paid).Error)

	containsContract := func(payments []models.Payment, id uint) bool {
		for i := range payments {
			if payments[i].ContractID == id {
				return true
			}
		}
		return false
	}

	// Project scope: in-scope project includes the fixture, every returned
	// row belongs to that project.
	query := NewListQuery()
	query.PerPage = 500
	query.Filters["status"] = models.PaymentStatusPaid
	query.Filters["project_id"] = strconv.FormatUint(uint64(lot.ProjectID), 10)
	payments, _, err := repo.List(context.Background(), query)
	require.NoError(t, err)
	assert.True(t, containsContract(payments, contract.ID))
	for i := range payments {
		assert.Equal(t, lot.ProjectID, payments[i].Contract.Lot.ProjectID)
	}

	// A different project id excludes the fixture entirely.
	query.Filters["project_id"] = "4294967295"
	payments, _, err = repo.List(context.Background(), query)
	require.NoError(t, err)
	assert.False(t, containsContract(payments, contract.ID))

	// Seller scope: attribute the contract to its applicant user as creator,
	// then the seller_id filter must include it.
	require.NoError(t, db.Model(&models.Contract{}).
		Where("id = ?", contract.ID).
		Update("creator_id", contract.ApplicantUserID).Error)

	query = NewListQuery()
	query.PerPage = 500
	query.Filters["status"] = models.PaymentStatusPaid
	query.Filters["seller_id"] = strconv.FormatUint(uint64(contract.ApplicantUserID), 10)
	payments, _, err = repo.List(context.Background(), query)
	require.NoError(t, err)
	assert.True(t, containsContract(payments, contract.ID))

	query.Filters["seller_id"] = "4294967294"
	payments, _, err = repo.List(context.Background(), query)
	require.NoError(t, err)
	assert.False(t, containsContract(payments, contract.ID))
}

// TestPaymentRepository_StreamPaidForReportLive checks the income-flow stream
// returns every in-scope paid payment exactly once across batches, honours the
// project scope, and skips payments of other projects.
func TestPaymentRepository_StreamPaidForReportLive(t *testing.T) {
	db := setupLiveDB(t)
	repo := NewPaymentRepository(db)

	contract := seedOverdueContract(t, db, 3)
	var lot models.Lot
	require.NoError(t, db.First(&lot, contract.LotID).Error)

	// Three paid payments within the report period.
	start := time.Now().AddDate(0, 0, -30)
	end := time.Now().AddDate(0, 1, 0)
	amount := 500.0
	var paidIDs []uint
	for i := 0; i < 3; i++ {
		pay := time.Now().AddDate(0, 0, -i)
		payment := models.Payment{
			ContractID: contract.ID, Amount: 500, PaidAmount: &amount,
			PaymentDate: &pay, DueDate: pay,
			Status: models.PaymentStatusPaid, PaymentType: models.PaymentTypeInstallment,
		}
		require.NoError(t, db.Create(&payment).Error)
		paidIDs = append(paidIDs, payment.ID)
	}

	seen := map[uint]bool{}
	batches := 0
	err := repo.StreamPaidForReport(context.Background(), &start, &end, &lot.ProjectID, 0, false, 2,
		func(batch []models.Payment) error {
			batches++
			for i := range batch {
				seen[batch[i].ID] = true
				assert.Equal(t, lot.ProjectID, batch[i].Contract.Lot.ProjectID)
			}
			return nil
		})
	require.NoError(t, err)

	for _, id := range paidIDs {
		assert.True(t, seen[id], "paid payment %d must be streamed", id)
	}
	assert.GreaterOrEqual(t, batches, 2, "3 rows in batches of 2 must use at least 2 callbacks")

	// A different project scope excludes every fixture payment.
	other := lot.ProjectID + 4294967290
	seenOther := 0
	err = repo.StreamPaidForReport(context.Background(), &start, &end, &other, 0, false, 500,
		func(batch []models.Payment) error {
			for i := range batch {
				if batch[i].ContractID == contract.ID {
					seenOther++
				}
			}
			return nil
		})
	require.NoError(t, err)
	assert.Zero(t, seenOther, "out-of-project payments never reach Go")
}

// TestPaymentRepository_StreamOverdueDetailedLive checks the overdue export
// stream covers the fixture's overdue installments in batches and applies the
// seller scope in SQL.
func TestPaymentRepository_StreamOverdueDetailedLive(t *testing.T) {
	db := setupLiveDB(t)
	repo := NewOverdueReportRepository(db)

	contract := seedOverdueContract(t, db, 5)
	require.NoError(t, db.Model(&models.Contract{}).
		Where("id = ?", contract.ID).
		Update("creator_id", contract.ApplicantUserID).Error)

	var projectID uint
	require.NoError(t, db.Model(&models.Lot{}).
		Where("id = ?", contract.LotID).
		Select("project_id").Row().Scan(&projectID))

	inScope := 0
	batches := 0
	err := repo.StreamOverdueForActiveContractsDetailed(context.Background(), &projectID, contract.ApplicantUserID, true, 2,
		func(batch []models.Payment) error {
			batches++
			for i := range batch {
				if batch[i].ContractID == contract.ID {
					inScope++
				}
			}
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, 5, inScope, "all 5 overdue installments streamed")
	assert.GreaterOrEqual(t, batches, 3)

	// A seller scope that owns nothing excludes the fixture entirely.
	otherSeller := contract.ApplicantUserID + 4294967290
	excluded := 0
	err = repo.StreamOverdueForActiveContractsDetailed(context.Background(), nil, otherSeller, true, 500,
		func(batch []models.Payment) error {
			for i := range batch {
				if batch[i].ContractID == contract.ID {
					excluded++
				}
			}
			return nil
		})
	require.NoError(t, err)
	assert.Zero(t, excluded)
}
