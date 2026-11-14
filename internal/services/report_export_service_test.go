package services

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNormalizeReportFormat(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "xlsx", raw: "xlsx", want: ReportFormatXLSX},
		{name: "csv uppercase", raw: " CSV ", want: ReportFormatCSV},
		{name: "pdf mixed case", raw: "Pdf", want: ReportFormatPDF},
		{name: "empty", raw: "", wantErr: true},
		{name: "blank", raw: "   ", wantErr: true},
		{name: "unsupported", raw: "docx", wantErr: true},
		{name: "xls alias rejected", raw: "xls", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeReportFormat(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeReportFormat(%q) expected error, got %q", test.raw, got)
				}
				if err.Error() != "formato inválido: use xlsx, csv o pdf" {
					t.Fatalf("unexpected error message: %q", err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("NormalizeReportFormat(%q) unexpected error: %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("NormalizeReportFormat(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

// newRenderTestDocument builds a minimal but complete report document used to
// exercise the format dispatcher.
func newRenderTestDocument() *ReportDocument {
	generatedAt := time.Date(2026, 10, 15, 10, 30, 0, 0, time.Local)
	amount := 1500000.0

	return &ReportDocument{
		Title:       "Reporte de Prueba",
		Subtitle:    "Documento de prueba",
		Period:      "Octubre 2026",
		PeriodKey:   "2026-10",
		BaseName:    "reporte_prueba",
		GeneratedAt: generatedAt,
		Currency:    "HNL",
		Sections: []ReportSection{
			{
				Title:   "Resumen",
				Kind:    ReportSectionTable,
				Headers: []string{"Concepto", "Monto"},
				Rows: []ReportRow{
					{Cells: []ReportCell{TextCell("Ventas"), MoneyCell(amount, "HNL")}},
				},
				Totals: &ReportRow{
					Cells: []ReportCell{TextCell("Total"), MoneyCell(amount, "HNL")},
					Kind:  ReportRowTotal,
				},
			},
		},
	}
}

func TestRenderReportDispatch(t *testing.T) {
	service := &ReportExportService{}
	document := newRenderTestDocument()

	t.Run("xlsx", func(t *testing.T) {
		data, err := service.RenderReport(document, ReportFormatXLSX, ReportSheetOperations)
		if err != nil {
			t.Fatalf("RenderReport(xlsx) error: %v", err)
		}
		// XLSX files are zip containers.
		if !bytes.HasPrefix(data, []byte("PK")) {
			t.Fatalf("xlsx output does not look like a zip container")
		}
	})

	t.Run("csv", func(t *testing.T) {
		data, err := service.RenderReport(document, ReportFormatCSV, ReportSheetOperations)
		if err != nil {
			t.Fatalf("RenderReport(csv) error: %v", err)
		}
		content := string(data)
		for _, want := range []string{"Reporte de Prueba", "Octubre 2026", "Concepto,Monto", "1500000.00"} {
			if !strings.Contains(content, want) {
				t.Fatalf("csv output missing %q:\n%s", want, content)
			}
		}
	})

	t.Run("pdf", func(t *testing.T) {
		data, err := service.RenderReport(document, ReportFormatPDF, ReportSheetOperations)
		if err != nil {
			t.Fatalf("RenderReport(pdf) error: %v", err)
		}
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Fatalf("pdf output does not start with the PDF signature")
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := service.RenderReport(document, "docx", ReportSheetOperations); err == nil {
			t.Fatal("RenderReport(docx) expected an error")
		}
	})
}
