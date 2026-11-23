package services

import (
	"context"
	"testing"

	"github.com/sjperalta/fintera-api/internal/jobs"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckOverduePayments_NotifiesOncePerUser(t *testing.T) {
	payments := []models.Payment{
		{ID: 1, Contract: models.Contract{ApplicantUserID: 7}},
		{ID: 2, Contract: models.Contract{ApplicantUserID: 7}},
		{ID: 3, Contract: models.Contract{ApplicantUserID: 9}},
	}

	paymentRepo := &mockPaymentRepositoryWithOverdue{
		mockFindOverdue: func(context.Context) ([]models.Payment, error) {
			return payments, nil
		},
	}

	var created []*models.Notification
	notifRepo := &mockNotificationRepository{
		mockCreate: func(_ context.Context, notification *models.Notification) error {
			created = append(created, notification)
			return nil
		},
	}

	worker := jobs.NewWorker(0)
	defer func() { _ = worker.Shutdown(context.Background()) }()

	service := NewPaymentService(
		paymentRepo,
		nil,
		nil,
		nil,
		nil,
		NewNotificationService(notifRepo, &mockUserRepository{}),
		nil,
		nil,
		nil,
		worker,
	)

	err := service.CheckOverduePayments(context.Background())
	require.NoError(t, err)

	require.Len(t, created, 2)
	assert.Equal(t, uint(7), created[0].UserID)
	assert.Equal(t, uint(9), created[1].UserID)
	for _, notification := range created {
		require.NotNil(t, notification.NotificationType)
		assert.Equal(t, models.NotificationTypePaymentOverdue, *notification.NotificationType)
	}
}

// TestCheckOverduePayments_DedupesAcrossBatches proves the notification
// dedupe survives the batched scan: the user has more overdue payments than
// fit in one batch, but must still be notified exactly once per run.
func TestCheckOverduePayments_DedupesAcrossBatches(t *testing.T) {
	payments := make([]models.Payment, 0, 501)
	for i := 0; i < 501; i++ {
		payments = append(payments, models.Payment{
			ID:       uint(i + 1),
			Contract: models.Contract{ApplicantUserID: 7},
		})
	}

	paymentRepo := &mockPaymentRepositoryWithOverdue{
		mockFindOverdue: func(context.Context) ([]models.Payment, error) {
			return payments, nil
		},
	}

	var created []*models.Notification
	notifRepo := &mockNotificationRepository{
		mockCreate: func(_ context.Context, notification *models.Notification) error {
			created = append(created, notification)
			return nil
		},
	}

	worker := jobs.NewWorker(0)
	defer func() { _ = worker.Shutdown(context.Background()) }()

	service := NewPaymentService(
		paymentRepo, nil, nil, nil, nil,
		NewNotificationService(notifRepo, &mockUserRepository{}),
		nil, nil, nil, worker,
	)

	err := service.CheckOverduePayments(context.Background())
	require.NoError(t, err)

	require.Len(t, created, 1, "one notification per user across all batches")
	assert.Equal(t, uint(7), created[0].UserID)
}
