package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeCommissionRepository struct {
	repository.CommissionRepository
	settings     models.CommissionSettings
	exceptions   map[uint]*models.SellerCommissionRate
	credits      map[uint]*models.SellerCommissionCredit
	nextID       uint
	nextCreditID uint
	deleted      []uint
}

func newFakeCommissionRepository() *fakeCommissionRepository {
	settings := models.DefaultCommissionSettings()
	return &fakeCommissionRepository{
		settings:     settings,
		exceptions:   make(map[uint]*models.SellerCommissionRate),
		credits:      make(map[uint]*models.SellerCommissionCredit),
		nextID:       1,
		nextCreditID: 1,
	}
}

func (r *fakeCommissionRepository) GetSettings(context.Context) (*models.CommissionSettings, error) {
	copy := r.settings
	return &copy, nil
}

func (r *fakeCommissionRepository) UpdateSettings(_ context.Context, settings *models.CommissionSettings) error {
	settings.ID = 1
	r.settings = *settings
	return nil
}

func (r *fakeCommissionRepository) FindSellerRate(_ context.Context, sellerID uint) (*models.SellerCommissionRate, error) {
	rate, ok := r.exceptions[sellerID]
	if !ok {
		return nil, gormRecordNotFound()
	}
	return rate, nil
}

func (r *fakeCommissionRepository) FindSellerRateByID(_ context.Context, id uint) (*models.SellerCommissionRate, error) {
	for _, rate := range r.exceptions {
		if rate.ID == id {
			return rate, nil
		}
	}
	return nil, gormRecordNotFound()
}

func (r *fakeCommissionRepository) ListSellerRates(context.Context) ([]models.SellerCommissionRate, error) {
	rates := make([]models.SellerCommissionRate, 0, len(r.exceptions))
	for _, rate := range r.exceptions {
		rates = append(rates, *rate)
	}
	return rates, nil
}

func (r *fakeCommissionRepository) UpsertSellerRate(_ context.Context, rate *models.SellerCommissionRate) error {
	if existing, ok := r.exceptions[rate.SellerID]; ok {
		existing.Rate = rate.Rate
		return nil
	}
	rate.ID = r.nextID
	r.nextID++
	copy := *rate
	r.exceptions[rate.SellerID] = &copy
	return nil
}

func (r *fakeCommissionRepository) DeleteSellerRate(_ context.Context, id uint) error {
	for sellerID, rate := range r.exceptions {
		if rate.ID == id {
			delete(r.exceptions, sellerID)
			r.deleted = append(r.deleted, id)
			return nil
		}
	}
	return gormRecordNotFound()
}

func (r *fakeCommissionRepository) CreateCredit(_ context.Context, credit *models.SellerCommissionCredit) error {
	credit.ID = r.nextCreditID
	r.nextCreditID++
	if credit.Status == "" {
		credit.Status = models.CommissionCreditStatusPending
	}
	copy := *credit
	r.credits[credit.ID] = &copy
	return nil
}

func (r *fakeCommissionRepository) FindCreditByID(_ context.Context, id uint) (*models.SellerCommissionCredit, error) {
	credit, ok := r.credits[id]
	if !ok {
		return nil, gormRecordNotFound()
	}
	copy := *credit
	return &copy, nil
}

func (r *fakeCommissionRepository) ListCredits(_ context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	credits := make([]models.SellerCommissionCredit, 0, len(r.credits))
	for _, credit := range r.credits {
		if sellerID > 0 && credit.SellerID != sellerID {
			continue
		}
		credits = append(credits, *credit)
	}
	return credits, nil
}

func (r *fakeCommissionRepository) SettleCredit(_ context.Context, id, actorID uint, settledAt time.Time) (bool, error) {
	credit, ok := r.credits[id]
	if !ok {
		return false, gormRecordNotFound()
	}
	if credit.Status == models.CommissionCreditStatusSettled {
		return false, nil
	}
	credit.Status = models.CommissionCreditStatusSettled
	credit.SettledAt = &settledAt
	credit.SettledBy = &actorID
	return true, nil
}

type fakeCommissionUserRepository struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (r *fakeCommissionUserRepository) FindByID(_ context.Context, id uint) (*models.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, gormRecordNotFound()
	}
	return user, nil
}

func (r *fakeCommissionUserRepository) FindByIDs(_ context.Context, ids []uint) ([]models.User, error) {
	users := make([]models.User, 0, len(ids))
	for _, id := range ids {
		if user, ok := r.users[id]; ok {
			users = append(users, *user)
		}
	}
	return users, nil
}

func gormRecordNotFound() error {
	return gorm.ErrRecordNotFound
}

