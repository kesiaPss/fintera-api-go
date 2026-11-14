package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
)

type AnalyticsRepository interface {
	GetCache(ctx context.Context, key string, projectID *uint) (*models.AnalyticsCache, error)
	SetCache(ctx context.Context, key string, projectID *uint, data interface{}, ttl time.Duration) error
	InvalidateCache(ctx context.Context, key string, projectID *uint) error
	CleanExpiredCache(ctx context.Context) error

	// Data retrieval for analytics
	GetTotalRevenue(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error)
	GetActiveContractsCount(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (int, error)
	GetAveragePayment(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error)
	GetOccupancyRate(ctx context.Context, projectID *uint, endDate *time.Time) (float64, error)
	GetRevenueTrend(ctx context.Context, projectID *uint, timeframe string, year *int) ([]models.RevenueTrendPoint, error)
	GetLotDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error)
	GetProjectPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.ProjectPerformance, error)
	GetSellerPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.SellerPerformance, error)

	// Report aggregations. Each method accepts optional project, seller and
	// period filters and returns zero values (never null scan errors) for empty
	// periods. A non-nil sellerID scopes the aggregation to the contracts where
	// the user is the assigned seller or the creator.
	GetContractPeriodMetrics(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.ContractPeriodMetrics, error)
	GetFinancingSales(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.FinancingSales, error)
	GetCollectionsSummary(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.CollectionsSummary, error)
	GetLedgerTotalsByEntryType(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) ([]models.LedgerTypeTotal, error)
	GetPortfolioSnapshot(ctx context.Context, projectID *uint, sellerID *uint) (models.PortfolioSnapshot, error)
	GetIncomeFlowMonthly(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) ([]models.IncomeFlowMonth, error)
	GetCollectionsByProject(ctx context.Context, sellerID *uint, startDate, endDate *time.Time) ([]models.ProjectCollections, error)
}

type analyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) AnalyticsRepository {
	return &analyticsRepository{db: db}
}

func (r *analyticsRepository) GetCache(ctx context.Context, key string, projectID *uint) (*models.AnalyticsCache, error) {
	var cache models.AnalyticsCache
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	} else {
		db = db.Where("project_id IS NULL")
	}

	err := db.Where("expires_at > ?", time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func (r *analyticsRepository) SetCache(ctx context.Context, key string, projectID *uint, data interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	cache := models.AnalyticsCache{
		CacheKey:  key,
		ProjectID: projectID,
		Data:      jsonData,
		ExpiresAt: time.Now().Add(ttl),
	}

	// Upsert strategy
	var existing models.AnalyticsCache
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	} else {
		db = db.Where("project_id IS NULL")
	}

	err = db.First(&existing).Error
	if err == nil {
		return r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"data":       jsonData,
			"expires_at": cache.ExpiresAt,
			"updated_at": time.Now(),
		}).Error
	}

	return r.db.WithContext(ctx).Create(&cache).Error
}

func (r *analyticsRepository) InvalidateCache(ctx context.Context, key string, projectID *uint) error {
	db := r.db.WithContext(ctx).Where("cache_key = ?", key)
	if projectID != nil {
		db = db.Where("project_id = ?", *projectID)
	}
	return db.Delete(&models.AnalyticsCache{}).Error
}

func (r *analyticsRepository) CleanExpiredCache(ctx context.Context) error {
	err := r.db.WithContext(ctx).Where("expires_at <= ?", time.Now()).Delete(&models.AnalyticsCache{}).Error
	if err != nil && strings.Contains(err.Error(), "42P01") {
		// Table does not exist yet (migration not run); skip cleanup
		return nil
	}
	return err
}

// Data retrieval implementations

func (r *analyticsRepository) GetTotalRevenue(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error) {
	var total float64
	query := r.db.WithContext(ctx).Table("payments").
		Select("COALESCE(SUM(paid_amount), 0)").
		Where("payments.status = ?", models.PaymentStatusPaid)

	if projectID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("payments.payment_date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("payment_date <= ?", *endDate)
	}

	err := query.Scan(&total).Error
	return total, err
}

