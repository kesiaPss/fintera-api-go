package services

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func sampleReportDocument() *ReportDocument {
	generatedAt := time.Date(2026, 10, 31, 18, 5, 0, 0, time.UTC)
	dueDate := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	return &ReportDocument{
		Title:       ReportTitleOperations,
		Subtitle:    "Resumen mensual de operaciones",
		Period:      "Octubre 2026",
		PeriodKey:   "2026-10",
		BaseName:    ReportBaseOperations,
		GeneratedAt: generatedAt,
		Currency:    "HNL",
		Sections: []ReportSection{
			{
				Title: "Resumen General",
				Kind:  ReportSectionSummary,
				Rows: []ReportRow{
					{Cells: []ReportCell{TextCell("Contratos aprobados"), IntCell(12)}},
					{Cells: []ReportCell{TextCell("Monto aprobado"), MoneyCell(1250000, "HNL")}},
					{Cells: []ReportCell{TextCell("Efectividad de cobro"), PercentCell(0.785)}},
					{Cells: []ReportCell{TextCell("Última actualización"), DateCell(dueDate)}},
					{Cells: []ReportCell{TextCell("Reserva en dólares"), MoneyCell(2500, "USD")}},
				},
				Totals: &ReportRow{
					Cells: []ReportCell{TextCell("Total"), MoneyCell(1250000, "HNL")},
					Kind:  ReportRowTotal,
				},
			},
			{
				Title: "Indicadores",
				Kind:  ReportSectionStats,
				Rows: []ReportRow{
					{
						Cells: []ReportCell{TextCell("Contratos cerrados"), IntCell(8), TextCell("del periodo")},
						Kind:  ReportRowPaid,
					},
					{
						Cells: []ReportCell{TextCell("Pagos vencidos"), IntCell(3), TextCell("requieren seguimiento")},
						Kind:  ReportRowDanger,
					},
				},
			},
			{
				Title:   "Detalle de contratos",
				Kind:    ReportSectionTable,
				Headers: []string{"Contrato", "Cliente", "Monto", "Vence", "Días mora", "Avance", "Estado"},
				Rows: []ReportRow{
					{
						Cells: []ReportCell{
							TextCell("#1001"), TextCell("Ana Pérez"), MoneyCell(500000, "HNL"),
							DateCell(dueDate), IntCell(12), PercentCell(0.5), TextCell("Pagado"),
						},
						Kind: ReportRowPaid,
					},
					{
						Cells: []ReportCell{
							TextCell("#1002"), TextCell("Luis Hernández"), MoneyCell(8900, "USD"),
							DateCell(dueDate), IntCell(45), PercentCell(0.25), TextCell("Pendiente"),
						},
						Kind: ReportRowPending,
					},
					{
						Cells: []ReportCell{
							TextCell("#1003"), TextCell("Marta López"), MoneyCell(320000, "HNL"),
							DateCell(dueDate), IntCell(95), PercentCell(0), TextCell("En mora"),
						},
						Kind: ReportRowDanger,
					},
					{
						Cells: []ReportCell{
							TextCell("#1004"), TextCell("Sin datos"), MoneyCell(0, "HNL"),
							DateCell(dueDate), IntCell(0), PercentCell(1), TextCell("—"),
						},
						Kind: ReportRowMuted,
					},
				},
				Totals: &ReportRow{
					Cells: []ReportCell{
						TextCell("Total"), TextCell(""), MoneyCell(828900, "HNL"),
						TextCell(""), IntCell(0), PercentCell(0.72), TextCell(""),
					},
					Kind: ReportRowTotal,
				},
				Note: "Los montos están expresados en la moneda indicada por cada fila.",
			},
			{
				Title: "Notas",
				Kind:  ReportSectionNote,
				Note:  "Reporte generado automáticamente por Fintera.",
			},
		},
	}
}

