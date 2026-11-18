package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AnalyticsCacheEntry is one cache row written by SetCacheEntries.
type AnalyticsCacheEntry struct {
	Key       string
	ProjectID *uint
	Data      interface{}
	TTL       time.Duration
}

type AnalyticsRepository interface {
	GetCache(ctx context.Context, key string, projectID *uint) (*models.AnalyticsCache, error)
	SetCache(ctx context.Context, key string, projectID *uint, data interface{}, ttl time.Duration) error
	SetCacheEntries(ctx context.Context, entries []AnalyticsCacheEntry) error
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

	// Batched refresh helpers: one aggregate query covers every project so the
	// scheduled job cost does not grow with the number of projects.
	GetProjectOverviewMetrics(ctx context.Context, curStart, curEnd, prevStart, prevEnd *time.Time) ([]models.ProjectOverviewMetrics, error)
	GetProjectLotDistributions(ctx context.Context) (map[uint]*models.LotDistribution, error)
	GetRevenueTrendByProject(ctx context.Context, timeframe string, year *int) (map[uint][]models.RevenueTrendPoint, error)
	GetWeeklyPerformanceByProject(ctx context.Context) (map[uint][]float64, error)

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
	return r.SetCacheEntries(ctx, []AnalyticsCacheEntry{{Key: key, ProjectID: projectID, Data: data, TTL: ttl}})
}

