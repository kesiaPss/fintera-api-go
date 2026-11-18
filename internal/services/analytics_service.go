package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/pkg/logger"
)

type AnalyticsService struct {
	analyticsRepo   repository.AnalyticsRepository
	projectRepo     repository.ProjectRepository
	notificationSvc *NotificationService
	userRepo        repository.UserRepository
}

func NewAnalyticsService(
	analyticsRepo repository.AnalyticsRepository,
	projectRepo repository.ProjectRepository,
	notificationSvc *NotificationService,
	userRepo repository.UserRepository,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo:   analyticsRepo,
		projectRepo:     projectRepo,
		notificationSvc: notificationSvc,
		userRepo:        userRepo,
	}
}

func (s *AnalyticsService) GetOverview(ctx context.Context, filters models.AnalyticsFilters) (*models.AnalyticsOverview, error) {
	cacheKey := "analytics_overview"
	if filters.RevenueTimeframe != "" {
		cacheKey += "_" + filters.RevenueTimeframe
	}

	// Bypass cache when date/year filters are present so the analytics page filters work correctly.
	if !filters.HasDateFilter() {
		cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, filters.ProjectID)
		if err == nil && cached != nil {
			var overview models.AnalyticsOverview
			if err := json.Unmarshal(cached.Data, &overview); err == nil {
				return &overview, nil
			}
		}
	}

	// Compute overview
	overview, err := s.computeOverview(ctx, filters)
	if err != nil {
		return nil, err
	}

	// Set cache only when no date filter (dashboard-style requests)
	if !filters.HasDateFilter() {
		_ = s.analyticsRepo.SetCache(ctx, cacheKey, filters.ProjectID, overview, 15*time.Minute)
	}

	return overview, nil
}

// calculatePercentageChange computes the percentage difference between current and previous values
func calculatePercentageChange(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	change := ((current - previous) / previous) * 100
	// Round to 1 decimal place
	return float64(int(change*10+0.5)) / 10
}

// defaultComparisonWindow returns the most recent complete 30-day window.
func defaultComparisonWindow() (time.Time, time.Time) {
	now := time.Now()
	return now.AddDate(0, 0, -30), now
}

// getPreviousPeriod returns the window immediately before the current one.
// The current window comes from the filters; when a bound is missing it is
// inferred with a 30-day length, and when no bound is given it is the most
// recent complete 30-day window.
func getPreviousPeriod(startDate, endDate *time.Time) (prevStart, prevEnd *time.Time) {
	var currentStart, currentEnd time.Time
	switch {
	case startDate != nil && endDate != nil:
		currentStart, currentEnd = *startDate, *endDate
	case startDate != nil:
		currentStart = *startDate
		currentEnd = startDate.AddDate(0, 0, 30)
	case endDate != nil:
		currentEnd = *endDate
		currentStart = endDate.AddDate(0, 0, -30)
	default:
		currentStart, currentEnd = defaultComparisonWindow()
	}

	duration := currentEnd.Sub(currentStart)
	prevEndTime := currentStart.AddDate(0, 0, -1)
	prevStartTime := prevEndTime.Add(-duration)

	return &prevStartTime, &prevEndTime
}

// currentPeriodFromPrevious reconstructs the current window that follows the
// previous one, so the refresh job can compute like-for-like comparisons.
func currentPeriodFromPrevious(prevStart, prevEnd *time.Time) (time.Time, time.Time) {
	currentStart := prevEnd.AddDate(0, 0, 1)
	duration := prevEnd.Sub(*prevStart)
	return currentStart, currentStart.Add(duration)
}