func TestReportDocumentFilename(t *testing.T) {
	doc := &ReportDocument{
		BaseName:    ReportBaseOperations,
		PeriodKey:   "2026-10",
		GeneratedAt: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
	}

	assert.Equal(t, "reporte_operaciones_2026-10.xlsx", doc.Filename(".xlsx"))
	assert.Equal(t, "reporte_operaciones_2026-10.pdf", doc.Filename("PDF"))
	assert.Equal(t, "reporte_operaciones_2026-10", doc.Filename(""))

	withoutKey := &ReportDocument{
		BaseName:    ReportBaseCommissions,
		GeneratedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, "reporte_comisiones_2026-11.csv", withoutKey.Filename("csv"))

	fallback := &ReportDocument{PeriodKey: "2026-10"}
	assert.Equal(t, "reporte_2026-10.xlsx", fallback.Filename("xlsx"))

	accented := &ReportDocument{BaseName: "Reporte Operaciones Año", PeriodKey: "2026-10"}
	assert.Equal(t, "reporte_operaciones_ano_2026-10.xlsx", accented.Filename("xlsx"))

	var nilDocument *ReportDocument
	assert.Equal(t, "", nilDocument.Filename("xlsx"))
}

func TestReportCellFormatting(t *testing.T) {
	date := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, "L 1,250,000.00", MoneyCell(1250000, "HNL").DisplayText("HNL"))
	assert.Equal(t, "$ 8,900.50", MoneyCell(8900.5, "USD").DisplayText("HNL"))
	assert.Equal(t, "1250000.00", MoneyCell(1250000, "HNL").DisplayTextPlain())
	assert.Equal(t, "78.50%", PercentCell(0.785).DisplayText("HNL"))
	assert.Equal(t, "15/10/2026", DateCell(date).DisplayText("HNL"))
	assert.Equal(t, "2026-10-15", DateCell(date).DisplayTextPlain())
	assert.Equal(t, "42", IntCell(42).DisplayText("HNL"))
	assert.Equal(t, "", ReportCell{}.DisplayText("HNL"))
}