func (r *analyticsRepository) GetActiveContractsCount(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (int, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.Contract{}).
		Where("contracts.status = ?", models.ContractStatusApproved).
		Where("contracts.active = ?", true)

	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("contracts.approved_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("contracts.approved_at <= ?", *endDate)
	}

	err := query.Count(&count).Error
	return int(count), err
}

func (r *analyticsRepository) GetAveragePayment(ctx context.Context, projectID *uint, startDate, endDate *time.Time) (float64, error) {
	var avg float64
	query := r.db.WithContext(ctx).Table("payments").
		Select("COALESCE(AVG(paid_amount), 0)").
		Where("payments.status = ?", models.PaymentStatusPaid)

	if projectID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	if startDate != nil {
		query = query.Where("payments.payment_date >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("payments.payment_date <= ?", *endDate)
	}

	err := query.Scan(&avg).Error
	return avg, err
}

func (r *analyticsRepository) GetOccupancyRate(ctx context.Context, projectID *uint, endDate *time.Time) (float64, error) {
	var total, occupied int64

	lotQuery := r.db.WithContext(ctx).Model(&models.Lot{})
	if projectID != nil {
		lotQuery = lotQuery.Where("project_id = ?", *projectID)
	}

	if err := lotQuery.Count(&total).Error; err != nil || total == 0 {
		return 0, err
	}

	if endDate != nil {
		// Occupancy as of endDate: count distinct lots that had an approved contract not yet closed by endDate
		subq := r.db.WithContext(ctx).Model(&models.Contract{}).
			Select("DISTINCT contracts.lot_id").
			Joins("INNER JOIN lots ON lots.id = contracts.lot_id").
			Where("contracts.status IN ?", []string{models.ContractStatusApproved, models.ContractStatusClosed}).
			Where("contracts.approved_at IS NOT NULL AND contracts.approved_at <= ?", *endDate).
			Where("(contracts.closed_at IS NULL OR contracts.closed_at > ?)", *endDate)
		if projectID != nil {
			subq = subq.Where("lots.project_id = ?", *projectID)
		}
		if err := r.db.WithContext(ctx).Table("(?) AS occupied_lots", subq).Count(&occupied).Error; err != nil {
			return 0, err
		}
	} else {
		// Current snapshot: lots whose status is reserved/financed/fully_paid
		err := lotQuery.Where("lots.status IN ?", []string{models.LotStatusReserved, models.LotStatusFinanced, models.LotStatusFullyPaid}).Count(&occupied).Error
		if err != nil {
			return 0, err
		}
	}

	return (float64(occupied) / float64(total)) * 100, nil
}

func (r *analyticsRepository) GetRevenueTrend(ctx context.Context, projectID *uint, timeframe string, year *int) ([]models.RevenueTrendPoint, error) {
	var points []models.RevenueTrendPoint

	var startDate, endDate time.Time
	if year != nil {
		startDate = time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(*year, 12, 31, 23, 59, 59, 0, time.UTC)
	} else {
		// Default to last 12 months if not specified
		months := 12
		if timeframe == "6M" {
			months = 6
		}
		// Calculate start date
		startDate = time.Now().AddDate(0, -months, 0)
		endDate = time.Now().AddDate(0, months, 0) // Future projections within same timeframe
	}

	// Real revenue (paid payments)
	var realResults []struct {
		Label string
		Total float64
	}

	realQuery := r.db.WithContext(ctx).Table("payments").
		Select("TO_CHAR(payments.payment_date, 'Mon') as label, SUM(payments.paid_amount) as total, MIN(payments.payment_date) as sort_date").
		Where("payments.status = ?", models.PaymentStatusPaid).
		Where("payments.payment_date >= ?", startDate).
		Where("payments.payment_date <= ?", endDate).
		Group("TO_CHAR(payments.payment_date, 'Mon')").
		Order("sort_date ASC")

	if projectID != nil {
		realQuery = realQuery.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	err := realQuery.Scan(&realResults).Error
	if err != nil {
		return nil, err
	}

	// Projected revenue (pending payments for future months)
	var projectedResults []struct {
		Label string
		Total float64
	}

	projectedQuery := r.db.WithContext(ctx).Table("payments").
		Select("TO_CHAR(payments.due_date, 'Mon') as label, SUM(payments.amount) as total, MIN(payments.due_date) as sort_date").
		Where("payments.status = ?", models.PaymentStatusPending).
		Where("payments.due_date >= ?", startDate).
		Where("payments.due_date <= ?", endDate).
		Group("TO_CHAR(payments.due_date, 'Mon')").
		Order("sort_date ASC")

	if projectID != nil {
		projectedQuery = projectedQuery.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	err = projectedQuery.Scan(&projectedResults).Error
	if err != nil {
		return nil, err
	}

	// Merge results
	labelMap := make(map[string]*models.RevenueTrendPoint)

	for _, res := range realResults {
		labelMap[res.Label] = &models.RevenueTrendPoint{Label: res.Label, Real: res.Total}
	}

	for _, res := range projectedResults {
		if pt, ok := labelMap[res.Label]; ok {
			pt.Projected = res.Total
		} else {
			labelMap[res.Label] = &models.RevenueTrendPoint{Label: res.Label, Projected: res.Total}
		}
	}

	// Combine all results and sort
	// Include both real and projected data points
	// In a real implementation, we would ensure all months in range are present
	for _, res := range realResults {
		if pt, ok := labelMap[res.Label]; ok {
			points = append(points, *pt)
			delete(labelMap, res.Label) // Remove to avoid duplicates
		}
	}

	// Add remaining projected-only months
	for _, pt := range labelMap {
		points = append(points, *pt)
	}

	return points, nil
}

func (r *analyticsRepository) GetLotDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error) {
	var dist models.LotDistribution

	query := r.db.WithContext(ctx).Model(&models.Lot{})
	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	var results []struct {
		Status string
		Count  int
	}

	err := query.Select("status, COUNT(*) as count").Group("status").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	for _, res := range results {
		dist.TotalLots += res.Count
		switch res.Status {
		case models.LotStatusAvailable:
			dist.Available = res.Count
		case models.LotStatusReserved:
			dist.Reserved = res.Count
		case models.LotStatusFinanced:
			dist.Financed = res.Count
		case models.LotStatusFullyPaid:
			dist.FullyPaid = res.Count
		}
	}

	// Calculate percentages
	if dist.TotalLots > 0 {
		dist.AvailablePercentage = (float64(dist.Available) / float64(dist.TotalLots)) * 100
		dist.ReservedPercentage = (float64(dist.Reserved) / float64(dist.TotalLots)) * 100
		dist.FinancedPercentage = (float64(dist.Financed) / float64(dist.TotalLots)) * 100
		dist.FullyPaidPercentage = (float64(dist.FullyPaid) / float64(dist.TotalLots)) * 100
	}

	return &dist, nil
}

func (r *analyticsRepository) GetProjectPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.ProjectPerformance, error) {
	query := r.db.WithContext(ctx).Preload("Lots")
	if filters.ProjectID != nil {
		query = query.Where("id = ?", *filters.ProjectID)
	}
	var projects []models.Project
	err := query.Find(&projects).Error
	if err != nil {
		return nil, err
	}

	var performance []models.ProjectPerformance
	for _, p := range projects {
		var totalLots, soldLots int
		totalLots = p.LotCount
		if totalLots == 0 {
			totalLots = len(p.Lots)
		}

		for _, lot := range p.Lots {
			if lot.Status == models.LotStatusFinanced || lot.Status == models.LotStatusFullyPaid {
				soldLots++
			}
		}

		growth := 0.0
		if totalLots > 0 {
			growth = (float64(soldLots) / float64(totalLots)) * 100
		}

		// Mock weekly performance for now or calculate real ones from payments
		performance = append(performance, models.ProjectPerformance{
			ProjectID:         strconv.FormatUint(uint64(p.ID), 10),
			ProjectName:       p.Name,
			GrowthPercentage:  growth,
			WeeklyPerformance: []float64{65, 59, 80, 81}, // Mock weekly data for now
		})
	}

	return performance, nil
}

func (r *analyticsRepository) GetSellerPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.SellerPerformance, error) {
	var results []models.SellerPerformance

	// Query users (sellers) and left join based on filters
	// We want Sum of Contracts Amount (Total Sales) and Count of Contracts (Approved Contracts)
	// filtering by approved_at date range.
	query := r.db.WithContext(ctx).Table("users").
		Select("users.id as seller_id, users.full_name as seller_name, COALESCE(SUM(contracts.amount), 0) as total_sales, COUNT(contracts.id) as active_contracts").
		Where("users.role = ?", models.RoleSeller).
		Group("users.id, users.full_name")

	// Construct dynamic LEFT JOIN to preserve sellers with 0 sales
	joinParams := []interface{}{[]string{models.ContractStatusApproved, models.ContractStatusClosed}}
	joinQuery := "LEFT JOIN contracts ON contracts.creator_id = users.id AND contracts.status IN (?)"

	if filters.StartDate != nil {
		joinQuery += " AND contracts.approved_at >= ?"
		joinParams = append(joinParams, *filters.StartDate)
	}
	if filters.EndDate != nil {
		joinQuery += " AND contracts.approved_at <= ?"
		joinParams = append(joinParams, *filters.EndDate)
	}

	query = query.Joins(joinQuery, joinParams...)

	err := query.Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Report aggregations.

// ledgerEntryTypeOrder is the order in which the ledger breakdown is exposed so
// every renderer prints the same rows, including the ones with a zero total.
var ledgerEntryTypeOrder = []string{
	models.EntryTypeInitial,
	models.EntryTypeInterest,
	models.EntryTypePayment,
	models.EntryTypePrepayment,
	models.EntryTypeAdjustment,
	models.EntryTypeRefund,
}

// applyDateRange adds inclusive date bounds for column. column is always a
// hard-coded identifier, never user input.
func applyDateRange(query *gorm.DB, column string, startDate, endDate *time.Time) *gorm.DB {
	if startDate != nil {
		query = query.Where(column+" >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where(column+" <= ?", *endDate)
	}
	return query
}

// applyContractSellerFilter scopes a contract aggregation to the contracts
// where the user is the assigned seller or the creator. It mirrors the seller
// scope used by the contract listing: an accepted assignment or authorship.
func applyContractSellerFilter(query *gorm.DB, sellerID *uint) *gorm.DB {
	if sellerID == nil {
		return query
	}
	return query.Where("(contracts.seller_id = ? OR contracts.creator_id = ?)", *sellerID, *sellerID)
}

// applyPaymentScope joins the payment -> contract chain (and -> lot when a
// project is selected) and applies the optional project and seller filters.
// Joins are only added when a filter needs them, so unfiltered aggregates keep
// their original single-table query.
func applyPaymentScope(query *gorm.DB, projectID *uint, sellerID *uint) *gorm.DB {
	if projectID == nil && sellerID == nil {
		return query
	}
	query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id")
	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}
	return applyContractSellerFilter(query, sellerID)
}

// contractPeriodCondition builds the CASE-WHEN predicate for a contract
// timestamp: the column must be set and fall inside the requested period.
func contractPeriodCondition(column string, startDate, endDate *time.Time) (string, []interface{}) {
	parts := []string{column + " IS NOT NULL"}
	args := make([]interface{}, 0, 2)
	if startDate != nil {
		parts = append(parts, column+" >= ?")
		args = append(args, *startDate)
	}
	if endDate != nil {
		parts = append(parts, column+" <= ?")
		args = append(args, *endDate)
	}
	return strings.Join(parts, " AND "), args
}

// cancelledContractCondition matches cancelled contracts. The contracts table
// has no cancelled_at column, so UpdatedAt (the moment the status became
// cancelled) defines the period membership.
func cancelledContractCondition(startDate, endDate *time.Time) (string, []interface{}) {
	parts := []string{"contracts.status = ?"}
	args := []interface{}{models.ContractStatusCancelled}
	if startDate != nil {
		parts = append(parts, "contracts.updated_at >= ?")
		args = append(args, *startDate)
	}
	if endDate != nil {
		parts = append(parts, "contracts.updated_at <= ?")
		args = append(args, *endDate)
	}
	return strings.Join(parts, " AND "), args
}

// GetContractPeriodMetrics counts contract activity in the period. See
// models.ContractPeriodMetrics for the counting rules.
func (r *analyticsRepository) GetContractPeriodMetrics(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.ContractPeriodMetrics, error) {
	approvedFilter, approvedArgs := contractPeriodCondition("contracts.approved_at", startDate, endDate)
	closedFilter, closedArgs := contractPeriodCondition("contracts.closed_at", startDate, endDate)
	cancelledFilter, cancelledArgs := cancelledContractCondition(startDate, endDate)
	rejectedFilter, rejectedArgs := contractPeriodCondition("contracts.rejected_at", startDate, endDate)

	selects := []string{
		"COALESCE(SUM(CASE WHEN " + approvedFilter + " THEN 1 ELSE 0 END), 0) AS approved_count",
		"COALESCE(SUM(CASE WHEN " + closedFilter + " THEN 1 ELSE 0 END), 0) AS closed_count",
		"COALESCE(SUM(CASE WHEN " + cancelledFilter + " THEN 1 ELSE 0 END), 0) AS cancelled_count",
		"COALESCE(SUM(CASE WHEN " + rejectedFilter + " THEN 1 ELSE 0 END), 0) AS rejected_count",
		"COALESCE(SUM(CASE WHEN " + approvedFilter + " THEN COALESCE(contracts.amount, 0) ELSE 0 END), 0) AS sales_value",
	}

	// Arguments follow the order the placeholders appear in the SELECT clause.
	args := make([]interface{}, 0, len(approvedArgs)*2+len(closedArgs)+len(cancelledArgs)+len(rejectedArgs))
	args = append(args, approvedArgs...)
	args = append(args, closedArgs...)
	args = append(args, cancelledArgs...)
	args = append(args, rejectedArgs...)
	args = append(args, approvedArgs...)

	query := r.db.WithContext(ctx).Table("contracts").Select(strings.Join(selects, ", "), args...)
	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}
	query = applyContractSellerFilter(query, sellerID)

	var row struct {
		ApprovedCount  int64   `gorm:"column:approved_count"`
		ClosedCount    int64   `gorm:"column:closed_count"`
		CancelledCount int64   `gorm:"column:cancelled_count"`
		RejectedCount  int64   `gorm:"column:rejected_count"`
		SalesValue     float64 `gorm:"column:sales_value"`
	}
	if err := query.Scan(&row).Error; err != nil {
		return models.ContractPeriodMetrics{}, err
	}

	return models.ContractPeriodMetrics{
		Approved:   int(row.ApprovedCount),
		Closed:     int(row.ClosedCount),
		Cancelled:  int(row.CancelledCount),
		Rejected:   int(row.RejectedCount),
		SalesValue: row.SalesValue,
	}, nil
}

