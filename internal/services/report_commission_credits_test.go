package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportService_CommissionCredits(t *testing.T) {
	repo := newFakeCommissionRepository()
	now := time.Now()
	settledAt := now.Add(time.Hour)

	credits := []*models.SellerCommissionCredit{
		{
			SellerID:   7,
			ContractID: 11,
			Amount:     1500,
			Status:     models.CommissionCreditStatusPending,
			Seller:     &models.User{ID: 7, FullName: "Ana Vendedora"},
			Contract:   &models.Contract{ID: 11, GUID: "contract-11"},
		},
		{
			SellerID:   7,
			ContractID: 12,
			Amount:     500,
			Status:     models.CommissionCreditStatusSettled,
			SettledAt:  &settledAt,
			Seller:     &models.User{ID: 7, FullName: "Ana Vendedora"},
			Contract:   &models.Contract{ID: 12, GUID: "contract-12"},
		},
		{
			SellerID:   8,
			ContractID: 13,
			Amount:     900,
			Status:     models.CommissionCreditStatusPending,
			Seller:     &models.User{ID: 8, FullName: "Carlos Vendedor"},
			Contract:   &models.Contract{ID: 13, GUID: "contract-13"},
		},
	}
	for _, credit := range credits {
		require.NoError(t, repo.CreateCredit(context.Background(), credit))
	}

	service := NewReportService(nil, nil, nil, nil)
	service.SetCommissionRepository(repo)

	all, pending, err := service.CommissionCredits(context.Background(), 0)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, 2400.0, pending)

	own, pendingOwn, err := service.CommissionCredits(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, own, 2)
	assert.Equal(t, 1500.0, pendingOwn)
	for _, credit := range own {
		assert.Equal(t, uint(7), credit.SellerID)
		assert.Equal(t, "Ana Vendedora", credit.Seller)
	}
}

func TestReportService_CommissionCreditsWithoutRepository(t *testing.T) {
	service := NewReportService(nil, nil, nil, nil)

	credits, pending, err := service.CommissionCredits(context.Background(), 0)

	require.NoError(t, err)
	assert.Empty(t, credits)
	assert.Zero(t, pending)
}

func TestPendingCommissionTotal(t *testing.T) {
	paidAt := time.Now()
	items := []CommissionReportItem{
		{Commission: 1000},
		{Commission: 500, CommissionPaidAt: &paidAt},
		{Commission: 250},
	}

	assert.Equal(t, 1250.0, PendingCommissionTotal(items))
}
