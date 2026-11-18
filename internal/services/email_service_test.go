package services

import (
	"context"
	"testing"

	"github.com/resend/resend-go/v2"
	"github.com/sjperalta/fintera-api/internal/config"
	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestEmailService_checkEmailPreconditions(t *testing.T) {
	logger.Setup("test")

	// Test case 1: Email notifications disabled
	cfg := &config.Config{
		EnableEmailNotifications: false,
	}
	service := NewEmailService(cfg)
	user := &models.User{Email: "test@example.com", FullName: "Test User", ID: 1}

	ok, err := service.checkEmailPreconditions(user, "test operation")
	assert.False(t, ok, "Should return false when notifications are disabled")
	assert.Nil(t, err, "Should not return error when notifications are disabled")

	// Test case 2: Email configured and valid
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "test_key",
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)

	ok, err = service.checkEmailPreconditions(user, "test operation")
	assert.True(t, ok, "Should return true when properly configured")
	assert.Nil(t, err, "Should not return error when properly configured")

	// Test case 3: Email not configured (missing key)
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "", // Missing key
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)

	ok, err = service.checkEmailPreconditions(user, "test operation")
	assert.False(t, ok, "Should return false when config is missing")
	assert.Error(t, err, "Should return error when config is missing")
	assert.Contains(t, err.Error(), "RESEND_API_KEY is not set")

	// Test case 4: Invalid email
	cfg = &config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "test_key",
		FromEmail:                "from@example.com",
	}
	service = NewEmailService(cfg)
	userInvalid := &models.User{Email: "", FullName: "Invalid User", ID: 2}

	ok, err = service.checkEmailPreconditions(userInvalid, "test operation")
	assert.False(t, ok, "Should return false when email is invalid")
	assert.Error(t, err, "Should return error when email is invalid")
	assert.Equal(t, "email address is empty", err.Error())
}

func TestEmailService_checkNotificationPreconditions(t *testing.T) {
	logger.Setup("test")

	t.Run("user opted out skips without error", func(t *testing.T) {
		service := NewEmailService(&config.Config{
			EnableEmailNotifications: true,
			ResendAPIKey:             "test_key",
			FromEmail:                "from@example.com",
		})
		user := &models.User{ID: 1, Email: "test@example.com", FullName: "Opted Out", EmailNotifications: false}

		ok, err := service.checkNotificationPreconditions(user, "test operation")
		assert.False(t, ok, "Should return false when the user opted out")
		assert.NoError(t, err, "Opting out is not an error")
	})

	t.Run("globally disabled still returns false without error", func(t *testing.T) {
		service := NewEmailService(&config.Config{EnableEmailNotifications: false})
		user := &models.User{ID: 2, Email: "test@example.com", EmailNotifications: true}

		ok, err := service.checkNotificationPreconditions(user, "test operation")
		assert.False(t, ok)
		assert.NoError(t, err)
	})

	t.Run("opted in delegates to checkEmailPreconditions", func(t *testing.T) {
		service := NewEmailService(&config.Config{
			EnableEmailNotifications: true,
			ResendAPIKey:             "test_key",
			FromEmail:                "from@example.com",
		})
		user := &models.User{ID: 3, Email: "test@example.com", FullName: "Opted In", EmailNotifications: true}

		ok, err := service.checkNotificationPreconditions(user, "test operation")
		expectedOk, expectedErr := service.checkEmailPreconditions(user, "test operation")
		assert.Equal(t, expectedOk, ok)
		assert.Equal(t, expectedErr, err)
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("opted in surfaces configuration errors", func(t *testing.T) {
		service := NewEmailService(&config.Config{
			EnableEmailNotifications: true,
			ResendAPIKey:             "",
			FromEmail:                "from@example.com",
		})
		user := &models.User{ID: 4, Email: "test@example.com", EmailNotifications: true}

		ok, err := service.checkNotificationPreconditions(user, "test operation")
		assert.False(t, ok)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "RESEND_API_KEY is not set")
	})
}

func TestEmailService_SendContractSubmittedSkipsUnresolvedRecipient(t *testing.T) {
	logger.Setup("test")

	service := NewEmailService(&config.Config{
		EnableEmailNotifications: true,
		ResendAPIKey:             "test_key",
		FromEmail:                "from@example.com",
	})
	contract := &models.Contract{ID: 11, ApplicantUserID: 4}
	contract.ApplicantUser.EmailNotifications = true

	err := service.SendContractSubmitted(context.Background(), contract)
	assert.NoError(t, err, "a missing applicant must be skipped, not reported as a send failure")
}

func TestApplyRecipientOverride(t *testing.T) {
	params := &resend.SendEmailRequest{
		To:      []string{"customer@example.com"},
		Subject: "Bienvenido",
	}
	original := applyRecipientOverride(params, "delivered+local@resend.dev")
	assert.Equal(t, []string{"customer@example.com"}, original)
	assert.Equal(t, []string{"delivered+local@resend.dev"}, params.To)
	assert.Equal(t, "[TEST → customer@example.com] Bienvenido", params.Subject)

	prod := &resend.SendEmailRequest{
		To:      []string{"customer@example.com"},
		Subject: "Bienvenido",
	}
	original = applyRecipientOverride(prod, "   ")
	assert.Nil(t, original)
	assert.Equal(t, []string{"customer@example.com"}, prod.To)
	assert.Equal(t, "Bienvenido", prod.Subject)
}
