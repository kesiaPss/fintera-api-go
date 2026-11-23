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
	"github.com/sjperalta/fintera-api/pkg/logger"
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
	errReportExportSellerID  = errors.New("seller_id inválido")
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

	filters, err := parseReportExportFilters(c, true, true, false, time.Now())
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
// @Param seller_id query int false "Admin only: limita el export a este vendedor id (comisiones, créditos y neto). Ignorado para vendedores, que siempre ven lo propio. Valor inválido o 0 responde 400"
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

	// Commissions is grouped by seller and has no project filter, but admins
	// may narrow it to one seller.
	filters, err := parseReportExportFilters(c, false, false, true, time.Now())
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

	filters, err := parseReportExportFilters(c, true, true, false, time.Now())
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

// parseReportExportFormat reads the required format query parameter and
// normalizes it to xlsx, csv or pdf.
func parseReportExportFormat(c *gin.Context) (string, error) {
	raw := c.Query("format")
	if strings.TrimSpace(raw) == "" {
		return "", errReportExportFormat
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

// parseReportExportSellerID reads the optional seller_id filter. Only positive
// integers are accepted; an invalid value or 0 is a validation error.
func parseReportExportSellerID(c *gin.Context) (*uint, error) {
	raw := strings.TrimSpace(c.Query("seller_id"))
	if raw == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return nil, errReportExportSellerID
	}

	sellerID := uint(parsed)
	return &sellerID, nil
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
// scoped to their own operations; admins see everything unless applySellerID is
// set and they pass a seller_id, in which case the export is scoped to that
// seller. When defaultToCurrentMonth is set and both dates are missing (the
// monthly operations and income-flow reports), the range defaults to the
// current calendar month so those exports never load the entire history.
// applyProject is false for reports that do not support a project filter.
func parseReportExportFilters(c *gin.Context, defaultToCurrentMonth, applyProject, applySellerID bool, now time.Time) (services.ReportExportFilters, error) {
	filters := reportExportScopeFilters(c)

	if applyProject {
		projectID, err := parseReportExportProjectID(c)
		if err != nil {
			return filters, err
		}
		filters.ProjectID = projectID
	}

	if applySellerID && !filters.FilterBySeller {
		sellerID, err := parseReportExportSellerID(c)
		if err != nil {
			return filters, err
		}
		if sellerID != nil {
			filters.SellerID = *sellerID
			filters.FilterBySeller = true
		}
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

// serveReportExport streams the rendered document as an attachment. The
// document has already been built by the caller, so any error that can be
// reported as JSON happens before this function writes file bytes; from the
// first rendered byte onward the response is the file and a render failure
// can only be logged (the client sees a truncated download, never a JSON
// body masquerading as one).
func (h *ReportExportHandler) serveReportExport(c *gin.Context, document *services.ReportDocument, format, sheetName string) {
	normalized, err := services.NormalizeReportFormat(format)
	if err != nil {
		writeReportExportBadRequest(c, err)
		return
	}

	contentType := "application/octet-stream"
	if normalized == services.ReportFormatCSV {
		contentType = "text/csv"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", document.Filename(normalized)))
	c.Header("Content-Type", contentType)
	c.Status(http.StatusOK)
	if err := h.exportSvc.RenderReportTo(c.Writer, document, normalized, sheetName); err != nil {
		logger.Error("report export failed mid-stream",
			"report", document.BaseName, "format", normalized, "error", err)
	}
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
