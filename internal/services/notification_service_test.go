package services

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationRepoStub struct {
	repository.NotificationRepository
	rows       []models.Notification
	total      int64
	unread     int64
	status     string
	cursorID   uint
	limit      int
	findErr    error
	unreadErr  error
	findCalls  int
	unreadCall int
}

func (s *notificationRepoStub) FindByUserPage(_ context.Context, _ uint, status string, cursorID uint, limit int) ([]models.Notification, int64, error) {
	s.findCalls++
	s.status = status
	s.cursorID = cursorID
	s.limit = limit
	return s.rows, s.total, s.findErr
}

func (s *notificationRepoStub) CountUnread(context.Context, uint) (int64, error) {
	s.unreadCall++
	return s.unread, s.unreadErr
}

func TestNotificationService_FindByUserPageReportsMoreResults(t *testing.T) {
	repo := &notificationRepoStub{
		rows: []models.Notification{
			{ID: 30}, {ID: 29}, {ID: 28},
		},
		total:  10,
		unread: 5,
	}
	service := NewNotificationService(repo, nil)

	page, err := service.FindByUserPage(context.Background(), 7, "unread", 40, 2)

	require.NoError(t, err)
	assert.Equal(t, 3, repo.limit, "requests one extra row")
	assert.Equal(t, uint(40), repo.cursorID)
	assert.Equal(t, "unread", repo.status)
	assert.True(t, page.HasMore)
	assert.Len(t, page.Notifications, 2)
	require.NotNil(t, page.NextCursor)
	assert.Equal(t, uint(29), *page.NextCursor)
	assert.Equal(t, int64(10), page.Total)
	assert.Equal(t, int64(5), page.UnreadCount)
}

func TestNotificationService_FindByUserPageAtEndOfList(t *testing.T) {
	repo := &notificationRepoStub{
		rows:  []models.Notification{{ID: 3}, {ID: 2}},
		total: 2,
	}
	service := NewNotificationService(repo, nil)

	page, err := service.FindByUserPage(context.Background(), 7, "", 0, 2)

	require.NoError(t, err)
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextCursor)
	assert.Len(t, page.Notifications, 2)
}

func TestNotificationService_FindByUserPageReturnsRepositoryError(t *testing.T) {
	repo := &notificationRepoStub{findErr: assert.AnError}
	service := NewNotificationService(repo, nil)

	page, err := service.FindByUserPage(context.Background(), 7, "", 0, 20)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, page)
	assert.Zero(t, repo.unreadCall, "skips the unread count when the page query fails")
}
