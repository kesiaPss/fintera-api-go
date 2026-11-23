package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sjperalta/fintera-api/internal/repository"
)

// Supported export formats for the financial reports.
const (
	ReportFormatXLSX = "xlsx"
	ReportFormatCSV  = "csv"
	ReportFormatPDF  = "pdf"
)

// ReportExportFilters carries the normalized filters shared by the financial
// report exports. Admin callers leave FilterBySeller false and SellerID zero;
// seller callers set FilterBySeller true and SellerID to their own user id so
// every report is scoped to their contracts.
type ReportExportFilters struct {
	ProjectID      *uint
	StartDate      *time.Time
	EndDate        *time.Time
	SellerID       uint
	FilterBySeller bool
}

// ReportExportService builds the neutral report documents rendered by the
// XLSX, CSV and PDF exporters. It aggregates analytics and reporting data but
// never renders files itself, keeping every format in sync.
type ReportExportService struct {
	analyticsRepo  repository.AnalyticsRepository
	overdueRepo    repository.OverdueReportRepository
	paymentRepo    repository.PaymentRepository
	reportSvc      *ReportService
	commissionRepo repository.CommissionRepository
}

// NewReportExportService wires the report export builders.
func NewReportExportService(
	analyticsRepo repository.AnalyticsRepository,
	overdueRepo repository.OverdueReportRepository,
	paymentRepo repository.PaymentRepository,
	reportSvc *ReportService,
	commissionRepo repository.CommissionRepository,
) *ReportExportService {
	return &ReportExportService{
		analyticsRepo:  analyticsRepo,
		overdueRepo:    overdueRepo,
		paymentRepo:    paymentRepo,
		reportSvc:      reportSvc,
		commissionRepo: commissionRepo,
	}
}

// NormalizeReportFormat validates an export format coming from a query string.
// Surrounding spaces and case are ignored; only xlsx, csv and pdf are accepted.
func NormalizeReportFormat(raw string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(raw))
	switch format {
	case ReportFormatXLSX, ReportFormatCSV, ReportFormatPDF:
		return format, nil
	default:
		return "", fmt.Errorf("formato inválido: use xlsx, csv o pdf")
	}
}

// RenderReport renders a report document in the requested format into memory.
// It is a thin wrapper over RenderReportTo for callers (and tests) that need
// the bytes; HTTP handlers should stream with RenderReportTo directly.
func (s *ReportExportService) RenderReport(doc *ReportDocument, format, sheetName string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := s.RenderReportTo(buffer, doc, format, sheetName); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// RenderReportTo renders a report document in the requested format directly
// into w, avoiding a full in-memory copy of the file. XLSX uses the Spanish
// sheet name provided by the caller; CSV and PDF ignore it because they do
// not have sheets.
func (s *ReportExportService) RenderReportTo(w io.Writer, doc *ReportDocument, format, sheetName string) error {
	normalized, err := NormalizeReportFormat(format)
	if err != nil {
		return err
	}

	switch normalized {
	case ReportFormatXLSX:
		data, err := renderReportXLSX(doc, sheetName)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	case ReportFormatCSV:
		return renderReportCSVTo(w, doc)
	default:
		return renderReportPDFTo(w, doc)
	}
}

// spanishMonthNames maps months to their capitalized Spanish names.
var spanishMonthNames = [...]string{
	"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
	"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
}

// spanishMonthName returns the Spanish name of a month.
func spanishMonthName(month time.Month) string {
	if month < time.January || month > time.December {
		return ""
	}
	return spanishMonthNames[month-1]
}

// reportPeriodLabel derives a human readable period label and a
// filename-safe period key from the optional date range. A single month is
// rendered as "Octubre 2026" / "2026-10", a wider range as
// "01/10/2026 - 31/12/2026" / "2026-10-01_2026-12-31", and an unset range
// defaults to the current month.
func reportPeriodLabel(startDate, endDate *time.Time, now time.Time) (label, key string) {
	switch {
	case startDate == nil && endDate == nil:
		return fmt.Sprintf("%s %d", spanishMonthName(now.Month()), now.Year()), now.Format("2006-01")
	case startDate != nil && endDate != nil:
		start := startDate.In(time.Local)
		end := endDate.In(time.Local)
		if start.Year() == end.Year() && start.Month() == end.Month() {
			return fmt.Sprintf("%s %d", spanishMonthName(start.Month()), start.Year()), start.Format("2006-01")
		}
		return fmt.Sprintf("%s - %s", start.Format("02/01/2006"), end.Format("02/01/2006")),
			fmt.Sprintf("%s_%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	case startDate != nil:
		start := startDate.In(time.Local)
		return fmt.Sprintf("Desde %s", start.Format("02/01/2006")), "desde_" + start.Format("2006-01-02")
	default:
		end := endDate.In(time.Local)
		return fmt.Sprintf("Hasta %s", end.Format("02/01/2006")), "hasta_" + end.Format("2006-01-02")
	}
}