// SetCacheEntries writes every entry with at most two upserts, one for
// project-scoped rows and one for global rows, because Postgres allows a single
// conflict target per statement. The upserts keep concurrent refreshes from
// inserting duplicates. Global entries have a NULL project_id, which needs the
// partial unique index and an explicit target predicate.
func (r *analyticsRepository) SetCacheEntries(ctx context.Context, entries []AnalyticsCacheEntry) error {
	if len(entries) == 0 {
		return nil
	}

	now := time.Now()
	var scoped, global []models.AnalyticsCache
	for _, entry := range entries {
		jsonData, err := json.Marshal(entry.Data)
		if err != nil {
			return err
		}
		row := models.AnalyticsCache{
			CacheKey:  entry.Key,
			ProjectID: entry.ProjectID,
			Data:      jsonData,
			ExpiresAt: now.Add(entry.TTL),
		}
		if entry.ProjectID != nil {
			scoped = append(scoped, row)
		} else {
			global = append(global, row)
		}
	}

	update := clause.AssignmentColumns([]string{"data", "expires_at", "updated_at"})
	db := r.db.WithContext(ctx)

	if len(scoped) > 0 {
		conflict := clause.OnConflict{
			Columns:   []clause.Column{{Name: "cache_key"}, {Name: "project_id"}},
			DoUpdates: update,
		}
		if err := db.Clauses(conflict).Create(&scoped).Error; err != nil {
			return err
		}
	}

	if len(global) > 0 {
		conflict := clause.OnConflict{
			Columns: []clause.Column{{Name: "cache_key"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{
				clause.Expr{SQL: "project_id IS NULL"},
			}},
			DoUpdates: update,
		}
		if err := db.Clauses(conflict).Create(&global).Error; err != nil {
			return err
		}
	}

	return nil
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

// revenueTrendRow is one calendar-month bucket of a trend query.
type revenueTrendRow struct {
	Month time.Time
	Label string
	Total float64
}

// trendRange resolves the trend window. A specific year wins; otherwise the
// window spans the requested timeframe around today so future projections are
// included.
func trendRange(timeframe string, year *int) (time.Time, time.Time) {
	if year != nil {
		return time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(*year, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	months := 12
	if timeframe == "6M" {
		months = 6
	}
	now := time.Now()
	return now.AddDate(0, -months, 0), now.AddDate(0, months, 0)
}

// mergeRevenueTrend combines real and projected rows by calendar month. The
// month key includes the year, so the same month in different years stays
// distinct, and the result is sorted chronologically.
func mergeRevenueTrend(realRows, projectedRows []revenueTrendRow) []models.RevenueTrendPoint {
	type bucket struct {
		month time.Time
		point models.RevenueTrendPoint
	}

	byMonth := make(map[string]*bucket, len(realRows)+len(projectedRows))
	add := func(row revenueTrendRow) *bucket {
		month := monthStart(row.Month)
		key := monthKey(month)
		b, ok := byMonth[key]
		if !ok {
			label := row.Label
			if label == "" {
				label = month.Format("Jan")
			}
			b = &bucket{month: month, point: models.RevenueTrendPoint{Label: label}}
			byMonth[key] = b
		}
		return b
	}

	for _, row := range realRows {
		add(row).point.Real += row.Total
	}
	for _, row := range projectedRows {
		add(row).point.Projected += row.Total
	}

	buckets := make([]*bucket, 0, len(byMonth))
	for _, b := range byMonth {
		buckets = append(buckets, b)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].month.Before(buckets[j].month)
	})

	points := make([]models.RevenueTrendPoint, 0, len(buckets))
	for _, b := range buckets {
		points = append(points, b.point)
	}
	return points
}

// revenueTrendRows runs one trend aggregate. column and amountColumn are
// hard-coded by the caller, never user input.
func (r *analyticsRepository) revenueTrendRows(ctx context.Context, projectID *uint, column, status, amountColumn string, startDate, endDate time.Time) ([]revenueTrendRow, error) {
	query := r.db.WithContext(ctx).Table("payments").
		Select("DATE_TRUNC('month', payments."+column+") AS month, TO_CHAR(payments."+column+", 'Mon') AS label, COALESCE(SUM(payments."+amountColumn+"), 0) AS total").
		Where("payments.status = ?", status).
		Where("payments."+column+" >= ?", startDate).
		Where("payments."+column+" <= ?", endDate).
		Group("DATE_TRUNC('month', payments." + column + "), TO_CHAR(payments." + column + ", 'Mon')").
		Order("month ASC")

	if projectID != nil {
		query = query.Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("lots.project_id = ?", *projectID)
	}

	var rows []revenueTrendRow
	err := query.Scan(&rows).Error
	return rows, err
}

func (r *analyticsRepository) GetRevenueTrend(ctx context.Context, projectID *uint, timeframe string, year *int) ([]models.RevenueTrendPoint, error) {
	startDate, endDate := trendRange(timeframe, year)

	realRows, err := r.revenueTrendRows(ctx, projectID, "payment_date", models.PaymentStatusPaid, "paid_amount", startDate, endDate)
	if err != nil {
		return nil, err
	}

	projectedRows, err := r.revenueTrendRows(ctx, projectID, "due_date", models.PaymentStatusPending, "amount", startDate, endDate)
	if err != nil {
		return nil, err
	}

	return mergeRevenueTrend(realRows, projectedRows), nil
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

// GetProjectLotDistributions returns the lot distribution for every project in
// one grouped query. Projects without lots are present with zero totals.
func (r *analyticsRepository) GetProjectLotDistributions(ctx context.Context) (map[uint]*models.LotDistribution, error) {
	rows := []struct {
		ProjectID uint
		Status    string
		Count     int
	}{}

	err := r.db.WithContext(ctx).Table("projects").
		Select("projects.id AS project_id, COALESCE(lots.status, '') AS status, COUNT(lots.id) AS count").
		Joins("LEFT JOIN lots ON lots.project_id = projects.id").
		Group("projects.id, lots.status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]*models.LotDistribution, len(rows))
	for _, row := range rows {
		dist, ok := result[row.ProjectID]
		if !ok {
			dist = &models.LotDistribution{}
			result[row.ProjectID] = dist
		}
		dist.TotalLots += row.Count
		switch row.Status {
		case models.LotStatusAvailable:
			dist.Available += row.Count
		case models.LotStatusReserved:
			dist.Reserved += row.Count
		case models.LotStatusFinanced:
			dist.Financed += row.Count
		case models.LotStatusFullyPaid:
			dist.FullyPaid += row.Count
		}
	}

	for _, dist := range result {
		applyLotPercentages(dist)
	}
	return result, nil
}

// applyLotPercentages fills the derived percentages of a distribution.
func applyLotPercentages(dist *models.LotDistribution) {
	if dist.TotalLots == 0 {
		return
	}
	total := float64(dist.TotalLots)
	dist.AvailablePercentage = (float64(dist.Available) / total) * 100
	dist.ReservedPercentage = (float64(dist.Reserved) / total) * 100
	dist.FinancedPercentage = (float64(dist.Financed) / total) * 100
	dist.FullyPaidPercentage = (float64(dist.FullyPaid) / total) * 100
}

// GetProjectOverviewMetrics aggregates the current and previous overview
// metrics for every project in one statement, so the refresh job no longer
// queries project by project.
func (r *analyticsRepository) GetProjectOverviewMetrics(ctx context.Context, curStart, curEnd, prevStart, prevEnd *time.Time) ([]models.ProjectOverviewMetrics, error) {
	if curStart == nil || curEnd == nil || prevStart == nil || prevEnd == nil {
		return nil, fmt.Errorf("comparison periods are required")
	}

	const query = `
SELECT p.id AS project_id,
       COALESCE((SELECT SUM(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id), 0) AS total_revenue,
       COALESCE((SELECT COUNT(*) FROM contracts c
                 JOIN lots l ON l.id = c.lot_id
                 WHERE c.status = ? AND c.active = true AND l.project_id = p.id), 0) AS active_contracts,
       COALESCE((SELECT AVG(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id), 0) AS average_payment,
       COALESCE((SELECT COUNT(*) FROM lots l WHERE l.project_id = p.id), 0) AS total_lots,
       COALESCE((SELECT COUNT(*) FROM lots l WHERE l.project_id = p.id AND l.status IN ?), 0) AS occupied_lots,
       COALESCE((SELECT SUM(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id
                   AND pay.payment_date >= ? AND pay.payment_date <= ?), 0) AS previous_revenue,
       COALESCE((SELECT COUNT(*) FROM contracts c
                 JOIN lots l ON l.id = c.lot_id
                 WHERE c.status = ? AND c.active = true AND l.project_id = p.id
                   AND c.approved_at >= ? AND c.approved_at <= ?), 0) AS previous_contracts,
       COALESCE((SELECT AVG(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id
                   AND pay.payment_date >= ? AND pay.payment_date <= ?), 0) AS previous_average_payment,
       COALESCE((SELECT COUNT(DISTINCT c.lot_id) FROM contracts c
                 JOIN lots l ON l.id = c.lot_id
                 WHERE c.status IN ? AND l.project_id = p.id
                   AND c.approved_at IS NOT NULL AND c.approved_at <= ?
                   AND (c.closed_at IS NULL OR c.closed_at > ?)), 0) AS previous_occupied_lots,
       COALESCE((SELECT SUM(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id
                   AND pay.payment_date >= ? AND pay.payment_date <= ?), 0) AS comparison_revenue,
       COALESCE((SELECT COUNT(*) FROM contracts c
                 JOIN lots l ON l.id = c.lot_id
                 WHERE c.status = ? AND c.active = true AND l.project_id = p.id
                   AND c.approved_at >= ? AND c.approved_at <= ?), 0) AS comparison_contracts,
       COALESCE((SELECT AVG(pay.paid_amount) FROM payments pay
                 JOIN contracts c ON c.id = pay.contract_id
                 JOIN lots l ON l.id = c.lot_id
                 WHERE pay.status = ? AND l.project_id = p.id
                   AND pay.payment_date >= ? AND pay.payment_date <= ?), 0) AS comparison_avg_payment
FROM projects p
ORDER BY p.id`

	occupiedStatuses := []string{
		models.LotStatusReserved,
		models.LotStatusFinanced,
		models.LotStatusFullyPaid,
	}
	portfolioStatuses := []string{
		models.ContractStatusApproved,
		models.ContractStatusClosed,
	}

	var rows []models.ProjectOverviewMetrics
	err := r.db.WithContext(ctx).Raw(query,
		models.PaymentStatusPaid,
		models.ContractStatusApproved,
		models.PaymentStatusPaid,
		occupiedStatuses,
		models.PaymentStatusPaid, *prevStart, *prevEnd,
		models.ContractStatusApproved, *prevStart, *prevEnd,
		models.PaymentStatusPaid, *prevStart, *prevEnd,
		portfolioStatuses, *prevEnd, *prevEnd,
		models.PaymentStatusPaid, *curStart, *curEnd,
		models.ContractStatusApproved, *curStart, *curEnd,
		models.PaymentStatusPaid, *curStart, *curEnd,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetWeeklyPerformanceByProject returns the paid collections of the last four
// weeks per project, in chronological order. Projects without collections are
// absent so callers can omit the series instead of fabricating one.
func (r *analyticsRepository) GetWeeklyPerformanceByProject(ctx context.Context) (map[uint][]float64, error) {
	since := time.Now().AddDate(0, 0, -28)

	rows := []struct {
		ProjectID uint
		Week      time.Time
		Total     float64
	}{}

	err := r.db.WithContext(ctx).Table("payments").
		Select("lots.project_id AS project_id, DATE_TRUNC('week', payments.payment_date) AS week, COALESCE(SUM(payments.paid_amount), 0) AS total").
		Joins("JOIN contracts ON contracts.id = payments.contract_id").
		Joins("JOIN lots ON lots.id = contracts.lot_id").
		Where("payments.status = ?", models.PaymentStatusPaid).
		Where("payments.payment_date >= ?", since).
		Group("lots.project_id, DATE_TRUNC('week', payments.payment_date)").
		Order("lots.project_id ASC, week ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint][]float64)
	for _, row := range rows {
		result[row.ProjectID] = append(result[row.ProjectID], row.Total)
	}
	return result, nil
}

// GetRevenueTrendByProject computes the revenue trend for every project with
// two bounded queries (paid and pending) and merges them per project.
func (r *analyticsRepository) GetRevenueTrendByProject(ctx context.Context, timeframe string, year *int) (map[uint][]models.RevenueTrendPoint, error) {
	startDate, endDate := trendRange(timeframe, year)

	type projectTrendRow struct {
		ProjectID uint
		Month     time.Time
		Label     string
		Total     float64
	}

	run := func(column, status, amountColumn string) ([]projectTrendRow, error) {
		query := r.db.WithContext(ctx).Table("payments").
			Select("lots.project_id AS project_id, DATE_TRUNC('month', payments."+column+") AS month, TO_CHAR(payments."+column+", 'Mon') AS label, COALESCE(SUM(payments."+amountColumn+"), 0) AS total").
			Joins("JOIN contracts ON contracts.id = payments.contract_id").
			Joins("JOIN lots ON lots.id = contracts.lot_id").
			Where("payments.status = ?", status).
			Where("payments."+column+" >= ?", startDate).
			Where("payments."+column+" <= ?", endDate).
			Group("lots.project_id, DATE_TRUNC('month', payments." + column + "), TO_CHAR(payments." + column + ", 'Mon')").
			Order("lots.project_id ASC, month ASC")
		var rows []projectTrendRow
		err := query.Scan(&rows).Error
		return rows, err
	}

	realRows, err := run("payment_date", models.PaymentStatusPaid, "paid_amount")
	if err != nil {
		return nil, err
	}
	projectedRows, err := run("due_date", models.PaymentStatusPending, "amount")
	if err != nil {
		return nil, err
	}

	realByProject := make(map[uint][]revenueTrendRow)
	for _, row := range realRows {
		realByProject[row.ProjectID] = append(realByProject[row.ProjectID], revenueTrendRow{
			Month: row.Month,
			Label: row.Label,
			Total: row.Total,
		})
	}
	projectedByProject := make(map[uint][]revenueTrendRow)
	for _, row := range projectedRows {
		projectedByProject[row.ProjectID] = append(projectedByProject[row.ProjectID], revenueTrendRow{
			Month: row.Month,
			Label: row.Label,
			Total: row.Total,
		})
	}

	projectIDs := make(map[uint]struct{}, len(realByProject)+len(projectedByProject))
	for id := range realByProject {
		projectIDs[id] = struct{}{}
	}
	for id := range projectedByProject {
		projectIDs[id] = struct{}{}
	}

	result := make(map[uint][]models.RevenueTrendPoint, len(projectIDs))
	for id := range projectIDs {
		result[id] = mergeRevenueTrend(realByProject[id], projectedByProject[id])
	}
	return result, nil
}

func (r *analyticsRepository) GetProjectPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.ProjectPerformance, error) {
	rows := []struct {
		ProjectID   uint
		ProjectName string
		TotalLots   int
		SoldLots    int
	}{}

	query := r.db.WithContext(ctx).Table("projects").
		Select("projects.id AS project_id, projects.name AS project_name, COUNT(lots.id) AS total_lots, "+
			"COALESCE(SUM(CASE WHEN lots.status IN ? THEN 1 ELSE 0 END), 0) AS sold_lots",
			[]string{models.LotStatusFinanced, models.LotStatusFullyPaid}).
		Joins("LEFT JOIN lots ON lots.project_id = projects.id").
		Group("projects.id, projects.name").
		Order("projects.id ASC")
	if filters.ProjectID != nil {
		query = query.Where("projects.id = ?", *filters.ProjectID)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	weekly, err := r.GetWeeklyPerformanceByProject(ctx)
	if err != nil {
		return nil, err
	}

	performance := make([]models.ProjectPerformance, 0, len(rows))
	for _, row := range rows {
		growth := 0.0
		if row.TotalLots > 0 {
			growth = (float64(row.SoldLots) / float64(row.TotalLots)) * 100
		}

		performance = append(performance, models.ProjectPerformance{
			ProjectID:         strconv.FormatUint(uint64(row.ProjectID), 10),
			ProjectName:       row.ProjectName,
			GrowthPercentage:  growth,
			WeeklyPerformance: weekly[row.ProjectID],
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