func TestCommissionRateResolution(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.settings = models.CommissionSettings{ID: 1, CommissionDefaultRate: 5, CommissionUnassignedRate: 3}
	sellerRepo := &fakeCommissionUserRepository{users: map[uint]*models.User{
		7: {ID: 7, Role: models.RoleSeller, FullName: "Ana Vendedora", Status: models.StatusActive},
	}}
	service := NewCommissionService(repo, sellerRepo, nil)
	project := models.Project{ID: 1, CommissionRateDirect: 4, CommissionRateBank: 6}

	amount := 100000.0
	base := func(sellerID *uint, financing string) *models.Contract {
		return &models.Contract{
			Amount:        &amount,
			FinancingType: financing,
			SellerID:      sellerID,
			Lot:           models.Lot{Project: project},
		}
	}

	sellerID := uint(7)
	rate, err := service.ResolveRateForContract(context.Background(), base(nil, models.FinancingTypeDirect))
	require.NoError(t, err)
	assert.Equal(t, 3.0, rate, "contracts without a seller use the unassigned rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeDirect))
	require.NoError(t, err)
	assert.Equal(t, 4.0, rate, "projects provide the direct rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeBank))
	require.NoError(t, err)
	assert.Equal(t, 6.0, rate, "projects provide the bank rate")

	repo.exceptions[sellerID] = &models.SellerCommissionRate{ID: 1, SellerID: sellerID, Rate: 7}
	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeBank))
	require.NoError(t, err)
	assert.Equal(t, 7.0, rate, "the seller exception wins over the project rate")

	rate, err = service.ResolveRateForContract(context.Background(), base(&sellerID, models.FinancingTypeCash))
	require.NoError(t, err)
	assert.Equal(t, 7.0, rate, "the seller exception wins for every financing type")

	projectWithoutBank := models.Project{ID: 1, CommissionRateDirect: 0}
	contract := base(&sellerID, models.FinancingTypeBank)
	contract.Lot.Project = projectWithoutBank
	delete(repo.exceptions, sellerID)
	rate, err = service.ResolveRateForContract(context.Background(), contract)
	require.NoError(t, err)
	assert.Equal(t, 5.0, rate, "missing project rates fall back to the global default")
}

func TestCommissionApplyFreezesOriginAndAccrual(t *testing.T) {
	repo := newFakeCommissionRepository()
	repo.settings = models.CommissionSettings{ID: 1, CommissionDefaultRate: 5, CommissionUnassignedRate: 3}
	service := NewCommissionService(repo, &fakeCommissionUserRepository{}, nil)

	amount := 100000.0
	sellerID := uint(7)
	approvedAt := time.Date(2026, 5, 20, 15, 4, 0, 0, time.UTC)
	contract := &models.Contract{
		Amount:           &amount,
		FinancingType:    models.FinancingTypeDirect,
		AssignmentOrigin: models.AssignmentOriginReferral,
		SellerID:         &sellerID,
		Lot:              models.Lot{Project: models.Project{ID: 1, CommissionRateDirect: 4}},
	}
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, 4000.0, contract.CommissionAmount)
	assert.Equal(t, models.CommissionOriginReferral, contract.CommissionOrigin)
	require.NotNil(t, contract.CommissionAccrualDate)
	assert.Equal(t, "2026-05-20", contract.CommissionAccrualDate.Format(models.HistoricalSaleDateLayout))

	contract.AssignmentOrigin = models.AssignmentOriginManual
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, models.CommissionOriginDirect, contract.CommissionOrigin)

	contract.SellerID = nil
	require.NoError(t, service.ApplyToContract(context.Background(), contract, approvedAt))
	assert.Equal(t, 3000.0, contract.CommissionAmount, "unassigned contracts use 3%")
}

func TestCommissionRulesAndExceptionsCRUD(t *testing.T) {
	repo := newFakeCommissionRepository()
	seller := &models.User{ID: 9, Role: models.RoleSeller, FullName: "Carlos Vendedor", Status: models.StatusActive}
	service := NewCommissionService(repo, &fakeCommissionUserRepository{users: map[uint]*models.User{9: seller}}, nil)
	ctx := context.Background()

	rules, err := service.Rules(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5.0, rules.DefaultRate)
	assert.Equal(t, 3.0, rules.UnassignedRate)
	assert.Equal(t, models.CommissionPaymentRule, rules.PaymentRule)
	assert.Equal(t, 2, rules.AcceptanceWindowHours)

	defaultRate := 5.5
	unassigned := 4.0
	rules, err = service.UpdateRules(ctx, 1, CommissionRulesInput{DefaultRate: &defaultRate, UnassignedRate: &unassigned})
	require.NoError(t, err)
	assert.Equal(t, 5.5, rules.DefaultRate)
	assert.Equal(t, 4.0, rules.UnassignedRate)
	assert.Equal(t, 5.5, repo.settings.CommissionDefaultRate)

	invalid := 101.0
	_, err = service.UpdateRules(ctx, 1, CommissionRulesInput{DefaultRate: &invalid})
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)

	exception, err := service.UpsertException(ctx, 1, 9, 6.5)
	require.NoError(t, err)
	assert.Equal(t, uint(9), exception.SellerID)
	assert.Equal(t, 6.5, exception.Rate)
	assert.Equal(t, "Carlos Vendedor", exception.SellerName)

	listed, err := service.ListExceptions(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	require.NoError(t, service.DeleteException(ctx, 1, exception.ID))
	assert.Empty(t, repo.exceptions)
	assert.ErrorIs(t, service.DeleteException(ctx, 1, exception.ID), ErrCommissionExceptionMissing)
}

