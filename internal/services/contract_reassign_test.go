package services

import (
	"context"
	"testing"
	"time"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func reassignContractFixture(currentSellerID uint) *models.Contract {
	amount := 100000.0
	contract := &models.Contract{
		ID:               10,
		GUID:             "contract-1",
		Status:           models.ContractStatusPending,
		FinancingType:    models.FinancingTypeDirect,
		Amount:           &amount,
		AssignmentOrigin: models.AssignmentOriginAutomatic,
		AssignmentStatus: models.AssignmentStatusAccepted,
	}
	if currentSellerID > 0 {
		contract.SellerID = &currentSellerID
	}
	return contract
}

func TestReassignAutomaticAvoidsCurrentSeller(t *testing.T) {
	first := testSeller(7, models.ScoreLevelHigh)
	second := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()
	assignmentRepo.assignable = []models.User{*first, *second}

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(first.ID)
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, first, second),
	}

	updated, assignment, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:           ReassignModeAutomatic,
		ExcludeCurrent: true,
		ActorID:        99,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.NotNil(t, assignment)
	require.NotNil(t, updated.SellerID)
	require.Equal(t, second.ID, *updated.SellerID)
	require.Equal(t, models.AssignmentOriginAutomatic, updated.AssignmentOrigin)
	require.Equal(t, models.AssignmentStatusPending, updated.AssignmentStatus)
	contractRepo.AssertExpectations(t)
}

func TestReassignRecoversPaidCommissionFromPreviousSeller(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:     ReassignModeSeller,
		SellerID: &next.ID,
		Accept:   true,
		ActorID:  99,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.SellerID)
	require.Equal(t, next.ID, *updated.SellerID)
	require.Nil(t, updated.CommissionPaidAt)
	require.Nil(t, updated.CommissionPaidBy)
	require.Len(t, commissionRepo.credits, 1)

	var credit *models.SellerCommissionCredit
	for _, item := range commissionRepo.credits {
		credit = item
	}
	require.NotNil(t, credit)
	require.Equal(t, previous.ID, credit.SellerID)
	require.Equal(t, 1500.0, credit.Amount)
	require.Equal(t, models.CommissionCreditStatusPending, credit.Status)
	contractRepo.AssertExpectations(t)
}

func TestReassignCancelledContractDoesNotDuplicateCredit(t *testing.T) {
	previous := testSeller(7, models.ScoreLevelHigh)
	next := testSeller(8, models.ScoreLevelHigh)
	assignmentRepo := newFakeAssignmentRepo()

	contractRepo := &mockContractRepo{}
	contract := reassignContractFixture(previous.ID)
	contract.Status = models.ContractStatusCancelled
	paidAt := time.Now()
	contract.CommissionAmount = 1500
	contract.CommissionPaidAt = &paidAt
	contractRepo.On("FindByGUIDWithDetails", mock.Anything, "contract-1").Return(contract, nil)
	contractRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	commissionRepo := newFakeCommissionRepository()
	service := &ContractService{
		repo:          contractRepo,
		assignmentSvc: newTestAssignmentService(assignmentRepo, previous, next),
		commissionSvc: NewCommissionService(commissionRepo, nil, nil),
	}

	updated, _, err := service.Reassign(context.Background(), "contract-1", ContractReassignInput{
		Mode:     ReassignModeSeller,
		SellerID: &next.ID,
		Accept:   true,
		ActorID:  99,
	})
	require.NoError(t, err)
	require.Empty(t, commissionRepo.credits, "cancellation already created the recoverable credit")
	require.NotNil(t, updated.CommissionPaidAt)
	require.Equal(t, next.ID, *updated.SellerID)
}
