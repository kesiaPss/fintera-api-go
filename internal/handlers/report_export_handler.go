package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sjperalta/fintera-api/internal/middleware"
	"github.com/sjperalta/fintera-api/internal/services"
)

// ReportExportHandler serves the financial report exports. It parses the
// shared filters, delegates the aggregation to ReportExportService and streams
// the rendered bytes with the Spanish filename and the correct content type.
type ReportExportHandler struct {
	exportSvc *services.ReportExportService
}

// NewReportExportHandler creates the report export handler.
func NewReportExportHandler(exportSvc *services.ReportExportService) *ReportExportHandler {
	return &ReportExportHandler{exportSvc: exportSvc}
}

var (
	errReportExportFormat    = errors.New("el parámetro format es obligatorio: use xlsx, csv o pdf")
	errReportExportProjectID = errors.New("project_id inválido")
	errReportExportDate      = errors.New("fecha inválida, use AAAA-MM-DD")
)

// @Summary Exportar Reporte de Operaciones
// @Description Genera el resumen mensual de operaciones (contabilidad) en XLSX, CSV o PDF. Si se omiten start_date y end_date, el período por defecto es el mes calendario actual.
// @Tags Reports
// @Security BearerAuth
// @Produce application/octet-stream
// @Produce text/csv
// @Param format query string true "Formato de exportación (xlsx, csv, pdf)"
// @Param project_id query int false "ID del proyecto"
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Por defecto, el primer día del mes actual"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Por defecto, el último día del mes actual"
// @Success 200 {file} file "reporte_operaciones_<período>.<formato>"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/operations_export [get]
func (h *ReportExportHandler) Operations(c *gin.Context) {
	format, err := parseReportExportFormat(c)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	filters, err := parseReportExportFilters(c, true, true, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildOperationsReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, format, services.ReportSheetOperations)
}

// @Summary Exportar Reporte de Comisiones
// @Description Genera el reporte de comisiones agrupado por vendedor en XLSX, CSV o PDF, con comisiones pagadas, pendientes, créditos y neto pendiente.
// @Tags Reports
// @Security BearerAuth
// @Produce application/octet-stream
// @Produce text/csv
// @Param format query string true "Formato de exportación (xlsx, csv, pdf)"
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Success 200 {file} file "reporte_comisiones_<período>.<formato>"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/commissions_export [get]
func (h *ReportExportHandler) Commissions(c *gin.Context) {
	format, err := parseReportExportFormat(c)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	// Commissions is grouped by seller and has no project filter.
	filters, err := parseReportExportFilters(c, false, false, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildCommissionsReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, format, services.ReportSheetCommissions)
}

// @Summary Exportar Reporte de Pagos Vencidos
// @Description Genera el reporte de pagos vencidos de contratos activos en XLSX, CSV o PDF, con tramos de mora de 1-30, 31-60, 61-90 y 90+ días. No incluye contratos cancelados.
// @Tags Reports
// @Security BearerAuth
// @Produce application/octet-stream
// @Produce text/csv
// @Param format query string true "Formato de exportación (xlsx, csv, pdf)"
// @Success 200 {file} file "reporte_pagos_vencidos_<período>.<formato>"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/overdue_export [get]
func (h *ReportExportHandler) Overdue(c *gin.Context) {
	format, err := parseReportExportFormat(c)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	filters := reportExportScopeFilters(c)

	document, err := h.exportSvc.BuildOverdueReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, format, services.ReportSheetOverdue)
}

// @Summary Exportar Reporte de Flujo de Ingresos
// @Description Genera el flujo de ingresos mensual (cobranza real contra proyectada) en XLSX, CSV o PDF. Si se omiten start_date y end_date, el período por defecto es el mes calendario actual.
// @Tags Reports
// @Security BearerAuth
// @Produce application/octet-stream
// @Produce text/csv
// @Param format query string true "Formato de exportación (xlsx, csv, pdf)"
// @Param project_id query int false "ID del proyecto"
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Por defecto, el primer día del mes actual"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Por defecto, el último día del mes actual"
// @Success 200 {file} file "reporte_flujo_ingresos_<período>.<formato>"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/income_flow_export [get]
func (h *ReportExportHandler) IncomeFlow(c *gin.Context) {
	format, err := parseReportExportFormat(c)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	filters, err := parseReportExportFilters(c, true, true, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildIncomeFlowReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, format, services.ReportSheetIncomeFlow)
}