// GetFinancingSales groups the contracts approved in the period by financing
// type. It uses the same approved-at predicate as GetContractPeriodMetrics so
// TotalValue matches SalesValue.
func (r *analyticsRepository) GetFinancingSales(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.FinancingSales, error) {
	query := r.db.WithContext(ctx).Table("contracts").
		Select("contracts.financing_type AS financing_type, COUNT(*) AS count, COALESCE(SUM(contracts.amount), 0) AS value").
		Where("contracts.approved_at IS NOT NULL")
	query = applyDateRange(query, "contracts.approved_at", startDate, endDate)
	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}
	query = applyContractSellerFilter(query, sellerID)
	query = query.Group("contracts.financing_type").
		Order("value DESC, contracts.financing_type ASC")

	rows := []models.FinancingSalesRow{}
	if err := query.Scan(&rows).Error; err != nil {
		return models.FinancingSales{}, err
	}

	sales := models.FinancingSales{Metrics: rows}
	for _, row := range rows {
		sales.TotalCount += row.Count
		sales.TotalValue += row.Value
	}
	return sales, nil
}

// GetCollectionsSummary aggregates paid payments by type and compares them with
// the pending amount that was due in the same period.
func (r *analyticsRepository) GetCollectionsSummary(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) (models.CollectionsSummary, error) {
	rows := []struct {
		PaymentType string
		Count       int64
		Amount      float64
		Interest    float64
	}{}

	query := r.db.WithContext(ctx).Table("payments").
		Select("payments.payment_type AS payment_type, COUNT(*) AS count, "+
			"COALESCE(SUM(payments.paid_amount), 0) AS amount, "+
			"COALESCE(SUM(payments.interest_amount), 0) AS interest").
		Where("payments.status = ?", models.PaymentStatusPaid)
	query = applyDateRange(query, "payments.payment_date", startDate, endDate)
	query = applyPaymentScope(query, projectID, sellerID)
	query = query.Group("payments.payment_type").
		Order("amount DESC, payments.payment_type ASC")

	if err := query.Scan(&rows).Error; err != nil {
		return models.CollectionsSummary{}, err
	}

	summary := models.CollectionsSummary{ByType: make([]models.CollectionTypeRow, 0, len(rows))}
	for _, row := range rows {
		summary.ByType = append(summary.ByType, models.CollectionTypeRow{
			PaymentType: row.PaymentType,
			Count:       int(row.Count),
			Amount:      row.Amount,
		})
		summary.Total += row.Amount
		summary.InterestCollected += row.Interest
	}

	var projected float64
	projectedQuery := r.db.WithContext(ctx).Table("payments").
		Select("COALESCE(SUM(payments.amount), 0)").
		Where("payments.status = ?", models.PaymentStatusPending)
	projectedQuery = applyDateRange(projectedQuery, "payments.due_date", startDate, endDate)
	projectedQuery = applyPaymentScope(projectedQuery, projectID, sellerID)
	if err := projectedQuery.Scan(&projected).Error; err != nil {
		return models.CollectionsSummary{}, err
	}

	summary.ProjectedDue = projected
	summary.Effectiveness = models.Effectiveness(summary.Total, projected)
	return summary, nil
}

