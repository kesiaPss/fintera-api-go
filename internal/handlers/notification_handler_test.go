package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/sjperalta/fintera-api/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationRepoForHandler struct {
	repository.NotificationRepository
	rows     []models.Notification
	total    int64
	unread   int64
	status   string
	cursorID uint
	limit    int
}

func (s *notificationRepoForHandler) FindByUserPage(_ context.Context, _ uint, status string, cursorID uint, limit int) ([]models.Notification, int64, error) {
	s.status = status
	s.cursorID = cursorID
	s.limit = limit
	return s.rows, s.total, nil
}

func (s *notificationRepoForHandler) CountUnread(context.Context, uint) (int64, error) {
	return s.unread, nil
}

func newNotificationTestContext(t *testing.T, target string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set("userID", userID)
	return c, recorder
}

func TestNotificationHandler_IndexReturnsCursorPage(t *testing.T) {
	repo := &notificationRepoForHandler{
		rows: []models.Notification{
			{ID: 30, Title: "Newest"},
			{ID: 29, Title: "Older"},
			{ID: 28, Title: "Oldest returned"},
			{ID: 27, Title: "Lookahead row"},
		},
		total:  12,
		unread: 4,
	}
	handler := NewNotificationHandler(services.NewNotificationService(repo, nil))
	c, recorder := newNotificationTestContext(t, "/api/v1/notifications?limit=3", 9)

	handler.Index(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 4, repo.limit, "requested limit plus one lookahead row")

	var body struct {
		Notifications []models.NotificationResponse `json:"notifications"`
		NextCursor    *uint                         `json:"next_cursor"`
		HasMore       bool                          `json:"has_more"`
		UnreadCount   int64                         `json:"unread_count"`
		Pagination    struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))

	assert.Len(t, body.Notifications, 3)
	assert.Equal(t, uint(30), body.Notifications[0].ID)
	assert.True(t, body.HasMore)
	require.NotNil(t, body.NextCursor)
	assert.Equal(t, uint(28), *body.NextCursor)
	assert.Equal(t, int64(4), body.UnreadCount)
	assert.Equal(t, int64(12), body.Pagination.Total)
}

func TestNotificationHandler_IndexForwardsCursorAndLimit(t *testing.T) {
	repo := &notificationRepoForHandler{rows: []models.Notification{{ID: 10}}}
	handler := NewNotificationHandler(services.NewNotificationService(repo, nil))
	c, recorder := newNotificationTestContext(t, "/api/v1/notifications?cursor=25&limit=5&status=unread", 9)

	handler.Index(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, uint(25), repo.cursorID)
	assert.Equal(t, 6, repo.limit, "requested limit plus one lookahead row")
	assert.Equal(t, "unread", repo.status)
}

func TestNotificationHandler_IndexClampsLimit(t *testing.T) {
	cases := []struct {
		query string
		want  int
	}{
		{query: "?limit=500", want: maxNotificationLimit + 1},
		{query: "?limit=0", want: defaultNotificationLimit + 1},
		{query: "?limit=abc", want: defaultNotificationLimit + 1},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			repo := &notificationRepoForHandler{}
			handler := NewNotificationHandler(services.NewNotificationService(repo, nil))
			c, recorder := newNotificationTestContext(t, "/api/v1/notifications"+tc.query, 9)

			handler.Index(c)

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, tc.want, repo.limit)
		})
	}
}

func TestNotificationHandler_IndexRejectsInvalidCursor(t *testing.T) {
	repo := &notificationRepoForHandler{}
	handler := NewNotificationHandler(services.NewNotificationService(repo, nil))
	c, recorder := newNotificationTestContext(t, "/api/v1/notifications?cursor=abc", 9)

	handler.Index(c)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, repo.limit, "does not hit the repository")
}
