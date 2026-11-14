package services

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// renderReportXLSX renders a report document as a styled XLSX workbook. The
// sheet name is provided by the caller in Spanish (for example
// ReportSheetOperations); an empty name falls back to "Reporte".
func renderReportXLSX(doc *ReportDocument, sheetName string) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("xlsx renderer requires a report document")
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := sanitizeXLSXSheetName(sheetName)
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, fmt.Errorf("failed to name xlsx sheet: %w", err)
	}
	if _, err := writeReportDocumentToSheet(file, sheet, doc); err != nil {
		return nil, err
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write xlsx report: %w", err)
	}
	return buffer.Bytes(), nil
}