// GetLedgerTotalsByEntryType sums the signed ledger amounts per entry type for
// the entry dates inside the period. Rows are returned in a fixed order that
// includes the types with no movements (total 0).
func (r *analyticsRepository) GetLedgerTotalsByEntryType(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) ([]models.LedgerTypeTotal, error) {
	rows := []models.LedgerTypeTotal{}

	query := r.db.WithContext(ctx).Table("contract_ledger_entries").
		Select("contract_ledger_entries.entry_type AS entry_type, " +
			"COALESCE(SUM(contract_ledger_entries.amount), 0) AS total")
	if projectID != nil || sellerID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = contract_ledger_entries.contract_id")
	}
	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}
	query = applyContractSellerFilter(query, sellerID)
	query = applyDateRange(query, "contract_ledger_entries.entry_date", startDate, endDate)
	query = query.Group("contract_ledger_entries.entry_type").
		Order("contract_ledger_entries.entry_type ASC")

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	totals := make(map[string]float64, len(rows))
	for _, row := range rows {
		totals[row.EntryType] = row.Total
	}

	result := make([]models.LedgerTypeTotal, 0, len(ledgerEntryTypeOrder)+len(rows))
	seen := make(map[string]bool, len(rows))
	for _, entryType := range ledgerEntryTypeOrder {
		result = append(result, models.LedgerTypeTotal{EntryType: entryType, Total: totals[entryType]})
		seen[entryType] = true
	}
	// Preserve any entry type not covered by the constants instead of
	// silently dropping it from the report.
	for _, row := range rows {
		if !seen[row.EntryType] {
			result = append(result, row)
			seen[row.EntryType] = true
		}
	}
	return result, nil
}