func TestRenderReportXLSX(t *testing.T) {
	doc := sampleReportDocument()

	data, err := renderReportXLSX(doc, ReportSheetOperations)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	file, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	assert.Equal(t, []string{ReportSheetOperations}, file.GetSheetList())

	rows, err := file.GetRows(ReportSheetOperations)
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	headerRow, contractColumn := findReportSheetCell(t, rows, "Contrato")
	assert.Equal(t, 0, contractColumn, "the contract column should be the first one")

	_, montoColumn := findReportSheetCell(t, rows, "Monto")
	findReportSheetCell(t, rows, "Total")
	findReportSheetCell(t, rows, "Reporte de Operaciones del Mes")

	// The first money cell below the "Monto" header must be numeric and carry
	// the lempira symbol in its number format.
	firstMoneyCell, err := excelize.CoordinatesToCellName(montoColumn+1, headerRow+2)
	require.NoError(t, err)
	value, err := file.GetCellValue(ReportSheetOperations, firstMoneyCell, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	assert.Equal(t, "500000", value)

	styleID, err := file.GetCellStyle(ReportSheetOperations, firstMoneyCell)
	require.NoError(t, err)
	style, err := file.GetStyle(styleID)
	require.NoError(t, err)
	require.NotNil(t, style.CustomNumFmt)
	assert.Contains(t, *style.CustomNumFmt, "L")

	// The USD row must use the dollar number format.
	var usdFormat string
	for rowIndex := headerRow + 2; rowIndex < len(rows); rowIndex++ {
		cell, coordErr := excelize.CoordinatesToCellName(montoColumn+1, rowIndex+1)
		require.NoError(t, coordErr)
		cellValue, valueErr := file.GetCellValue(ReportSheetOperations, cell, excelize.Options{RawCellValue: true})
		require.NoError(t, valueErr)
		if cellValue != "8900" {
			continue
		}
		styleID, styleErr := file.GetCellStyle(ReportSheetOperations, cell)
		require.NoError(t, styleErr)
		moneyStyle, styleErr := file.GetStyle(styleID)
		require.NoError(t, styleErr)
		if moneyStyle.CustomNumFmt != nil {
			usdFormat = *moneyStyle.CustomNumFmt
		}
		break
	}
	require.NotEmpty(t, usdFormat, "USD money cell must be present")
	assert.Contains(t, usdFormat, "$")
}

// TestRenderReportToMatchesBufferedBytes guards the streaming refactor. CSV
// bytes must match the buffered wrapper exactly; the PDF wrapper IS the
// writer target (so identity holds by construction) and gofpdf is not
// deterministic across calls (font object ordering varies), so the PDF check
// is structural instead of byte-level.
func TestRenderReportToMatchesBufferedBytes(t *testing.T) {
	doc := sampleReportDocument()
	svc := NewReportExportService(nil, nil, nil, nil, nil)

	data, err := svc.RenderReport(doc, ReportFormatCSV, "")
	require.NoError(t, err)
	var csvBuffer bytes.Buffer
	require.NoError(t, svc.RenderReportTo(&csvBuffer, doc, ReportFormatCSV, ""))
	assert.Equal(t, data, csvBuffer.Bytes(), "CSV streaming bytes must match")

	var pdfBuffer bytes.Buffer
	require.NoError(t, svc.RenderReportTo(&pdfBuffer, doc, ReportFormatPDF, ""))
	assert.True(t, bytes.HasPrefix(pdfBuffer.Bytes(), []byte("%PDF")), "streamed output should be a PDF")
	assert.Greater(t, pdfBuffer.Len(), 1024, "streamed PDF should not be tiny")
}

// TestRenderReportXLSXStreamStructure locks the sheet-level properties the
// StreamWriter path must preserve: merged banner ranges, explicit measured
// column widths, a frozen header pane and raw numeric money values.
func TestRenderReportXLSXStreamStructure(t *testing.T) {
	doc := sampleReportDocument()

	data, err := renderReportXLSX(doc, ReportSheetOperations)
	require.NoError(t, err)

	file, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	sheet := ReportSheetOperations

	merges, err := file.GetMergeCells(sheet)
	require.NoError(t, err)
	require.NotEmpty(t, merges, "title banner, subtitle, meta, section titles and notes are merged rows")

	titleMerged := false
	for _, merge := range merges {
		topLeft := merge.GetStartAxis()
		if topLeft == "A1" {
			assert.Equal(t, "G1", merge.GetEndAxis(), "the title banner spans the widest section")
			titleMerged = true
		}
	}
	require.True(t, titleMerged, "the title row is merged across the table")

	width, err := file.GetColWidth(sheet, "A")
	require.NoError(t, err)
	assert.Greater(t, width, 0.0, "column A carries an explicit measured width")

	panes, err := file.GetPanes(sheet)
	require.NoError(t, err)
	require.NotNil(t, panes)
	assert.True(t, panes.Freeze, "the header pane stays frozen")
	assert.Greater(t, panes.YSplit, 0, "the freeze happens below the header rows")

	// Raw numeric money cells are asserted in TestRenderReportXLSX; here the
	// formatted display text confirms the values survived streaming.
	rows, err := file.GetRows(sheet)
	require.NoError(t, err)
	formattedMoney := false
	for _, row := range rows {
		for _, cell := range row {
			if strings.Contains(cell, "500,000.00") {
				formattedMoney = true
			}
		}
	}
	assert.True(t, formattedMoney, "money values survive the stream writer")
}

func TestRenderReportCSV(t *testing.T) {
	doc := sampleReportDocument()

	data, err := renderReportCSV(doc)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	content := string(data)
	assert.Contains(t, content, "Reporte de Operaciones del Mes")
	assert.Contains(t, content, "Concepto,Valor")
	assert.Contains(t, content, "Contrato,Cliente,Monto,Vence,Días mora,Avance,Estado")
	assert.Contains(t, content, "Total")
	assert.Contains(t, content, "1250000.00")
	assert.Contains(t, content, "78.50%")
	assert.Contains(t, content, "2026-10-15")
	assert.NotContains(t, content, "L 1,250,000.00", "CSV money must not include the currency symbol")
	assert.NotContains(t, content, "$ 8,900.00", "CSV money must not include the currency symbol")
}

func TestRenderReportPDF(t *testing.T) {
	doc := sampleReportDocument()

	data, err := renderReportPDF(doc)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	assert.True(t, bytes.HasPrefix(data, []byte("%PDF")), "output should be a PDF")
	assert.Greater(t, len(data), 1024, "PDF should not be tiny")
}

func TestRenderReportXLSXSanitizesSheetName(t *testing.T) {
	doc := &ReportDocument{Title: "Reporte", BaseName: ReportBaseOverdue, PeriodKey: "2026-10"}

	data, err := renderReportXLSX(doc, "Pagos/Vencidos:Octubre")
	require.NoError(t, err)

	file, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	assert.Equal(t, []string{"PagosVencidosOctubre"}, file.GetSheetList())
}

func findReportSheetCell(t *testing.T, rows [][]string, label string) (rowIndex, column int) {
	t.Helper()
	for rowIndex, row := range rows {
		for column, cell := range row {
			if strings.TrimSpace(cell) == label {
				return rowIndex, column
			}
		}
	}
	t.Fatalf("cell %q not found in sheet", label)
	return -1, -1
}
