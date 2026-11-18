package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// fakeAnalyticsRepo implements only the methods used by the refresh path. The
// embedded interface satisfies the rest of repository.AnalyticsRepository.
type fakeAnalyticsRepo struct {
	repository.AnalyticsRepository
	calls   map[string]int
	metrics []models.ProjectOverviewMetrics
	entries []repository.AnalyticsCacheEntry
}

func newFakeAnalyticsRepo() *fakeAnalyticsRepo {
	return &fakeAnalyticsRepo{calls: make(map[string]int)}
}

func (f *fakeAnalyticsRepo) count(name string) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[name]++
}

func (f *fakeAnalyticsRepo) GetCache(context.Context, string, *uint) (*models.AnalyticsCache, error) {
	f.count("GetCache")
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAnalyticsRepo) SetCache(_ context.Context, _ string, _ *uint, _ interface{}, _ time.Duration) error {
	f.count("SetCache")
	return nil
}

func (f *fakeAnalyticsRepo) SetCacheEntries(_ context.Context, entries []repository.AnalyticsCacheEntry) error {
	f.count("SetCacheEntries")
	f.entries = entries
	return nil
}

func (f *fakeAnalyticsRepo) CleanExpiredCache(context.Context) error {
	f.count("CleanExpiredCache")
	return nil
}

func (f *fakeAnalyticsRepo) GetTotalRevenue(context.Context, *uint, *time.Time, *time.Time) (float64, error) {
	f.count("GetTotalRevenue")
	return 0, nil
}

func (f *fakeAnalyticsRepo) GetActiveContractsCount(context.Context, *uint, *time.Time, *time.Time) (int, error) {
	f.count("GetActiveContractsCount")
	return 0, nil
}

func (f *fakeAnalyticsRepo) GetAveragePayment(context.Context, *uint, *time.Time, *time.Time) (float64, error) {
	f.count("GetAveragePayment")
	return 0, nil
}

func (f *fakeAnalyticsRepo) GetOccupancyRate(context.Context, *uint, *time.Time) (float64, error) {
	f.count("GetOccupancyRate")
	return 0, nil
}

func (f *fakeAnalyticsRepo) GetRevenueTrend(context.Context, *uint, string, *int) ([]models.RevenueTrendPoint, error) {
	f.count("GetRevenueTrend")
	return nil, nil
}

func (f *fakeAnalyticsRepo) GetLotDistribution(context.Context, *uint) (*models.LotDistribution, error) {
	f.count("GetLotDistribution")
	return &models.LotDistribution{}, nil
}

func (f *fakeAnalyticsRepo) GetProjectPerformance(context.Context, models.AnalyticsFilters) ([]models.ProjectPerformance, error) {
	f.count("GetProjectPerformance")
	return nil, nil
}

func (f *fakeAnalyticsRepo) GetProjectOverviewMetrics(context.Context, *time.Time, *time.Time, *time.Time, *time.Time) ([]models.ProjectOverviewMetrics, error) {
	f.count("GetProjectOverviewMetrics")
	return f.metrics, nil
}

func (f *fakeAnalyticsRepo) GetProjectLotDistributions(context.Context) (map[uint]*models.LotDistribution, error) {
	f.count("GetProjectLotDistributions")
	return map[uint]*models.LotDistribution{}, nil
}

func (f *fakeAnalyticsRepo) GetRevenueTrendByProject(context.Context, string, *int) (map[uint][]models.RevenueTrendPoint, error) {
	f.count("GetRevenueTrendByProject")
	return map[uint][]models.RevenueTrendPoint{}, nil
}

type fakeProjectRepo struct {
	repository.ProjectRepository
	projects []models.Project
}

func (f *fakeProjectRepo) FindAllSummaries(context.Context) ([]models.Project, error) {
	return f.projects, nil
}

func TestGetPreviousPeriod(t *testing.T) {
	octoberStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	octoberEnd := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	t.Run("explicit month", func(t *testing.T) {
		prevStart, prevEnd := getPreviousPeriod(&octoberStart, &octoberEnd)

		assert.Equal(t, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), *prevEnd)
		assert.Equal(t, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), *prevStart)
	})

	t.Run("only start", func(t *testing.T) {
		prevStart, prevEnd := getPreviousPeriod(&octoberStart, nil)

		assert.Equal(t, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), *prevEnd)
		assert.Equal(t, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), *prevStart)
	})

	t.Run("only end", func(t *testing.T) {
		prevStart, prevEnd := getPreviousPeriod(nil, &octoberEnd)

		assert.Equal(t, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), *prevEnd)
		assert.Equal(t, time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), *prevStart)
	})

	t.Run("default 30 day windows", func(t *testing.T) {
		before := time.Now()
		prevStart, prevEnd := getPreviousPeriod(nil, nil)
		after := time.Now()

		require.NotNil(t, prevStart)
		require.NotNil(t, prevEnd)
		// The previous window ends about 31 days ago and covers the 30 days
		// before that.
		assert.WithinDuration(t, before.AddDate(0, 0, -31), *prevEnd, time.Minute)
		assert.WithinDuration(t, after.AddDate(0, 0, -31), *prevEnd, time.Minute)
		assert.Equal(t, 30*24*time.Hour, prevEnd.Sub(*prevStart))
	})
}