// LegacyAnalyticsExport is the deprecated /analytics/export alias. It keeps the
// old behaviour (CSV when format is omitted) and reuses the operations report
// builder, so the staging UI keeps working until it migrates.
//
// @Summary Exportar Reporte de Operaciones (alias heredado)
// @Description Alias de compatibilidad (deprecado): use /reports/operations_export. Se mantiene temporalmente para clientes antiguos y se eliminará tras la migración de la UI. Si se omite format, el formato por defecto es csv.
// @Tags Reports
// @Security BearerAuth
// @Produce application/octet-stream
// @Produce text/csv
// @Param format query string false "Formato de exportación (xlsx, csv, pdf). Por defecto, csv"
// @Param project_id query int false "ID del proyecto"
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Por defecto, el primer día del mes actual"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Por defecto, el último día del mes actual"
// @Success 200 {file} file "reporte_operaciones_<período>.<formato>"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /analytics/export [get]
func (h *ReportExportHandler) LegacyAnalyticsExport(c *gin.Context) {
	format, err := legacyExportFormat(c)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	filters, err := parseReportExportFilters(c, true, true, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildOperationsReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, format, services.ReportSheetOperations)
}

// LegacyCommissionsCSV is the deprecated /reports/commissions_csv alias. It
// always serves CSV and forwards the date range.
//
// @Summary Exportar Reporte de Comisiones (alias heredado)
// @Description Alias de compatibilidad (deprecado): use /reports/commissions_export?format=csv. Se mantiene temporalmente para clientes antiguos y se eliminará tras la migración de la UI. Siempre responde CSV.
// @Tags Reports
// @Security BearerAuth
// @Produce text/csv
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Success 200 {file} file "reporte_comisiones_<período>.csv"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/commissions_csv [get]
func (h *ReportExportHandler) LegacyCommissionsCSV(c *gin.Context) {
	// Commissions is grouped by seller and has no project filter.
	filters, err := parseReportExportFilters(c, false, false, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildCommissionsReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, services.ReportFormatCSV, services.ReportSheetCommissions)
}

// LegacyTotalRevenueCSV is the deprecated /reports/total_revenue_csv alias. It
// always serves CSV and reuses the income-flow builder.
//
// @Summary Exportar Reporte de Flujo de Ingresos (alias heredado)
// @Description Alias de compatibilidad (deprecado): use /reports/income_flow_export?format=csv. Se mantiene temporalmente para clientes antiguos y se eliminará tras la migración de la UI. Siempre responde CSV.
// @Tags Reports
// @Security BearerAuth
// @Produce text/csv
// @Param project_id query int false "ID del proyecto"
// @Param start_date query string false "Fecha inicial (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Param end_date query string false "Fecha final (AAAA-MM-DD o RFC3339). Sin valor, incluye todos los períodos"
// @Success 200 {file} file "reporte_flujo_ingresos_<período>.csv"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reports/total_revenue_csv [get]
func (h *ReportExportHandler) LegacyTotalRevenueCSV(c *gin.Context) {
	filters, err := parseReportExportFilters(c, false, true, time.Now())
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	document, err := h.exportSvc.BuildIncomeFlowReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, services.ReportFormatCSV, services.ReportSheetIncomeFlow)
}

// LegacyOverduePaymentsCSV is the deprecated /reports/overdue_payments_csv
// alias. It is a snapshot, so it ignores dates and always serves CSV.
//
// @Summary Exportar Reporte de Pagos Vencidos (alias heredado)
// @Description Alias de compatibilidad (deprecado): use /reports/overdue_export?format=csv. Se mantiene temporalmente para clientes antiguos y se eliminará tras la migración de la UI. Es una foto del estado actual (sin filtros de fecha) y siempre responde CSV.
// @Tags Reports
// @Security BearerAuth
// @Produce text/csv
// @Success 200 {file} file "reporte_pagos_vencidos_<período>.csv"
// @Failure 500 {object} map[string]string
// @Router /reports/overdue_payments_csv [get]
func (h *ReportExportHandler) LegacyOverduePaymentsCSV(c *gin.Context) {
	filters := reportExportScopeFilters(c)

	document, err := h.exportSvc.BuildOverdueReport(c.Request.Context(), filters)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	h.serveReportExport(c, document, services.ReportFormatCSV, services.ReportSheetOverdue)
}

