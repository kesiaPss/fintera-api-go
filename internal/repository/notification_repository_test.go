package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestNotificationRepository_FindByUserPageFirstPage(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewNotificationRepository(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "notifications" WHERE user_id = \$1`).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))

	mock.ExpectQuery(`SELECT \* FROM "notifications" WHERE user_id = \$1 ORDER BY id DESC LIMIT \$2`).
		WithArgs(uint(7), 21).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title"}).
			AddRow(12, 7, "Newest").
			AddRow(11, 7, "Older"))

	notifications, total, err := repo.FindByUserPage(context.Background(), 7, "", 0, 21)

	assert.NoError(t, err)
	assert.Equal(t, int64(12), total)
	assert.Len(t, notifications, 2)
	assert.Equal(t, uint(12), notifications[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepository_FindByUserPageUsesCursorAndUnreadFilter(t *testing.T) {
	db, mock := setupMockDB(t)
	repo := NewNotificationRepository(db)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "notifications" WHERE user_id = \$1 AND read_at IS NULL`).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))

	mock.ExpectQuery(`SELECT \* FROM "notifications" WHERE user_id = \$1 AND read_at IS NULL AND id < \$2 ORDER BY id DESC LIMIT \$3`).
		WithArgs(uint(7), uint(30), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title"}).AddRow(29, 7, "Next"))

	notifications, total, err := repo.FindByUserPage(context.Background(), 7, "unread", 30, 20)

	assert.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Len(t, notifications, 1)
	assert.Equal(t, uint(29), notifications[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}