func TestBuildProjectOverviewComputesPreviousOccupancy(t *testing.T) {
	metric := models.ProjectOverviewMetrics{
		TotalLots:            10,
		OccupiedLots:         5,
		PreviousOccupiedLots: 2,
		ComparisonRevenue:    1000,
		PreviousRevenue:      500,
	}

	overview := buildProjectOverview(metric, nil)

	assert.InDelta(t, 50.0, overview.OccupancyRate, 0.001)
	assert.InDelta(t, 20.0, overview.PreviousOccupancyRate, 0.001)
	assert.InDelta(t, 150.0, overview.OccupancyChangePercentage, 0.001)
	assert.NotEqual(t, 100.0, overview.OccupancyChangePercentage)
	assert.InDelta(t, 100.0, overview.RevenueChangePercentage, 0.001)
	assert.NotNil(t, overview.RevenueTrend)
	assert.Empty(t, overview.RevenueTrend)
}

func TestRefreshCacheUsesBoundedQueries(t *testing.T) {
	run := func(projectCount int) map[string]int {
		projects := make([]models.Project, projectCount)
		for i := range projects {
			projects[i] = models.Project{ID: uint(i + 1), Name: "Project"}
		}

		repo := newFakeAnalyticsRepo()
		service := NewAnalyticsService(repo, &fakeProjectRepo{projects: projects}, nil, nil)

		require.NoError(t, service.RefreshCache(context.Background()))
		return repo.calls
	}

	oneProject := run(1)
	manyProjects := run(10)

	// Aggregate reads run once regardless of how many projects exist.
	for _, call := range []string{
		"GetProjectOverviewMetrics",
		"GetProjectLotDistributions",
		"GetRevenueTrendByProject",
	} {
		assert.Equal(t, 1, oneProject[call], "%s called once for one project", call)
		assert.Equal(t, 1, manyProjects[call], "%s called once for many projects", call)
	}

	// The remaining reads are constant too, even if the global overview needs
	// several of them.
	for _, call := range []string{
		"GetTotalRevenue",
		"GetActiveContractsCount",
		"GetAveragePayment",
		"GetOccupancyRate",
		"GetRevenueTrend",
		"GetLotDistribution",
		"GetProjectPerformance",
	} {
		assert.Equal(t, oneProject[call], manyProjects[call], "%s cost does not grow with projects", call)
	}

	// All cache rows are written by one batched call, so the write count does
	// not grow with the project count either.
	assert.Equal(t, 1, oneProject["SetCacheEntries"])
	assert.Equal(t, 1, manyProjects["SetCacheEntries"])
	assert.Zero(t, manyProjects["SetCache"])
}

func TestRefreshCacheWritesEveryProjectEvenWithoutActivity(t *testing.T) {
	projects := []models.Project{{ID: 1, Name: "Active"}, {ID: 2, Name: "Empty"}}
	repo := newFakeAnalyticsRepo()
	repo.metrics = []models.ProjectOverviewMetrics{{ProjectID: 1, TotalLots: 4, OccupiedLots: 2}}
	service := NewAnalyticsService(repo, &fakeProjectRepo{projects: projects}, nil, nil)

	require.NoError(t, service.RefreshCache(context.Background()))

	projectEntries := make(map[uint]map[string]bool)
	globalKeys := make(map[string]bool)
	for _, entry := range repo.entries {
		if entry.ProjectID == nil {
			globalKeys[entry.Key] = true
			continue
		}
		if projectEntries[*entry.ProjectID] == nil {
			projectEntries[*entry.ProjectID] = make(map[string]bool)
		}
		projectEntries[*entry.ProjectID][entry.Key] = true
	}

	for _, project := range projects {
		assert.True(t, projectEntries[project.ID]["analytics_overview"], "overview cached for project %d", project.ID)
		assert.True(t, projectEntries[project.ID]["analytics_distribution"], "distribution cached for project %d", project.ID)
	}
	assert.True(t, globalKeys["analytics_overview"])
	assert.True(t, globalKeys["analytics_distribution"])
	assert.True(t, globalKeys["analytics_performance"])
}
