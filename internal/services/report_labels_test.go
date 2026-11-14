package services

import (
	"testing"

	"github.com/sjperalta/fintera-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportLabelsPaymentTypes(t *testing.T) {
	expected := map[string]string{
		models.PaymentTypeReservation:      "Reservación",
		models.PaymentTypeDownPayment:      "Prima",
		models.PaymentTypeInstallment:      "Cuota",
		models.PaymentTypeFull:             "Pago total",
		models.PaymentTypeAdvance:          "Abono a capital",
		models.PaymentTypeCapitalRepayment: "Abono a capital",
	}
	for key, want := range expected {
		got, ok := PaymentTypeLabels[key]
		require.True(t, ok, "payment type %q is missing a label", key)
		assert.Equal(t, want, got)
		assert.NotEqual(t, key, got, "payment type %q must not keep the English label", key)
		assert.Equal(t, want, TranslatePaymentType(key))
	}
}

func TestReportLabelsFinancingTypes(t *testing.T) {
	expected := map[string]string{
		models.FinancingTypeDirect: "Directo",
		models.FinancingTypeBank:   "Bancario",
		models.FinancingTypeCash:   "Contado",
	}
	for key, want := range expected {
		got, ok := FinancingTypeLabels[key]
		require.True(t, ok, "financing type %q is missing a label", key)
		assert.Equal(t, want, got)
		assert.NotEqual(t, key, got)
		assert.Equal(t, want, TranslateFinancingType(key))
	}
}

func TestReportLabelsCommissionOrigins(t *testing.T) {
	expected := map[string]string{
		models.CommissionOriginReferral:   "Enlace",
		models.CommissionOriginHistorical: "Histórico",
		models.CommissionOriginDirect:     "Directo",
	}
	for key, want := range expected {
		got, ok := CommissionOriginLabels[key]
		require.True(t, ok, "commission origin %q is missing a label", key)
		assert.Equal(t, want, got)
		assert.NotEqual(t, key, got)
		assert.Equal(t, want, TranslateCommissionOrigin(key))
	}
}

func TestReportLabelsStatuses(t *testing.T) {
	expected := map[string]string{
		"pending":      "Pendiente",
		"submitted":    "Enviado",
		"paid":         "Pagado",
		"approved":     "Aprobado",
		"closed":       "Cerrado",
		"cancelled":    "Cancelado",
		"rejected":     "Rechazado",
		"signed":       "Firmado",
		"readjustment": "Reajuste",
	}
	for key, want := range expected {
		got, ok := StatusLabels[key]
		require.True(t, ok, "status %q is missing a label", key)
		assert.Equal(t, want, got)
		assert.NotEqual(t, key, got, "status %q must not keep the English label", key)
		assert.Equal(t, want, TranslateStatus(key))
	}
}

func TestReportLabelsAgingBuckets(t *testing.T) {
	expected := map[string]string{
		AgingBucket1To30:  "1-30 días",
		AgingBucket31To60: "31-60 días",
		AgingBucket61To90: "61-90 días",
		AgingBucket90Plus: "90+ días",
	}
	for key, want := range expected {
		got, ok := AgingBucketLabels[key]
		require.True(t, ok, "aging bucket %q is missing a label", key)
		assert.Equal(t, want, got)
		assert.Equal(t, want, TranslateAgingBucket(key))
	}
}

func TestReportLabelsFilenamesAndTitles(t *testing.T) {
	expected := map[string]string{
		ReportTitleOperations:  ReportBaseOperations,
		ReportTitleCommissions: ReportBaseCommissions,
		ReportTitleOverdue:     ReportBaseOverdue,
		ReportTitleIncomeFlow:  ReportBaseIncomeFlow,
	}
	for title, base := range expected {
		assert.NotEmpty(t, title)
		assert.NotEmpty(t, base)
		assert.Equal(t, base, sanitizeFilenameSegment(base), "filename base %q must be filename safe", base)
	}

	assert.Equal(t, "Reporte de Operaciones del Mes", ReportTitleOperations)
	assert.Equal(t, "Reporte de Comisiones", ReportTitleCommissions)
	assert.Equal(t, "Reporte de Pagos Vencidos", ReportTitleOverdue)
	assert.Equal(t, "Reporte de Flujo de Ingresos", ReportTitleIncomeFlow)

	assert.Equal(t, "Operaciones", ReportSheetOperations)
	assert.Equal(t, "Comisiones", ReportSheetCommissions)
	assert.Equal(t, "Pagos Vencidos", ReportSheetOverdue)
	assert.Equal(t, "Flujo de Ingresos", ReportSheetIncomeFlow)
}

func TestTranslateReportLabelFallsBackToRawValue(t *testing.T) {
	assert.Equal(t, "modo_desconocido", TranslatePaymentType("modo_desconocido"))
	assert.Equal(t, "otro", TranslateFinancingType("otro"))
	assert.Equal(t, "externo", TranslateCommissionOrigin("externo"))
	assert.Equal(t, "archivado", TranslateStatus("archivado"))
	assert.Equal(t, "120+", TranslateAgingBucket("120+"))
}
