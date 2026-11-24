package services

import (
	"errors"
	"fmt"
	"strings"

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

// ErrIdentityDocumentsRequired is returned when a contract is created for an
// applicant that is missing a DNI or an RTN. The missing kinds are named so
// the caller can ask for exactly what is missing.
type ErrIdentityDocumentsRequired struct {
	Missing []string
}

func (e ErrIdentityDocumentsRequired) Error() string {
	return "faltan documentos de identificación: " + strings.Join(e.Missing, ", ")
}

// ErrIdentityDocumentType is returned when an upload has a type the API does
// not accept. It names the offending file.
type ErrIdentityDocumentType struct {
	Filename    string
	ContentType string
}

func (e ErrIdentityDocumentType) Error() string {
	if e.ContentType == "" {
		return fmt.Sprintf("el archivo %q no tiene un tipo permitido", e.Filename)
	}
	return fmt.Sprintf("el archivo %q no tiene un tipo permitido (%s)", e.Filename, e.ContentType)
}

// ErrIdentityDocumentSize is returned when an upload is larger than the
// limit. It names the offending file.
type ErrIdentityDocumentSize struct {
	Filename string
	Size     int64
	Max      int64
}

func (e ErrIdentityDocumentSize) Error() string {
	return fmt.Sprintf("el archivo %q supera el tamaño máximo de %d MB", e.Filename, e.Max/(1024*1024))
}

// logIgnoredError records a non-critical error without interrupting the main flow.
func logIgnoredError(action string, err error) {
	if err != nil {
		logger.Warn("ignored error", "action", action, "error", err)
	}
}
