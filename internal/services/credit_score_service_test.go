package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type creditScoreUserRepo struct {
	repository.UserRepository
	updateCreditScoreCalls int
	updateCalls            int
	updatedUserID          uint
	updatedScore           int
}

func (m *creditScoreUserRepo) UpdateCreditScore(_ context.Context, userID uint, score int) error {
	m.updateCreditScoreCalls++
	m.updatedUserID = userID
	m.updatedScore = score
	return nil
}

func (m *creditScoreUserRepo) Update(_ context.Context, _ *models.User) error {
	m.updateCalls++
	return nil
}

type creditScoreContractRepo struct {
	repository.ContractRepository
	contracts []models.Contract
}

func (m *creditScoreContractRepo) FindByUser(_ context.Context, _ uint) ([]models.Contract, error) {
	return m.contracts, nil
}

type creditScorePaymentRepo struct {
	repository.PaymentRepository
	payments map[uint][]models.Payment
}

func (m *creditScorePaymentRepo) FindByContract(_ context.Context, contractID uint) ([]models.Payment, error) {
	return m.payments[contractID], nil
}

func TestCreditScoreService_UpdateScoreUsesMinimalUpdate(t *testing.T) {
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	paidOnTime := due.AddDate(0, 0, -1)

	userRepo := &creditScoreUserRepo{}
	contractRepo := &creditScoreContractRepo{contracts: []models.Contract{{ID: 1}}}
	paymentRepo := &creditScorePaymentRepo{payments: map[uint][]models.Payment{
		1: {{Status: models.PaymentStatusPaid, DueDate: due, PaymentDate: &paidOnTime}},
	}}

	service := NewCreditScoreService(userRepo, contractRepo, paymentRepo)

	require.NoError(t, service.UpdateScore(context.Background(), 42))

	assert.Equal(t, 1, userRepo.updateCreditScoreCalls)
	assert.Equal(t, 0, userRepo.updateCalls)
	assert.Equal(t, uint(42), userRepo.updatedUserID)
	assert.Equal(t, 505, userRepo.updatedScore)
}