// parseReportExportFormat reads the required format query parameter and
// normalizes it to xlsx, csv or pdf.
func parseReportExportFormat(c *gin.Context) (string, error) {
	raw := c.Query("format")
	if strings.TrimSpace(raw) == "" {
		return "", errReportExportFormat
	}
	return services.NormalizeReportFormat(raw)
}

// legacyExportFormat reads the format query parameter for the deprecated
// /analytics/export alias. Unlike the new exports, format is optional and
// defaults to CSV to preserve the old client behaviour.
func legacyExportFormat(c *gin.Context) (string, error) {
	raw := strings.TrimSpace(c.Query("format"))
	if raw == "" {
		return services.ReportFormatCSV, nil
	}
	return services.NormalizeReportFormat(raw)
}

// parseReportExportProjectID reads the optional project_id filter.
func parseReportExportProjectID(c *gin.Context) (*uint, error) {
	raw := strings.TrimSpace(c.Query("project_id"))
	if raw == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil, errReportExportProjectID
	}

	projectID := uint(parsed)
	return &projectID, nil
}

// parseReportExportDate reads an optional date accepting AAAA-MM-DD (local) and
// RFC3339 timestamps.
func parseReportExportDate(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.Local); err == nil {
		return &parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return &parsed, nil
	}

	return nil, errReportExportDate
}

// currentMonthReportRange returns the current calendar month: the first day at
// 00:00 and the last day at 23:59:59, both in the location of now.
func currentMonthReportRange(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	return start, end
}

// reportExportScopeFilters builds the authorization scope shared by every
// export. Admins see everything; sellers are limited to their own operations.
func reportExportScopeFilters(c *gin.Context) services.ReportExportFilters {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)

	filters := services.ReportExportFilters{FilterBySeller: role != "admin"}
	if filters.FilterBySeller {
		filters.SellerID = userID
	}
	return filters
}

// parseReportExportFilters builds the shared report filters. Sellers are always
// scoped to their own operations; admins see everything. When
// defaultToCurrentMonth is set and both dates are missing (the monthly
// operations and income-flow reports), the range defaults to the current
// calendar month so those exports never load the entire history. applyProject
// is false for reports that do not support a project filter.
func parseReportExportFilters(c *gin.Context, defaultToCurrentMonth, applyProject bool, now time.Time) (services.ReportExportFilters, error) {
	filters := reportExportScopeFilters(c)

	if applyProject {
		projectID, err := parseReportExportProjectID(c)
		if err != nil {
			return filters, err
		}
		filters.ProjectID = projectID
	}

	startDate, err := parseReportExportDate(c.Query("start_date"))
	if err != nil {
		return filters, err
	}
	endDate, err := parseReportExportDate(c.Query("end_date"))
	if err != nil {
		return filters, err
	}

	if defaultToCurrentMonth && startDate == nil && endDate == nil {
		start, end := currentMonthReportRange(now)
		startDate = &start
		endDate = &end
	}

	filters.StartDate = startDate
	filters.EndDate = endDate

	return filters, nil
}

// serveReportExport renders the document and streams it as an attachment.
func (h *ReportExportHandler) serveReportExport(c *gin.Context, document *services.ReportDocument, format, sheetName string) {
	normalized, err := services.NormalizeReportFormat(format)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	data, err := h.exportSvc.RenderReport(document, normalized, sheetName)
	if err != nil {
		writeReportExportServerError(c, err)
		return
	}

	contentType := "application/octet-stream"
	if normalized == services.ReportFormatCSV {
		contentType = "text/csv"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", document.Filename(normalized)))
	c.Data(http.StatusOK, contentType, data)
}

// writeReportExportBadRequest returns a Spanish validation error.
func writeReportExportBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

// writeReportExportServerError wraps builder and renderer failures in a
// Spanish message.
func writeReportExportServerError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("no se pudo generar el reporte: %v", err)})
}
