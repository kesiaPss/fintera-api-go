package services

import "github.com/sjperalta/fintera-api/internal/models"

// This file is the single source of Spanish labels for every report export.
// Builders and renderers must read labels from here instead of hardcoding
// strings, so XLSX, CSV and PDF stay in sync.

// PaymentTypeLabels maps payment type constants to Spanish labels.
var PaymentTypeLabels = map[string]string{
	models.PaymentTypeReservation:      "Reservación",
	models.PaymentTypeDownPayment:      "Prima",
	models.PaymentTypeInstallment:      "Cuota",
	models.PaymentTypeFull:             "Pago total",
	models.PaymentTypeAdvance:          "Abono a capital",
	models.PaymentTypeCapitalRepayment: "Abono a capital",
}

// FinancingTypeLabels maps financing type constants to Spanish labels.
var FinancingTypeLabels = map[string]string{
	models.FinancingTypeDirect: "Directo",
	models.FinancingTypeBank:   "Bancario",
	models.FinancingTypeCash:   "Contado",
}

// CommissionOriginLabels maps commission origin constants to Spanish labels.
var CommissionOriginLabels = map[string]string{
	models.CommissionOriginReferral:   "Enlace",
	models.CommissionOriginHistorical: "Histórico",
	models.CommissionOriginDirect:     "Directo",
}

// StatusLabels maps payment and contract status constants to Spanish labels.
// Payment and contract statuses that share a value (pending, submitted,
// rejected, cancelled) share the same translation, so each key appears once.
var StatusLabels = map[string]string{
	models.PaymentStatusPending:          "Pendiente",
	models.PaymentStatusSubmitted:        "Enviado",
	models.PaymentStatusPaid:             "Pagado",
	models.PaymentStatusRejected:         "Rechazado",
	models.PaymentStatusCancelled:        "Cancelado",
	models.PaymentStatusReadjustment:     "Reajuste",
	models.ContractStatusApproved:        "Aprobado",
	models.ContractStatusClosed:          "Cerrado",
	models.ContractStatusSigned:          "Firmado",
	models.CommissionCreditStatusSettled: "Liquidado",
}

// Aging bucket keys and labels. Boundaries are inclusive: a payment exactly
// 90 days overdue belongs to the 61-90 bucket, and anything above 90 days uses
// the final bucket.
const (
	AgingBucket1To30  = "1-30"
	AgingBucket31To60 = "31-60"
	AgingBucket61To90 = "61-90"
	AgingBucket90Plus = "90+"
)

// AgingBucketLabels maps aging bucket keys to Spanish labels.
var AgingBucketLabels = map[string]string{
	AgingBucket1To30:  "1-30 días",
	AgingBucket31To60: "31-60 días",
	AgingBucket61To90: "61-90 días",
	AgingBucket90Plus: "90+ días",
}

// Report titles and filename bases for the four financial exports. Filenames
// combine the base with the period, e.g. "reporte_operaciones_2026-10.xlsx".
const (
	ReportTitleOperations = "Reporte de Operaciones del Mes"
	ReportBaseOperations  = "reporte_operaciones"

	ReportTitleCommissions = "Reporte de Comisiones"
	ReportBaseCommissions  = "reporte_comisiones"

	ReportTitleOverdue = "Reporte de Pagos Vencidos"
	ReportBaseOverdue  = "reporte_pagos_vencidos"

	ReportTitleIncomeFlow = "Reporte de Flujo de Ingresos"
	ReportBaseIncomeFlow  = "reporte_flujo_ingresos"
)

// Spanish XLSX sheet names used by the report exports.
const (
	ReportSheetOperations  = "Operaciones"
	ReportSheetCommissions = "Comisiones"
	ReportSheetOverdue     = "Pagos Vencidos"
	ReportSheetIncomeFlow  = "Flujo de Ingresos"
)

// TranslatePaymentType returns the Spanish label for a payment type, falling
// back to the raw value when it is unknown.
func TranslatePaymentType(value string) string {
	return translateReportLabel(PaymentTypeLabels, value)
}

// TranslateFinancingType returns the Spanish label for a financing type,
// falling back to the raw value when it is unknown.
func TranslateFinancingType(value string) string {
	return translateReportLabel(FinancingTypeLabels, value)
}

// TranslateCommissionOrigin returns the Spanish label for a commission origin,
// falling back to the raw value when it is unknown.
func TranslateCommissionOrigin(value string) string {
	return translateReportLabel(CommissionOriginLabels, value)
}

// TranslateStatus returns the Spanish label for a payment or contract status,
// falling back to the raw value when it is unknown.
func TranslateStatus(value string) string {
	return translateReportLabel(StatusLabels, value)
}

// TranslateAgingBucket returns the Spanish label for an aging bucket, falling
// back to the raw value when it is unknown.
func TranslateAgingBucket(value string) string {
	return translateReportLabel(AgingBucketLabels, value)
}

func translateReportLabel(labels map[string]string, value string) string {
	if translated, ok := labels[value]; ok {
		return translated
	}
	return value
}
