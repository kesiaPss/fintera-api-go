package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// renderReportXLSX renders a report document as a styled XLSX workbook in
// memory. It is a thin wrapper over renderReportXLSXTo for tests and callers
// that need the bytes.
func renderReportXLSX(doc *ReportDocument, sheetName string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := renderReportXLSXTo(buffer, doc, sheetName); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// renderReportXLSXTo renders a report document as a styled XLSX workbook
// straight into w. The sheet name is provided by the caller in Spanish (for
// example ReportSheetOperations); an empty name falls back to "Reporte".
func renderReportXLSXTo(w io.Writer, doc *ReportDocument, sheetName string) error {
	if doc == nil {
		return fmt.Errorf("xlsx renderer requires a report document")
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := sanitizeXLSXSheetName(sheetName)
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return fmt.Errorf("failed to name xlsx sheet: %w", err)
	}

	styles, err := newXLSXStyleSet(file)
	if err != nil {
		return err
	}

	renderer, err := newReportXLSTreamWriter(file, sheet, doc, styles)
	if err != nil {
		return err
	}
	if err := renderer.renderDocument(); err != nil {
		return err
	}
	if err := renderer.flush(); err != nil {
		return err
	}

	if err := file.Write(w); err != nil {
		return fmt.Errorf("failed to write xlsx report: %w", err)
	}
	return nil
}

// reportXLSTreamWriter writes a report document through excelize's
// StreamWriter so rows stream to a buffer instead of building the whole sheet
// in memory.
//
// Important: with StreamWriter, sheet-level XML is serialized by the stream
// itself, so every property must go through the StreamWriter API
// (sw.MergeCell, sw.SetColWidth, RowOpts heights, sw.SetPanes). The
// equivalent functions on *excelize.File operate on a worksheet instance the
// stream never writes and are silently lost. SetPanes additionally has to run
// before the first SetRow, which is why the freeze row is computed from the
// document layout up front (reportDocumentFreezeRow mirrors the row counting
// in renderDocument; the XLSX structure test guards against drift).
type reportXLSTreamWriter struct {
	file   *excelize.File
	stream *excelize.StreamWriter
	sheet  string
	doc    *ReportDocument
	styles *xlsxStyleSet

	columns int
	row     int
}

func newReportXLSTreamWriter(file *excelize.File, sheet string, doc *ReportDocument, styles *xlsxStyleSet) (*reportXLSTreamWriter, error) {
	stream, err := file.NewStreamWriter(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create xlsx stream writer: %w", err)
	}
	return &reportXLSTreamWriter{
		file:    file,
		stream:  stream,
		sheet:   sheet,
		doc:     doc,
		styles:  styles,
		columns: reportDocumentColumnCount(doc),
		row:     1,
	}, nil
}

func (r *reportXLSTreamWriter) setRow(values []interface{}, height float64) error {
	origin, err := excelize.CoordinatesToCellName(1, r.row)
	if err != nil {
		return err
	}
	if height > 0 {
		err = r.stream.SetRow(origin, values, excelize.RowOpts{Height: height})
	} else {
		err = r.stream.SetRow(origin, values)
	}
	if err != nil {
		return fmt.Errorf("failed to stream xlsx row %d: %w", r.row, err)
	}
	r.row++
	return nil
}

// mergedRow writes a full-width row whose single value is merged across all
// columns and styled in every cell so the fill spans the merged range.
func (r *reportXLSTreamWriter) mergedRow(value string, styleID int, height float64) error {
	values := make([]interface{}, 0, r.columns)
	for column := 0; column < r.columns; column++ {
		cell := excelize.Cell{StyleID: styleID}
		if column == 0 {
			cell.Value = value
		}
		values = append(values, cell)
	}

	if r.columns > 1 {
		first, err := excelize.CoordinatesToCellName(1, r.row)
		if err != nil {
			return err
		}
		last, err := excelize.CoordinatesToCellName(r.columns, r.row)
		if err != nil {
			return err
		}
		if err := r.stream.MergeCell(first, last); err != nil {
			return fmt.Errorf("failed to merge xlsx row %d: %w", r.row, err)
		}
	}

	return r.setRow(values, height)
}

// tableRow writes one report row with per-cell styles and zebra striping.
func (r *reportXLSTreamWriter) tableRow(cells []ReportCell, kind ReportRowKind, zebra bool) error {
	rowStyles := r.styles.rowStyles(kind, zebra)
	values := make([]interface{}, 0, len(cells))
	for _, cell := range cells {
		values = append(values, excelize.Cell{
			StyleID: rowStyles.styleFor(cell, r.doc.Currency),
			Value:   cell.Value(),
		})
	}
	return r.setRow(values, 0)
}

// headerRow writes one styled header row at the fixed header height.
func (r *reportXLSTreamWriter) headerRow(headers []string) error {
	values := make([]interface{}, 0, len(headers))
	for _, header := range headers {
		values = append(values, excelize.Cell{StyleID: r.styles.Header, Value: header})
	}
	return r.setRow(values, 22)
}

func (r *reportXLSTreamWriter) writeNote(note string) error {
	if strings.TrimSpace(note) == "" {
		return nil
	}
	lines := utf8.RuneCountInString(note)/95 + 1
	return r.mergedRow(note, r.styles.Note, float64(lines)*14)
}

// renderDocument writes column widths, the frozen pane and then the banner,
// metadata and every section in order.
func (r *reportXLSTreamWriter) renderDocument() error {
	for index, width := range reportDocumentColumnWidths(r.doc, r.columns) {
		if width <= 0 {
			width = 12
		}
		if err := r.stream.SetColWidth(index+1, index+1, width); err != nil {
			return fmt.Errorf("failed to set xlsx column width: %w", err)
		}
	}

	if freezeRow := reportDocumentFreezeRow(r.doc); freezeRow > 0 {
		topLeft, err := excelize.CoordinatesToCellName(1, freezeRow+1)
		if err != nil {
			return err
		}
		if err := r.stream.SetPanes(&excelize.Panes{
			Freeze:      true,
			YSplit:      freezeRow,
			TopLeftCell: topLeft,
			ActivePane:  "bottomLeft",
		}); err != nil {
			return fmt.Errorf("failed to freeze the xlsx header pane: %w", err)
		}
	}

	if r.doc.Title != "" {
		if err := r.mergedRow(r.doc.Title, r.styles.Title, 30); err != nil {
			return err
		}
	}
	if r.doc.Subtitle != "" {
		if err := r.mergedRow(r.doc.Subtitle, r.styles.Subtitle, 20); err != nil {
			return err
		}
	}

	metaParts := make([]string, 0, 3)
	if r.doc.Period != "" {
		metaParts = append(metaParts, "Periodo: "+r.doc.Period)
	}
	if !r.doc.GeneratedAt.IsZero() {
		metaParts = append(metaParts, "Generado el "+r.doc.GeneratedAt.Format("02/01/2006 15:04"))
	}
	if r.doc.Currency != "" {
		metaParts = append(metaParts, "Moneda: "+r.doc.Currency)
	}
	if len(metaParts) > 0 {
		if err := r.mergedRow(strings.Join(metaParts, " · "), r.styles.Meta, 18); err != nil {
			return err
		}
	}
	r.row++

	for sectionIndex, section := range r.doc.Sections {
		if sectionIndex > 0 {
			r.row++
		}
		if section.Title != "" {
			if err := r.mergedRow(section.Title, r.styles.SectionTitle, 22); err != nil {
				return err
			}
		}

		if section.Kind == ReportSectionNote {
			if err := r.writeNote(section.Note); err != nil {
				return err
			}
			continue
		}

		headers := section.Headers
		if len(headers) == 0 {
			switch section.Kind {
			case ReportSectionSummary:
				headers = []string{"Concepto", "Valor"}
			case ReportSectionStats:
				headers = []string{"Métrica", "Valor"}
			}
		}
		if len(headers) > 0 {
			if err := r.headerRow(headers); err != nil {
				return err
			}
		}

		for index, reportRow := range section.Rows {
			if err := r.tableRow(reportRow.Cells, reportRow.Kind, index%2 == 1); err != nil {
				return err
			}
		}

		if section.Totals != nil {
			kind := section.Totals.Kind
			if kind == "" {
				kind = ReportRowTotal
			}
			if err := r.tableRow(section.Totals.Cells, kind, false); err != nil {
				return err
			}
		}

		if err := r.writeNote(section.Note); err != nil {
			return err
		}
	}
	return nil
}

func (r *reportXLSTreamWriter) flush() error {
	if err := r.stream.Flush(); err != nil {
		return fmt.Errorf("failed to flush the xlsx stream writer: %w", err)
	}
	return nil
}

// reportDocumentFreezeRow mirrors the row counting of renderDocument and
// returns the row number of the first header row, or 0 when the document has
// no headers. StreamWriter requires SetPanes before the first SetRow, so the
// position must be known before any row is written. Keep this in sync with
// renderDocument; TestRenderReportXLSXStreamStructure locks the frozen pane.
func reportDocumentFreezeRow(doc *ReportDocument) int {
	row := 1
	if doc.Title != "" {
		row++
	}
	if doc.Subtitle != "" {
		row++
	}
	if doc.Period != "" || !doc.GeneratedAt.IsZero() || doc.Currency != "" {
		row++
	}
	row++ // blank spacer after the metadata line

	for index, section := range doc.Sections {
		if index > 0 {
			row++
		}
		if section.Title != "" {
			row++
		}
		if section.Kind == ReportSectionNote {
			continue
		}

		headers := section.Headers
		if len(headers) == 0 {
			switch section.Kind {
			case ReportSectionSummary:
				headers = []string{"Concepto", "Valor"}
			case ReportSectionStats:
				headers = []string{"Métrica", "Valor"}
			}
		}
		if len(headers) > 0 {
			return row
		}
	}
	return 0
}