func (s *AnalyticsService) computeOverview(ctx context.Context, filters models.AnalyticsFilters) (*models.AnalyticsOverview, error) {
	// Current period stats
	totalRevenue, err := s.analyticsRepo.GetTotalRevenue(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	activeContracts, err := s.analyticsRepo.GetActiveContractsCount(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	avgPayment, err := s.analyticsRepo.GetAveragePayment(ctx, filters.ProjectID, filters.StartDate, filters.EndDate)
	if err != nil {
		return nil, err
	}

	occupancyRate, err := s.analyticsRepo.GetOccupancyRate(ctx, filters.ProjectID, filters.EndDate)
	if err != nil {
		return nil, err
	}

	revenueTrend, err := s.analyticsRepo.GetRevenueTrend(ctx, filters.ProjectID, filters.RevenueTimeframe, filters.Year)
	if err != nil {
		return nil, err
	}

	// Get previous period data for percentage calculations
	prevStart, prevEnd := getPreviousPeriod(filters.StartDate, filters.EndDate)

	prevRevenue, err := s.analyticsRepo.GetTotalRevenue(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevRevenue = 0
	}

	prevContracts, err := s.analyticsRepo.GetActiveContractsCount(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevContracts = 0
	}

	prevAvgPayment, err := s.analyticsRepo.GetAveragePayment(ctx, filters.ProjectID, prevStart, prevEnd)
	if err != nil {
		prevAvgPayment = 0
	}

	// When no period is selected the displayed totals are all-time, so the
	// change compares two complete 30-day windows instead of mixing an
	// all-time total with an unrelated prior window.
	comparisonRevenue := totalRevenue
	comparisonContracts := activeContracts
	comparisonAvgPayment := avgPayment
	if !filters.HasDateFilter() {
		curStart, curEnd := defaultComparisonWindow()
		if value, err := s.analyticsRepo.GetTotalRevenue(ctx, filters.ProjectID, &curStart, &curEnd); err == nil {
			comparisonRevenue = value
		}
		if value, err := s.analyticsRepo.GetActiveContractsCount(ctx, filters.ProjectID, &curStart, &curEnd); err == nil {
			comparisonContracts = value
		}
		if value, err := s.analyticsRepo.GetAveragePayment(ctx, filters.ProjectID, &curStart, &curEnd); err == nil {
			comparisonAvgPayment = value
		}
	}

	// Previous occupancy is computed for the comparison window whenever one
	// exists, including the default dashboard.
	prevOccupancy := 0.0
	if prevEnd != nil {
		if rate, err := s.analyticsRepo.GetOccupancyRate(ctx, filters.ProjectID, prevEnd); err == nil {
			prevOccupancy = rate
		}
	}

	// Calculate percentage changes
	revenueChange := calculatePercentageChange(comparisonRevenue, prevRevenue)
	contractsChange := calculatePercentageChange(float64(comparisonContracts), float64(prevContracts))
	paymentChange := calculatePercentageChange(comparisonAvgPayment, prevAvgPayment)
	occupancyChange := calculatePercentageChange(occupancyRate, prevOccupancy)

	// Default currency symbol (can be project-specific in the future)
	currencySymbol := "L"

	return &models.AnalyticsOverview{
		TotalRevenue:              totalRevenue,
		PreviousRevenue:           prevRevenue,
		RevenueChangePercentage:   revenueChange,
		ActiveContracts:           activeContracts,
		PreviousContracts:         prevContracts,
		ContractsChangePercentage: contractsChange,
		AveragePayment:            avgPayment,
		PreviousAveragePayment:    prevAvgPayment,
		PaymentChangePercentage:   paymentChange,
		OccupancyRate:             occupancyRate,
		PreviousOccupancyRate:     prevOccupancy,
		OccupancyChangePercentage: occupancyChange,
		CurrencySymbol:            currencySymbol,
		RevenueTrend:              revenueTrend,
	}, nil
}

func (s *AnalyticsService) GetDistribution(ctx context.Context, projectID *uint) (*models.LotDistribution, error) {
	cacheKey := "analytics_distribution"

	// Check cache
	cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, projectID)
	if err == nil && cached != nil {
		var dist models.LotDistribution
		if err := json.Unmarshal(cached.Data, &dist); err == nil {
			return &dist, nil
		}
	}

	dist, err := s.analyticsRepo.GetLotDistribution(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Set cache (15 min TTL)
	_ = s.analyticsRepo.SetCache(ctx, cacheKey, projectID, dist, 15*time.Minute)

	return dist, nil
}

func (s *AnalyticsService) GetPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.ProjectPerformance, error) {
	cacheKey := "analytics_performance"
	if filters.ProjectID != nil {
		cacheKey += fmt.Sprintf("_project_%d", *filters.ProjectID)
	}

	// Bypass cache when date/year filters are present so the analytics page filters work correctly.
	if !filters.HasDateFilter() {
		cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, filters.ProjectID)
		if err == nil && cached != nil {
			var perf []models.ProjectPerformance
			if err := json.Unmarshal(cached.Data, &perf); err == nil {
				return perf, nil
			}
		}
	}

	perf, err := s.analyticsRepo.GetProjectPerformance(ctx, filters)
	if err != nil {
		return nil, err
	}

	if !filters.HasDateFilter() {
		_ = s.analyticsRepo.SetCache(ctx, cacheKey, filters.ProjectID, perf, 30*time.Minute)
	}

	return perf, nil
}

