package services

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jung-kurt/gofpdf"
)

// renderReportPDF renders a report document as a PDF using the shared
// reportPDFRenderer design system (title banner, section titles, tables,
// badges and footer page numbers), matching the contract and balance PDFs.
func renderReportPDF(doc *ReportDocument) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("pdf renderer requires a report document")
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to initialise report pdf: %w", err)
	}

	pdf.SetMargins(reportMarginX, reportMarginTop, reportMarginX)
	pdf.SetAutoPageBreak(true, reportMarginBottom)
	pdf.SetTitle(doc.Title, false)
	pdf.SetAuthor("FINTERA", false)
	pdf.SetCreator("FINTERA", false)
	pdf.AliasNbPages("{nb}")

	renderer := newReportPDFRenderer(pdf, tr)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() > 1 {
			renderer.continuationHeader(doc.Title)
		}
	})
	pdf.SetFooterFunc(func() {
		renderer.drawFooter("FINTERA · " + doc.Title)
	})

	pdf.AddPage()
	renderer.reportDocument(doc)

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("failed to render report pdf: %w", err)
	}

	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, fmt.Errorf("failed to write report pdf: %w", err)
	}
	return buffer.Bytes(), nil
}

// reportDocument draws the whole neutral document with the shared primitives.
func (r *reportPDFRenderer) reportDocument(doc *ReportDocument) {
	if doc.Title != "" {
		r.titleBanner(doc.Title)
	}
	r.reportDocumentMeta(doc)

	for index, section := range doc.Sections {
		if index > 0 {
			r.pdf.Ln(2)
		}
		r.reportSection(section, doc.Currency)
	}
}

func (r *reportPDFRenderer) reportDocumentMeta(doc *ReportDocument) {
	if doc.Subtitle != "" {
		r.pdf.SetFont("Helvetica", "B", 10)
		r.setTextColor(pdfText)
		r.pdf.CellFormat(r.contentWidth, 5, r.tr(doc.Subtitle), "", 1, "L", false, 0, "")
	}

	metaParts := make([]string, 0, 3)
	if doc.Period != "" {
		metaParts = append(metaParts, "Periodo: "+doc.Period)
	}
	if !doc.GeneratedAt.IsZero() {
		metaParts = append(metaParts, "Generado el "+doc.GeneratedAt.Format("02/01/2006 15:04"))
	}
	if doc.Currency != "" {
		metaParts = append(metaParts, "Moneda: "+doc.Currency)
	}
	if len(metaParts) > 0 {
		r.pdf.SetFont("Helvetica", "", 8.5)
		r.setTextColor(pdfMuted)
		r.pdf.CellFormat(r.contentWidth, 4.6, r.tr(strings.Join(metaParts, " · ")), "", 1, "L", false, 0, "")
	}
	r.pdf.Ln(2)
}

func (r *reportPDFRenderer) reportSection(section ReportSection, fallbackCurrency string) {
	if section.Title != "" {
		r.sectionTitle(section.Title)
	}

	switch section.Kind {
	case ReportSectionNote:
		r.reportNote(section.Note)
		return
	case ReportSectionStats:
		r.reportStatSection(section, fallbackCurrency)
		return
	}

	headers := section.Headers
	if len(headers) == 0 && section.Kind == ReportSectionSummary {
		headers = []string{"Concepto", "Valor"}
	}

	columns := reportSectionColumnCount(section)
	if columns < len(headers) {
		columns = len(headers)
	}
	if columns == 0 {
		return
	}

	widths := reportSectionColumnWidths(section, headers, columns, r.contentWidth, fallbackCurrency)
	aligns := reportSectionColumnAligns(section, columns)

	paddedHeaders := make([]string, columns)
	copy(paddedHeaders, headers)
	if len(headers) > 0 {
		r.tableHeader(widths, paddedHeaders, aligns)
	}

	for index, row := range section.Rows {
		r.ensureSpace(pdfTableRowHeight + 2)
		fill, color, bold := reportPDFRowStyle(row.Kind, index%2 == 1)
		r.reportTableRow(widths, aligns, reportRowText(row.Cells, fallbackCurrency), fill, color, bold)
	}

	if section.Totals != nil {
		r.ensureSpace(pdfTableRowHeight + 2)
		kind := section.Totals.Kind
		if kind == "" {
			kind = ReportRowTotal
		}
		fill, color, bold := reportPDFRowStyle(kind, false)
		r.reportTableRow(widths, aligns, reportRowText(section.Totals.Cells, fallbackCurrency), fill, color, bold)
	}

	if section.Note != "" {
		r.reportNote(section.Note)
	}
}

// reportStatSection renders stats rows as a highlight grid. Each row carries a
// label cell, a value cell and an optional hint cell.
func (r *reportPDFRenderer) reportStatSection(section ReportSection, fallbackCurrency string) {
	stats := make([]pdfStat, 0, len(section.Rows))
	for _, row := range section.Rows {
		if len(row.Cells) == 0 {
			continue
		}
		stat := pdfStat{
			Label:     row.Cells[0].DisplayText(fallbackCurrency),
			Highlight: row.Kind == ReportRowTotal || row.Kind == ReportRowSubtotal || row.Kind == ReportRowPaid,
			Danger:    row.Kind == ReportRowDanger,
		}
		if len(row.Cells) > 1 {
			stat.Value = row.Cells[1].DisplayText(fallbackCurrency)
		}
		if len(row.Cells) > 2 {
			stat.Hint = row.Cells[2].DisplayText(fallbackCurrency)
		}
		stats = append(stats, stat)
	}
	r.drawStatGrid(stats, 3)

	if section.Note != "" {
		r.reportNote(section.Note)
	}
}

