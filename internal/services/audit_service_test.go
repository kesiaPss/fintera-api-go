package services

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditService_LogSystemActorStoresNullUser(t *testing.T) {
	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).
		WithArgs(nil, "CREATE", "Contract", uint(7), "details", "", "", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	svc := NewAuditService(gdb)
	err := svc.Log(context.Background(), SystemAuditActor, "CREATE", "Contract", 7, "details", "", "")

	require.NoError(t, err)
	require.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestAuditService_LogAuthenticatedActorStoresUser(t *testing.T) {
	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).
		WithArgs(sqlmock.AnyArg(), "UPDATE", "Lot", uint(3), "details", "", "", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	svc := NewAuditService(gdb)
	err := svc.Log(context.Background(), 5, "UPDATE", "Lot", 3, "details", "", "")

	require.NoError(t, err)
	require.NoError(t, mockSQL.ExpectationsWereMet())
}

func TestAuditService_LogReturnsContextualError(t *testing.T) {
	gdb, mockSQL := setupMockDB(t)
	mockSQL.ExpectQuery(`INSERT INTO "audit_logs"`).WillReturnError(errors.New("fk violation"))

	svc := NewAuditService(gdb)
	err := svc.Log(context.Background(), 9, "CREATE", "Contract", 7, "details", "", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "audit log CREATE Contract #7 (actor 9)")
	assert.ErrorContains(t, err, "fk violation")
}