// GetPortfolioSnapshot classifies the cached balances of approved and closed
// contracts. OutstandingBalance is returned as a positive debt magnitude.
func (r *analyticsRepository) GetPortfolioSnapshot(ctx context.Context, projectID *uint, sellerID *uint) (models.PortfolioSnapshot, error) {
	query := r.db.WithContext(ctx).Table("contracts").
		Select("COALESCE(SUM(CASE WHEN contracts.balance < 0 THEN contracts.balance ELSE 0 END), 0) AS outstanding_debt, "+
			"COALESCE(SUM(CASE WHEN contracts.balance >= 0 THEN 1 ELSE 0 END), 0) AS settled_contracts").
		Where("contracts.status IN ?", []string{models.ContractStatusApproved, models.ContractStatusClosed})
	if projectID != nil {
		query = query.Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}
	query = applyContractSellerFilter(query, sellerID)

	var row struct {
		OutstandingDebt  float64 `gorm:"column:outstanding_debt"`
		SettledContracts int64   `gorm:"column:settled_contracts"`
	}
	if err := query.Scan(&row).Error; err != nil {
		return models.PortfolioSnapshot{}, err
	}

	// Debt is stored as a negative ledger balance; the report shows the
	// outstanding amount as a positive magnitude.
	return models.PortfolioSnapshot{
		OutstandingBalance: -row.OutstandingDebt,
		SettledContracts:   int(row.SettledContracts),
	}, nil
}