func (s *AnalyticsService) GetSellerPerformance(ctx context.Context, filters models.AnalyticsFilters) ([]models.SellerPerformance, error) {
	cacheKey := "analytics_sellers"

	// Bypass cache when date/year filters are present
	if !filters.HasDateFilter() {
		cached, err := s.analyticsRepo.GetCache(ctx, cacheKey, nil)
		if err == nil && cached != nil {
			var sellers []models.SellerPerformance
			if err := json.Unmarshal(cached.Data, &sellers); err == nil {
				return sellers, nil
			}
		}
	}

	sellers, err := s.analyticsRepo.GetSellerPerformance(ctx, filters)
	if err != nil {
		return nil, err
	}

	if !filters.HasDateFilter() {
		_ = s.analyticsRepo.SetCache(ctx, cacheKey, nil, sellers, 30*time.Minute)
	}

	return sellers, nil
}

func (s *AnalyticsService) RefreshCache(ctx context.Context) error {
	logger.Info("[AnalyticsService] Refreshing analytics cache in background...")

	// Only the project identities are needed; avoid loading every column.
	projects, err := s.projectRepo.FindAllSummaries(ctx)
	if err != nil {
		logger.Error("Failed to fetch projects for cache refresh", "error", err)
		return err
	}

	// Refresh global stats
	_, _ = s.GetOverview(ctx, models.AnalyticsFilters{})
	_, _ = s.GetDistribution(ctx, nil)
	_, _ = s.GetPerformance(ctx, models.AnalyticsFilters{})

	// Refresh every project from a bounded set of aggregate queries instead of
	// querying project by project.
	prevStart, prevEnd := getPreviousPeriod(nil, nil)
	curStart, curEnd := currentPeriodFromPrevious(prevStart, prevEnd)

	metrics, err := s.analyticsRepo.GetProjectOverviewMetrics(ctx, &curStart, &curEnd, prevStart, prevEnd)
	if err != nil {
		logger.Error("Failed to compute project overview metrics", "error", err)
		return err
	}

	distributions, err := s.analyticsRepo.GetProjectLotDistributions(ctx)
	if err != nil {
		logger.Error("Failed to compute project lot distributions", "error", err)
		return err
	}

	trends, err := s.analyticsRepo.GetRevenueTrendByProject(ctx, "", nil)
	if err != nil {
		logger.Error("Failed to compute project revenue trends", "error", err)
		return err
	}

	metricsByProject := make(map[uint]models.ProjectOverviewMetrics, len(metrics))
	for _, metric := range metrics {
		metricsByProject[metric.ProjectID] = metric
	}

	for _, project := range projects {
		pid := project.ID

		overview := buildProjectOverview(metricsByProject[pid], trends[pid])
		_ = s.analyticsRepo.SetCache(ctx, "analytics_overview", &pid, overview, 15*time.Minute)

		dist := distributions[pid]
		if dist == nil {
			dist = &models.LotDistribution{}
		}
		_ = s.analyticsRepo.SetCache(ctx, "analytics_distribution", &pid, dist, 15*time.Minute)
	}

	// Clean up old cache
	_ = s.analyticsRepo.CleanExpiredCache(ctx)

	logger.Info("[AnalyticsService] Analytics cache refresh completed.")
	// No admin notifications here: job runs every 15 min (and per instance in multi-replica setups),
	// which would spam admins. Progress is visible in logs.
	return nil
}

// buildProjectOverview assembles the cached overview for one project from the
// batched metrics and trend rows.
func buildProjectOverview(metric models.ProjectOverviewMetrics, trend []models.RevenueTrendPoint) *models.AnalyticsOverview {
	occupancy := 0.0
	previousOccupancy := 0.0
	if metric.TotalLots > 0 {
		occupancy = (float64(metric.OccupiedLots) / float64(metric.TotalLots)) * 100
		previousOccupancy = (float64(metric.PreviousOccupiedLots) / float64(metric.TotalLots)) * 100
	}

	revenueTrend := trend
	if revenueTrend == nil {
		revenueTrend = []models.RevenueTrendPoint{}
	}

	return &models.AnalyticsOverview{
		TotalRevenue:              metric.TotalRevenue,
		PreviousRevenue:           metric.PreviousRevenue,
		RevenueChangePercentage:   calculatePercentageChange(metric.ComparisonRevenue, metric.PreviousRevenue),
		ActiveContracts:           metric.ActiveContracts,
		PreviousContracts:         metric.PreviousContracts,
		ContractsChangePercentage: calculatePercentageChange(float64(metric.ComparisonContracts), float64(metric.PreviousContracts)),
		AveragePayment:            metric.AveragePayment,
		PreviousAveragePayment:    metric.PreviousAveragePayment,
		PaymentChangePercentage:   calculatePercentageChange(metric.ComparisonAvgPayment, metric.PreviousAveragePayment),
		OccupancyRate:             occupancy,
		PreviousOccupancyRate:     previousOccupancy,
		OccupancyChangePercentage: calculatePercentageChange(occupancy, previousOccupancy),
		CurrencySymbol:            "L",
		RevenueTrend:              revenueTrend,
	}
}
