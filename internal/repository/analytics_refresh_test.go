package repository

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMergeRevenueTrendKeepsYearsSeparate(t *testing.T) {
	nov2025 := revenueTrendRow{
		Month: time.Date(2025, 11, 1, 0, 0, 0, 0, time.UTC),
		Label: "Nov",
		Total: 100,
	}
	dec2025 := revenueTrendRow{
		Month: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		Label: "Dec",
		Total: 250,
	}
	nov2026 := revenueTrendRow{
		Month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		Label: "Nov",
		Total: 200,
	}

	points := mergeRevenueTrend([]revenueTrendRow{nov2025, dec2025}, []revenueTrendRow{nov2026})

	require.Len(t, points, 3)
	assert.Equal(t, "Nov", points[0].Label)
	assert.Equal(t, 100.0, points[0].Real)
	assert.Equal(t, "Dec", points[1].Label)
	assert.Equal(t, 250.0, points[1].Real)
	assert.Equal(t, "Nov", points[2].Label)
	assert.Equal(t, 200.0, points[2].Projected)
}

func TestGetProjectPerformanceAggregatesLots(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	mock.ExpectQuery(`SELECT projects.id AS project_id, projects.name AS project_name, COUNT\(lots.id\) AS total_lots`).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "project_name", "total_lots", "sold_lots"}).
			AddRow(1, "Altos", 10, 4))
	mock.ExpectQuery(`SELECT lots.project_id AS project_id, DATE_TRUNC\('week'`).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "week", "total"}))

	performance, err := repo.GetProjectPerformance(context.Background(), models.AnalyticsFilters{})

	require.NoError(t, err)
	require.Len(t, performance, 1)
	assert.InDelta(t, 40.0, performance[0].GrowthPercentage, 0.001)
	assert.Empty(t, performance[0].WeeklyPerformance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindAllSummariesSelectsIdentityOnly(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewProjectRepository(db)

	mock.ExpectQuery(`SELECT id, name FROM "projects"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
			AddRow(1, "Altos de la Sierra"))

	projects, err := repo.FindAllSummaries(context.Background())

	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, uint(1), projects[0].ID)
	assert.Equal(t, "Altos de la Sierra", projects[0].Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetCacheUsesAtomicUpsert(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	mock.ExpectQuery(`(?s)INSERT INTO "analytics_cache".*ON CONFLICT.*DO UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err := repo.SetCache(context.Background(), "analytics_overview", nil, map[string]int{"value": 1}, time.Minute)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetCacheEntriesUsesOneUpsertPerScope(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewAnalyticsRepository(db)

	projectOne, projectTwo := uint(1), uint(2)
	mock.ExpectQuery(`(?s)INSERT INTO "analytics_cache".*ON CONFLICT \("cache_key","project_id"\) DO UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	mock.ExpectQuery(`(?s)INSERT INTO "analytics_cache".*ON CONFLICT \("cache_key"\) WHERE project_id IS NULL DO UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	err := repo.SetCacheEntries(context.Background(), []AnalyticsCacheEntry{
		{Key: "analytics_overview", ProjectID: &projectOne, Data: map[string]int{"value": 1}, TTL: time.Minute},
		{Key: "analytics_distribution", ProjectID: &projectOne, Data: map[string]int{"value": 2}, TTL: time.Minute},
		{Key: "analytics_overview", ProjectID: &projectTwo, Data: map[string]int{"value": 3}, TTL: time.Minute},
		{Key: "analytics_overview", Data: map[string]int{"value": 4}, TTL: time.Minute},
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestSetCacheConcurrentUpsert runs against a real database when DATABASE_URL
// is available, because sqlmock cannot model uniqueness enforcement.
func TestSetCacheConcurrentUpsert(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set; skipping live cache upsert test")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	key := "test_concurrent_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() {
		db.Exec("DELETE FROM analytics_cache WHERE cache_key = ?", key)
	})

	repo := NewAnalyticsRepository(db)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			errs <- repo.SetCache(context.Background(), key, nil, map[string]int{"index": index}, time.Minute)
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	var count int64
	require.NoError(t, db.Table("analytics_cache").
		Where("cache_key = ? AND project_id IS NULL", key).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
