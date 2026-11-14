package repository

import (
	"context"
	"errors"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommissionRepository owns the global commission settings (stored on the
// general_settings singleton), the per-seller rate exceptions and the seller
// commission credits created when a paid commission is reversed.
type CommissionRepository interface {
	GetSettings(ctx context.Context) (*models.CommissionSettings, error)
	UpdateSettings(ctx context.Context, settings *models.CommissionSettings) error
	FindSellerRate(ctx context.Context, sellerID uint) (*models.SellerCommissionRate, error)
	FindSellerRateByID(ctx context.Context, id uint) (*models.SellerCommissionRate, error)
	ListSellerRates(ctx context.Context) ([]models.SellerCommissionRate, error)
	UpsertSellerRate(ctx context.Context, rate *models.SellerCommissionRate) error
	DeleteSellerRate(ctx context.Context, id uint) error
	CreateCredit(ctx context.Context, credit *models.SellerCommissionCredit) error
	FindCreditByID(ctx context.Context, id uint) (*models.SellerCommissionCredit, error)
	ListCredits(ctx context.Context, sellerID uint) ([]models.SellerCommissionCredit, error)
	SettleCredit(ctx context.Context, id, actorID uint, settledAt time.Time) (bool, error)
}

type commissionRepository struct {
	db *gorm.DB
}

// NewCommissionRepository creates the commission repository.
func NewCommissionRepository(db *gorm.DB) CommissionRepository {
	return &commissionRepository{db: db}
}

// GetSettings returns the singleton rates. When the row is absent it returns
// the documented defaults instead of an error.
func (r *commissionRepository) GetSettings(ctx context.Context) (*models.CommissionSettings, error) {
	var settings models.CommissionSettings
	if err := r.db.WithContext(ctx).First(&settings, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			defaults := models.DefaultCommissionSettings()
			return &defaults, nil
		}
		return nil, err
	}
	return &settings, nil
}

func (r *commissionRepository) UpdateSettings(ctx context.Context, settings *models.CommissionSettings) error {
	if settings.ID == 0 {
		settings.ID = 1
	}
	rateUpdates := map[string]interface{}{
		"commission_default_rate":            settings.CommissionDefaultRate,
		"commission_unassigned_rate":         settings.CommissionUnassignedRate,
		"commission_acceptance_window_hours": settings.CommissionAcceptanceWindowHours,
	}
	result := r.db.WithContext(ctx).
		Model(&models.CommissionSettings{}).
		Where("id = ?", settings.ID).
		Updates(rateUpdates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	// The singleton row is normally seeded. Insert it when a fresh database
	// skipped the seed so the rates are never silently lost.
	return r.db.WithContext(ctx).Create(&models.CommissionSettings{
		ID:                       settings.ID,
		CommissionDefaultRate:    settings.CommissionDefaultRate,
		CommissionUnassignedRate: settings.CommissionUnassignedRate,
	}).Error
}

func (r *commissionRepository) FindSellerRate(ctx context.Context, sellerID uint) (*models.SellerCommissionRate, error) {
	var rate models.SellerCommissionRate
	if err := r.db.WithContext(ctx).Where("seller_id = ?", sellerID).First(&rate).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *commissionRepository) FindSellerRateByID(ctx context.Context, id uint) (*models.SellerCommissionRate, error) {
	var rate models.SellerCommissionRate
	if err := r.db.WithContext(ctx).Preload("Seller").First(&rate, id).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *commissionRepository) ListSellerRates(ctx context.Context) ([]models.SellerCommissionRate, error) {
	var rates []models.SellerCommissionRate
	err := r.db.WithContext(ctx).
		Preload("Seller").
		Order("id ASC").
		Find(&rates).Error
	return rates, err
}

func (r *commissionRepository) UpsertSellerRate(ctx context.Context, rate *models.SellerCommissionRate) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "seller_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rate", "updated_at"}),
	}).Create(rate).Error
}

func (r *commissionRepository) DeleteSellerRate(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.SellerCommissionRate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *commissionRepository) CreateCredit(ctx context.Context, credit *models.SellerCommissionCredit) error {
	if credit.Status == "" {
		credit.Status = models.CommissionCreditStatusPending
	}
	return r.db.WithContext(ctx).Create(credit).Error
}

func (r *commissionRepository) FindCreditByID(ctx context.Context, id uint) (*models.SellerCommissionCredit, error) {
	var credit models.SellerCommissionCredit
	if err := r.db.WithContext(ctx).
		Preload("Seller").
		Preload("Contract").
		First(&credit, id).Error; err != nil {
		return nil, err
	}
	return &credit, nil
}

// ListCredits returns every credit, or only one seller's credits when sellerID
// is greater than zero.
func (r *commissionRepository) ListCredits(ctx context.Context, sellerID uint) ([]models.SellerCommissionCredit, error) {
	var credits []models.SellerCommissionCredit
	query := r.db.WithContext(ctx).
		Preload("Seller").
		Preload("Contract").
		Order("created_at DESC, id DESC")
	if sellerID > 0 {
		query = query.Where("seller_id = ?", sellerID)
	}
	err := query.Find(&credits).Error
	return credits, err
}

// SettleCredit marks a pending credit as settled. The conditional update keeps
// repeated applies idempotent: the second call reports no affected rows.
func (r *commissionRepository) SettleCredit(ctx context.Context, id, actorID uint, settledAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.SellerCommissionCredit{}).
		Where("id = ? AND status = ?", id, models.CommissionCreditStatusPending).
		Updates(map[string]interface{}{
			"status":     models.CommissionCreditStatusSettled,
			"settled_at": settledAt,
			"settled_by": actorID,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