func TestCommissionService_CreateCancellationCredit(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)

	paidAt := time.Now()
	sellerID := uint(7)
	contract := &models.Contract{
		ID:               11,
		SellerID:         &sellerID,
		CommissionAmount: 1500,
		CommissionPaidAt: &paidAt,
	}

	credit, err := service.CreateCancellationCredit(context.Background(), contract)

	require.NoError(t, err)
	require.NotNil(t, credit)
	assert.Equal(t, sellerID, credit.SellerID)
	assert.Equal(t, uint(11), credit.ContractID)
	assert.Equal(t, 1500.0, credit.Amount)
	assert.Equal(t, models.CommissionCreditStatusPending, credit.Status)
	require.Len(t, repo.credits, 1)
}

func TestCommissionService_CreateCancellationCredit_SkipsUnpaidCommission(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)

	sellerID := uint(7)
	contract := &models.Contract{
		ID:               11,
		SellerID:         &sellerID,
		CommissionAmount: 1500,
	}

	credit, err := service.CreateCancellationCredit(context.Background(), contract)

	require.NoError(t, err)
	assert.Nil(t, credit)
	assert.Empty(t, repo.credits, "a contract without a paid commission must not create a credit")
}

func TestCommissionService_ApplyCredit(t *testing.T) {
	repo := newFakeCommissionRepository()
	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	service := NewCommissionService(repo, nil, NewAuditService(gdb))

	seller := &models.User{ID: 7, FullName: "Ana Vendedora", Role: models.RoleSeller}
	require.NoError(t, repo.CreateCredit(context.Background(), &models.SellerCommissionCredit{
		SellerID:   seller.ID,
		ContractID: 11,
		Amount:     1500,
		Status:     models.CommissionCreditStatusPending,
	}))
	// FindCreditByID preloads the seller in production; mirror it in the fake.
	repo.credits[1].Seller = seller

	credit, err := service.ApplyCredit(context.Background(), 3, 1)

	require.NoError(t, err)
	assert.Equal(t, models.CommissionCreditStatusSettled, credit.Status)
	assert.NotNil(t, credit.SettledAt)
	require.NotNil(t, credit.SettledBy)
	assert.Equal(t, uint(3), *credit.SettledBy)
	assert.Equal(t, "Ana Vendedora", credit.Seller)
	assert.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestCommissionService_ApplyCredit_IsIdempotent(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)
	require.NoError(t, repo.CreateCredit(context.Background(), &models.SellerCommissionCredit{
		SellerID:   7,
		ContractID: 11,
		Amount:     1500,
		Status:     models.CommissionCreditStatusPending,
	}))

	_, err := service.ApplyCredit(context.Background(), 3, 1)
	require.NoError(t, err)

	_, err = service.ApplyCredit(context.Background(), 3, 1)
	assert.ErrorIs(t, err, ErrCommissionCreditSettled)
}

func TestCommissionService_ApplyCredit_Missing(t *testing.T) {
	service := NewCommissionService(newFakeCommissionRepository(), nil, nil)

	_, err := service.ApplyCredit(context.Background(), 3, 99)

	assert.ErrorIs(t, err, ErrCommissionCreditMissing)
}

func TestCommissionService_ListCreditsFiltersSeller(t *testing.T) {
	repo := newFakeCommissionRepository()
	service := NewCommissionService(repo, nil, nil)
	for _, credit := range []*models.SellerCommissionCredit{
		{SellerID: 7, ContractID: 11, Amount: 500},
		{SellerID: 7, ContractID: 12, Amount: 250},
		{SellerID: 8, ContractID: 13, Amount: 900},
	} {
		require.NoError(t, repo.CreateCredit(context.Background(), credit))
	}

	all, err := service.ListCredits(context.Background(), 0)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	own, err := service.ListCredits(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, own, 2)
	for _, credit := range own {
		assert.Equal(t, uint(7), credit.SellerID)
	}
}