// monthlyCollectionRow is one (month, payment type) bucket of paid payments.
type monthlyCollectionRow struct {
	Month       time.Time
	PaymentType string
	Count       int64
	Amount      float64
}

// monthlyProjectionRow is the pending amount due in one calendar month.
type monthlyProjectionRow struct {
	Month time.Time
	Total float64
}

// monthKey renders the calendar month of a timestamp for bucket lookups.
func monthKey(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// monthStart normalizes a timestamp to the first day of its month in UTC.
func monthStart(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// incomeFlowMonthRange returns every calendar month covered by the requested
// period. When a bound is missing it falls back to the observed data range.
func incomeFlowMonthRange(startDate, endDate *time.Time, actual []monthlyCollectionRow, projected []monthlyProjectionRow) []time.Time {
	var start, end time.Time
	if startDate != nil {
		start = monthStart(*startDate)
	} else {
		for _, row := range actual {
			if month := monthStart(row.Month); start.IsZero() || month.Before(start) {
				start = month
			}
		}
		for _, row := range projected {
			if month := monthStart(row.Month); start.IsZero() || month.Before(start) {
				start = month
			}
		}
	}
	if endDate != nil {
		end = monthStart(*endDate)
	} else {
		for _, row := range actual {
			if month := monthStart(row.Month); end.IsZero() || month.After(end) {
				end = month
			}
		}
		for _, row := range projected {
			if month := monthStart(row.Month); end.IsZero() || month.After(end) {
				end = month
			}
		}
	}
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}

	months := make([]time.Time, 0, 12)
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		months = append(months, month)
	}
	return months
}