func (r *reportPDFRenderer) reportNote(note string) {
	if strings.TrimSpace(note) == "" {
		return
	}
	r.ensureSpace(12)
	r.pdf.SetFont("Helvetica", "I", 8.7)
	r.setTextColor(pdfMuted)
	r.pdf.MultiCell(r.contentWidth, pdfLineHeight, r.tr(note), "", "L", false)
	r.pdf.Ln(2.4)
}

// reportTableRow draws one styled table row without touching the shared
// tableRow behaviour used by the contract and balance PDFs.
func (r *reportPDFRenderer) reportTableRow(
	widths []float64,
	aligns, cells []string,
	fill *pdfRGB,
	color pdfRGB,
	bold bool,
) {
	tableWidth := 0.0
	for _, width := range widths {
		tableWidth += width
	}

	y := r.pdf.GetY()
	if fill != nil {
		r.setFillColor(*fill)
		r.pdf.Rect(reportMarginX, y, tableWidth, pdfTableRowHeight, "F")
	}

	r.setDrawColor(pdfBorder)
	r.pdf.SetLineWidth(0.2)
	r.pdf.Line(reportMarginX, y+pdfTableRowHeight, reportMarginX+tableWidth, y+pdfTableRowHeight)

	fontStyle := ""
	if bold {
		fontStyle = "B"
	}
	r.pdf.SetFont("Helvetica", fontStyle, 8)

	x := reportMarginX
	for index, cell := range cells {
		if index >= len(widths) {
			break
		}
		align := "L"
		if index < len(aligns) {
			align = aligns[index]
		}
		r.setTextColor(color)
		r.pdf.SetXY(x, y)
		r.pdf.CellFormat(widths[index], pdfTableRowHeight, r.tr(r.fit(cell, widths[index]-2)), "", 0, align, false, 0, "")
		x += widths[index]
	}
	r.pdf.SetY(y + pdfTableRowHeight)
}

// reportPDFRowStyle maps a row kind to its fill, text color and weight.
func reportPDFRowStyle(kind ReportRowKind, zebra bool) (*pdfRGB, pdfRGB, bool) {
	switch kind {
	case ReportRowSubtotal:
		fill := pdfSoftSlate
		return &fill, pdfText, true
	case ReportRowTotal:
		fill := pdfSoftGreen
		return &fill, pdfGreen, true
	case ReportRowPaid:
		fill := pdfSoftGreen
		return &fill, pdfGreen, false
	case ReportRowPending:
		fill := pdfSoftAmber
		return &fill, pdfWarning, false
	case ReportRowWarning:
		return nil, pdfWarning, false
	case ReportRowDanger:
		fill := pdfSoftRed
		return &fill, pdfDanger, false
	case ReportRowMuted:
		return nil, pdfMuted, false
	default:
		if zebra {
			fill := pdfZebra
			return &fill, pdfText, false
		}
		return nil, pdfText, false
	}
}

// reportRowText converts cells into PDF display strings.
func reportRowText(cells []ReportCell, fallbackCurrency string) []string {
	texts := make([]string, 0, len(cells))
	for _, cell := range cells {
		texts = append(texts, cell.DisplayText(fallbackCurrency))
	}
	return texts
}

func reportSectionColumnCount(section ReportSection) int {
	columns := len(section.Headers)
	for _, row := range section.Rows {
		if len(row.Cells) > columns {
			columns = len(row.Cells)
		}
	}
	if section.Totals != nil && len(section.Totals.Cells) > columns {
		columns = len(section.Totals.Cells)
	}
	return columns
}

func reportSectionColumnAligns(section ReportSection, columns int) []string {
	aligns := make([]string, columns)
	for index := range aligns {
		aligns[index] = "L"
	}

	observe := func(row ReportRow) {
		for index, cell := range row.Cells {
			if index >= columns {
				break
			}
			switch cell.Align {
			case AlignRight:
				aligns[index] = "R"
			case AlignCenter:
				if aligns[index] != "R" {
					aligns[index] = "C"
				}
			case AlignLeft:
				// Text cells keep the default left alignment.
			}
		}
	}
	for _, row := range section.Rows {
		observe(row)
	}
	if section.Totals != nil {
		observe(*section.Totals)
	}
	return aligns
}

// reportSectionColumnWidths distributes the content width proportionally to
// the length of each column's widest value, keeping a sensible minimum.
func reportSectionColumnWidths(
	section ReportSection,
	headers []string,
	columns int,
	contentWidth float64,
	fallbackCurrency string,
) []float64 {
	weights := make([]float64, columns)
	for index := range weights {
		weights[index] = 5
	}

	observe := func(text string, column int) {
		if column < 0 || column >= columns {
			return
		}
		length := float64(utf8.RuneCountInString(text))
		if length < 4 {
			length = 4
		}
		if length > 32 {
			length = 32
		}
		if length > weights[column] {
			weights[column] = length
		}
	}
	for index, header := range headers {
		observe(header, index)
	}
	observeRows := func(rows []ReportRow) {
		for _, row := range rows {
			for index, cell := range row.Cells {
				observe(cell.DisplayText(fallbackCurrency), index)
			}
		}
	}
	observeRows(section.Rows)
	if section.Totals != nil {
		observeRows([]ReportRow{*section.Totals})
	}

	minWidth := 10.0
	if minWidth*float64(columns) > contentWidth {
		minWidth = contentWidth / float64(columns)
	}
	remaining := contentWidth - minWidth*float64(columns)

	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	widths := make([]float64, columns)
	for index, weight := range weights {
		widths[index] = minWidth
		if totalWeight > 0 {
			widths[index] += remaining * weight / totalWeight
		}
	}
	return widths
}
