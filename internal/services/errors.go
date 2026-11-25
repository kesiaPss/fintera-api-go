package services

import (
	"context"
	"errors"

	"github.com/sjperalta/fintera-api/pkg/logger"
)

// Common service errors
var (
	ErrNotFound            = errors.New("registro no encontrado")
	ErrInvalidPassword     = errors.New("contraseña inválida")
	ErrUnauthorized        = errors.New("no autorizado")
	ErrInvalidState        = errors.New("transición de estado inválida")
	ErrContractNotApproved = errors.New("no se puede aprobar el pago de reserva sin antes aprobar el contrato")
	ErrDuplicate           = errors.New("registro duplicado")
	ErrInvalidRecoveryCode = errors.New("código de recuperación inválido o expirado")

	ErrReferralNotFound         = errors.New("enlace de referido no encontrado")
	ErrAssignmentNotFound       = errors.New("asignación no encontrada")
	ErrAssignmentNotOwned       = errors.New("la asignación pertenece a otro vendedor")
	ErrAssignmentNotPending     = errors.New("la asignación ya fue respondida o expiró")
	ErrAssignmentExpired        = errors.New("la asignación expiró")
	ErrAssignmentSellerInactive = errors.New("el vendedor no está activo")
	ErrNoAssignableSeller       = errors.New("no hay vendedores disponibles para asignar")
	ErrScoreOverrideInvalid     = errors.New("puntaje o motivo inválido")

	ErrInvalidCommissionRate      = errors.New("la tasa de comisión debe estar entre 0 y 100")
	ErrInvalidCommissionRules     = errors.New("reglas de comisión inválidas")
	ErrCommissionExceptionMissing = errors.New("excepción de comisión no encontrada")
	ErrCommissionCreditMissing    = errors.New("crédito de comisión no encontrado")
	ErrCommissionCreditSettled    = errors.New("el crédito de comisión ya fue aplicado")
	ErrHistoricalSaleNotFound     = errors.New("venta histórica no encontrada")
	ErrHistoricalSaleForbidden    = errors.New("no tienes permiso para modificar esta venta histórica")
	ErrHistoricalSaleInvalid      = errors.New("datos de la venta histórica inválidos")
	ErrHistoricalSaleDuplicate    = errors.New("ya existe una venta histórica con esa identidad y fecha")
)

// lookupError classifies a failed repository lookup. A missing row is
// ErrNotFound, but an aborted caller is not: context.Canceled must keep
// traveling so the handler ends the request as 499 instead of inventing a 404
// for a resource that exists (the portal retries and gets a 200).
func lookupError(err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	return ErrNotFound
}

// logIgnoredError records a non-critical error without interrupting the main flow.
func logIgnoredError(action string, err error) {
	if err != nil {
		logger.Warn("ignored error", "action", action, "error", err)
	}
}