// buildIncomeFlowMonths merges the actual and projected buckets into one row
// per calendar month. Missing months stay in the result with zero values.
func buildIncomeFlowMonths(months []time.Time, actual []monthlyCollectionRow, projected []monthlyProjectionRow) []models.IncomeFlowMonth {
	projectedByMonth := make(map[string]float64, len(projected))
	for _, row := range projected {
		projectedByMonth[monthKey(row.Month)] += row.Total
	}
	actualByMonth := make(map[string][]monthlyCollectionRow, len(actual))
	for _, row := range actual {
		key := monthKey(row.Month)
		actualByMonth[key] = append(actualByMonth[key], row)
	}

	flow := make([]models.IncomeFlowMonth, 0, len(months))
	for _, month := range months {
		key := monthKey(month)
		entry := models.IncomeFlowMonth{
			Month:     month,
			Projected: projectedByMonth[key],
			ByType:    []models.CollectionTypeRow{},
		}
		for _, row := range actualByMonth[key] {
			entry.Actual += row.Amount
			entry.ByType = append(entry.ByType, models.CollectionTypeRow{
				PaymentType: row.PaymentType,
				Count:       int(row.Count),
				Amount:      row.Amount,
			})
		}
		entry.Deviation = entry.Actual - entry.Projected
		entry.Effectiveness = models.Effectiveness(entry.Actual, entry.Projected)
		flow = append(flow, entry)
	}
	return flow
}

