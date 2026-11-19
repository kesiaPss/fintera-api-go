package services

import (
	"context"
	"strings"
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/sjperalta/fintera-api/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakeHistoricalUserRepo struct {
	repository.UserRepository
	users   []models.User
	nextID  uint
	creates int
	updates int
}

func newFakeHistoricalUserRepo() *fakeHistoricalUserRepo {
	return &fakeHistoricalUserRepo{nextID: 1}
}

func (r *fakeHistoricalUserRepo) FindByID(_ context.Context, id uint) (*models.User, error) {
	for i := range r.users {
		if r.users[i].ID == id {
			copy := r.users[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeHistoricalUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	for i := range r.users {
		if strings.EqualFold(r.users[i].Email, email) {
			copy := r.users[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeHistoricalUserRepo) FindByIdentity(_ context.Context, identity string) (*models.User, error) {
	for i := range r.users {
		if strings.EqualFold(r.users[i].Identity, identity) {
			copy := r.users[i]
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeHistoricalUserRepo) CreateHistoricalClient(_ context.Context, user *models.User) error {
	for i := range r.users {
		if strings.EqualFold(r.users[i].Email, user.Email) {
			return gorm.ErrDuplicatedKey
		}
		if user.Identity != "" && strings.EqualFold(r.users[i].Identity, user.Identity) {
			return gorm.ErrDuplicatedKey
		}
	}
	user.ID = r.nextID
	r.nextID++
	r.users = append(r.users, *user)
	r.creates++
	return nil
}

func (r *fakeHistoricalUserRepo) Update(_ context.Context, user *models.User) error {
	for i := range r.users {
		if r.users[i].ID == user.ID {
			r.users[i] = *user
			r.updates++
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func historicalSaleWithClient(name, identity, email string) *models.HistoricalSale {
	sale := &models.HistoricalSale{
		ClientName:       name,
		SalePrice:        100000,
		ValidationStatus: models.HistoricalSaleStatusPending,
	}
	if identity != "" {
		sale.ClientIdentity = stringPtrForTest(identity)
	}
	if email != "" {
		sale.ClientEmail = stringPtrForTest(email)
	}
	return sale
}

func TestHistoricalClientEnsureLinkedCreatesPlaceholderUser(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	service := NewHistoricalClientService(users, nil)
	sale := historicalSaleWithClient("María López", "0801-1990-12345", "")

	err := service.EnsureLinked(context.Background(), sale, 4)
	require.NoError(t, err)
	require.NotNil(t, sale.ClientUserID)
	require.Len(t, users.users, 1)

	user := users.users[0]
	assert.Equal(t, models.RoleUser, user.Role)
	assert.Equal(t, models.StatusActive, user.Status)
	assert.True(t, user.HistoricalClient)
	assert.True(t, user.MustChangePassword)
	assert.False(t, user.EmailNotifications)
	assert.Equal(t, "María López", user.FullName)
	assert.Equal(t, "0801-1990-12345", user.Identity)
	assert.Contains(t, user.Email, "historico+0801199012345@fintera.local")
	require.NotNil(t, user.CreatedBy)
	assert.Equal(t, uint(4), *user.CreatedBy)
}

func TestHistoricalClientEnsureLinkedReusesExistingIdentity(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	users.users = append(users.users, models.User{
		ID:       9,
		Email:    "cliente@example.com",
		Role:     models.RoleUser,
		Status:   models.StatusActive,
		FullName: "Cliente Existente",
		Identity: "0801-1990-00001",
	})
	service := NewHistoricalClientService(users, nil)
	sale := historicalSaleWithClient("Cliente Existente", "0801-1990-00001", "cliente@example.com")

	err := service.EnsureLinked(context.Background(), sale, 4)
	require.NoError(t, err)
	require.NotNil(t, sale.ClientUserID)
	assert.Equal(t, uint(9), *sale.ClientUserID)
	assert.Zero(t, users.creates)
}

func TestHistoricalClientEnsureLinkedUsesProvidedEmail(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	service := NewHistoricalClientService(users, nil)
	sale := historicalSaleWithClient("Ana Torres", "0801-1990-00002", "ana@example.com")

	require.NoError(t, service.EnsureLinked(context.Background(), sale, 4))
	require.Len(t, users.users, 1)
	assert.Equal(t, "ana@example.com", users.users[0].Email)
}

func TestHistoricalClientEnsureLinkedFallsBackWhenEmailIsTaken(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	users.users = append(users.users, models.User{
		ID:       3,
		Email:    "shared@example.com",
		Role:     models.RoleUser,
		Status:   models.StatusActive,
		Identity: "otra-identidad",
	})
	service := NewHistoricalClientService(users, nil)
	sale := historicalSaleWithClient("Nuevo Cliente", "0801-1990-00003", "shared@example.com")

	require.NoError(t, service.EnsureLinked(context.Background(), sale, 4))
	require.Len(t, users.users, 2)
	assert.NotEqual(t, "shared@example.com", users.users[1].Email)
	assert.Contains(t, users.users[1].Email, "historico+")
}

func TestHistoricalClientEnsureLinkedRejectsStaffIdentity(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	users.users = append(users.users, models.User{
		ID:       1,
		Email:    "admin@example.com",
		Role:     models.RoleAdmin,
		Status:   models.StatusActive,
		Identity: "0801-1990-99999",
	})
	service := NewHistoricalClientService(users, nil)
	sale := historicalSaleWithClient("Admin", "0801-1990-99999", "admin@example.com")

	err := service.EnsureLinked(context.Background(), sale, 4)
	assert.ErrorIs(t, err, ErrHistoricalClientStaffIdentity)
	assert.Nil(t, sale.ClientUserID)
}

func TestHistoricalClientBackfillLinksPendingSales(t *testing.T) {
	users := newFakeHistoricalUserRepo()
	salesRepo := newFakeHistoricalSaleRepository()
	service := NewHistoricalClientService(users, salesRepo)
	ctx := context.Background()

	first := &models.HistoricalSale{
		ClientName:       "Cliente Uno",
		ClientIdentity:   stringPtrForTest("0801-0001"),
		SalePrice:        1000,
		SaleDate:         mustDate(t, "2023-01-10"),
		ValidationStatus: models.HistoricalSaleStatusPending,
		CreatedBy:        4,
	}
	second := &models.HistoricalSale{
		ClientName:       "Cliente Dos",
		ClientIdentity:   stringPtrForTest("0801-0002"),
		SalePrice:        2000,
		SaleDate:         mustDate(t, "2023-02-10"),
		ValidationStatus: models.HistoricalSaleStatusPending,
		CreatedBy:        4,
	}
	require.NoError(t, salesRepo.Create(ctx, first))
	require.NoError(t, salesRepo.Create(ctx, second))

	result, err := service.Backfill(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Linked)
	assert.Zero(t, result.Errors)

	for _, sale := range salesRepo.sales {
		assert.NotNil(t, sale.ClientUserID)
	}

	again, err := service.Backfill(ctx, 10)
	require.NoError(t, err)
	assert.Zero(t, again.Linked)
}