// GetIncomeFlowMonthly returns one row per calendar month with actual
// collections (paid payments by payment_date), projected collections (pending
// payments by due_date), deviation, effectiveness and the payment-type
// breakdown of the actual collections.
func (r *analyticsRepository) GetIncomeFlowMonthly(ctx context.Context, projectID *uint, sellerID *uint, startDate, endDate *time.Time) ([]models.IncomeFlowMonth, error) {
	actual := []monthlyCollectionRow{}
	actualQuery := r.db.WithContext(ctx).Table("payments").
		Select("DATE_TRUNC('month', payments.payment_date) AS month, payments.payment_type AS payment_type, "+
			"COUNT(*) AS count, COALESCE(SUM(payments.paid_amount), 0) AS amount").
		Where("payments.status = ?", models.PaymentStatusPaid).
		Where("payments.payment_date IS NOT NULL")
	actualQuery = applyDateRange(actualQuery, "payments.payment_date", startDate, endDate)
	actualQuery = applyPaymentScope(actualQuery, projectID, sellerID)
	actualQuery = actualQuery.Group("DATE_TRUNC('month', payments.payment_date), payments.payment_type").
		Order("month ASC, amount DESC")
	if err := actualQuery.Scan(&actual).Error; err != nil {
		return nil, err
	}

	projected := []monthlyProjectionRow{}
	projectedQuery := r.db.WithContext(ctx).Table("payments").
		Select("DATE_TRUNC('month', payments.due_date) AS month, COALESCE(SUM(payments.amount), 0) AS total").
		Where("payments.status = ?", models.PaymentStatusPending)
	projectedQuery = applyDateRange(projectedQuery, "payments.due_date", startDate, endDate)
	projectedQuery = applyPaymentScope(projectedQuery, projectID, sellerID)
	projectedQuery = projectedQuery.Group("DATE_TRUNC('month', payments.due_date)").
		Order("month ASC")
	if err := projectedQuery.Scan(&projected).Error; err != nil {
		return nil, err
	}

	months := incomeFlowMonthRange(startDate, endDate, actual, projected)
	return buildIncomeFlowMonths(months, actual, projected), nil
}

// GetCollectionsByProject groups the paid payments of the period by project.
func (r *analyticsRepository) GetCollectionsByProject(ctx context.Context, sellerID *uint, startDate, endDate *time.Time) ([]models.ProjectCollections, error) {
	rows := []struct {
		ProjectID   uint
		ProjectName string
		Amount      float64
		Count       int64
	}{}

	query := r.db.WithContext(ctx).Table("payments").
		Select("lots.project_id AS project_id, projects.name AS project_name, "+
			"COALESCE(SUM(payments.paid_amount), 0) AS amount, COUNT(*) AS count").
		Joins("JOIN contracts ON contracts.id = payments.contract_id").
		Joins("JOIN lots ON lots.id = contracts.lot_id").
		Joins("JOIN projects ON projects.id = lots.project_id").
		Where("payments.status = ?", models.PaymentStatusPaid)
	query = applyDateRange(query, "payments.payment_date", startDate, endDate)
	query = applyContractSellerFilter(query, sellerID)
	query = query.Group("lots.project_id, projects.name").
		Order("amount DESC, projects.name ASC")

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	collections := make([]models.ProjectCollections, 0, len(rows))
	for _, row := range rows {
		collections = append(collections, models.ProjectCollections{
			ProjectID:   row.ProjectID,
			ProjectName: row.ProjectName,
			Amount:      row.Amount,
			Count:       int(row.Count),
		})
	}
	return collections, nil
}
