package services

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

// Shared XLSX palette. The values mirror the PDF renderer palette so both
// formats share one visual identity.
const (
	xlsxGreen     = "15803D"
	xlsxSoftGreen = "DCFCE7"
	xlsxText      = "0F172A"
	xlsxMuted     = "475569"
	xlsxBorder    = "E5E9F0"
	xlsxZebra     = "F8FAFC"
	xlsxDanger    = "B91C1C"
	xlsxWarning   = "B45309"
	xlsxSoftAmber = "FEF3C7"
	xlsxSoftRed   = "FEE2E2"
	xlsxSoftSlate = "F1F5F9"
)

// Currency-aware number formats. Excel requires literal currency letters to be
// quoted, hence the embedded quotes in the lempira format.
const (
	xlsxMoneyFormatHNL = `"L "#,##0.00`
	xlsxMoneyFormatUSD = `"$"#,##0.00`
	xlsxDateFormat     = "dd/mm/yyyy"
	xlsxPercentFormat  = "0.00%"
	xlsxIntegerFormat  = "#,##0"
)

// xlsxCellStyles holds the pre-built cell styles for one row kind, split by
// value type and alignment. All styles are created once from the workbook.
type xlsxCellStyles struct {
	TextLeft   int
	TextCenter int
	TextRight  int
	Integer    int
	Percent    int
	Date       int
	MoneyHNL   int
	MoneyUSD   int
}

// styleFor returns the style ID matching the cell value type and alignment.
// The fallback currency is used for money cells without an explicit currency.
func (s xlsxCellStyles) styleFor(cell ReportCell, fallbackCurrency string) int {
	switch {
	case cell.Money != nil:
		if strings.EqualFold(cell.ResolveCurrency(fallbackCurrency), "USD") {
			return s.MoneyUSD
		}
		return s.MoneyHNL
	case cell.Int != nil:
		return s.Integer
	case cell.Percent != nil:
		return s.Percent
	case cell.Date != nil:
		return s.Date
	default:
		switch cell.Align {
		case AlignRight:
			return s.TextRight
		case AlignCenter:
			return s.TextCenter
		default:
			return s.TextLeft
		}
	}
}

// xlsxStyleSet bundles the shared styles for a workbook: title banner,
// subtitle, metadata, section titles, table headers, body/zebra rows, totals,
// status variants, and typed money/date/percent/integer cells.
type xlsxStyleSet struct {
	Title        int
	Subtitle     int
	Meta         int
	SectionTitle int
	Header       int
	Note         int

	Body     xlsxCellStyles
	Zebra    xlsxCellStyles
	Subtotal xlsxCellStyles
	Total    xlsxCellStyles
	Paid     xlsxCellStyles
	Pending  xlsxCellStyles
	Warning  xlsxCellStyles
	Danger   xlsxCellStyles
	Muted    xlsxCellStyles
}

// rowStyles selects the cell style group for a row kind. Zebra striping only
// applies to normal rows; status/total kinds keep their semantic fill.
func (s *xlsxStyleSet) rowStyles(kind ReportRowKind, zebra bool) xlsxCellStyles {
	switch kind {
	case ReportRowSubtotal:
		return s.Subtotal
	case ReportRowTotal:
		return s.Total
	case ReportRowPaid:
		return s.Paid
	case ReportRowPending:
		return s.Pending
	case ReportRowWarning:
		return s.Warning
	case ReportRowDanger:
		return s.Danger
	case ReportRowMuted:
		return s.Muted
	default:
		if zebra {
			return s.Zebra
		}
		return s.Body
	}
}

// xlsxStyleBuilder accumulates style creation errors so building the style set
// stays a linear, readable sequence.
type xlsxStyleBuilder struct {
	file *excelize.File
	err  error
}

func (b *xlsxStyleBuilder) style(style *excelize.Style) int {
	if b.err != nil {
		return 0
	}
	id, err := b.file.NewStyle(style)
	if err != nil {
		b.err = err
		return 0
	}
	return id
}

// cellStyles builds the typed styles for one font/fill/border combination.
func (b *xlsxStyleBuilder) cellStyles(font excelize.Font, fill excelize.Fill, topBorder bool) xlsxCellStyles {
	borders := make([]excelize.Border, 0, 2)
	if topBorder {
		borders = append(borders,
			excelize.Border{Type: "top", Color: xlsxBorder, Style: 1},
			excelize.Border{Type: "bottom", Color: xlsxBorder, Style: 1},
		)
	}

	base := func(align string, wrap bool) *excelize.Style {
		fontCopy := font
		return &excelize.Style{
			Font:      &fontCopy,
			Fill:      fill,
			Border:    append([]excelize.Border(nil), borders...),
			Alignment: &excelize.Alignment{Horizontal: align, Vertical: "center", WrapText: wrap},
		}
	}
	textStyle := func(align string) int {
		return b.style(base(align, align == "left"))
	}
	numberStyle := func(format string) int {
		numericFormat := format
		style := base("right", false)
		style.CustomNumFmt = &numericFormat
		return b.style(style)
	}

	return xlsxCellStyles{
		TextLeft:   textStyle("left"),
		TextCenter: textStyle("center"),
		TextRight:  textStyle("right"),
		Integer:    numberStyle(xlsxIntegerFormat),
		Percent:    numberStyle(xlsxPercentFormat),
		Date:       numberStyle(xlsxDateFormat),
		MoneyHNL:   numberStyle(xlsxMoneyFormatHNL),
		MoneyUSD:   numberStyle(xlsxMoneyFormatUSD),
	}
}

func xlsxPatternFill(color string) excelize.Fill {
	if color == "" {
		return excelize.Fill{}
	}
	return excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}}
}

// newXLSXStyleSet builds every shared style once for the given workbook.
func newXLSXStyleSet(file *excelize.File) (*xlsxStyleSet, error) {
	if file == nil {
		return nil, fmt.Errorf("xlsx style set requires a workbook")
	}

	builder := &xlsxStyleBuilder{file: file}
	styles := &xlsxStyleSet{}

	styles.Title = builder.style(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 15, Color: xlsxGreen},
		Fill:      xlsxPatternFill(xlsxSoftGreen),
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styles.Subtitle = builder.style(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: xlsxText},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	styles.Meta = builder.style(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: xlsxMuted},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	styles.SectionTitle = builder.style(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 11, Color: xlsxGreen},
		Fill:   xlsxPatternFill(xlsxSoftGreen),
		Border: []excelize.Border{{Type: "bottom", Color: xlsxBorder, Style: 1}},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
	})
	styles.Header = builder.style(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 9, Color: xlsxGreen},
		Fill:      xlsxPatternFill(xlsxSoftGreen),
		Border:    []excelize.Border{{Type: "bottom", Color: xlsxBorder, Style: 1}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	styles.Note = builder.style(&excelize.Style{
		Font:      &excelize.Font{Italic: true, Size: 9, Color: xlsxMuted},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", WrapText: true},
	})

	normalFont := excelize.Font{Size: 10, Color: xlsxText}
	boldFont := excelize.Font{Size: 10, Bold: true, Color: xlsxText}
	greenFont := excelize.Font{Size: 10, Bold: true, Color: xlsxGreen}
	statusGreenFont := excelize.Font{Size: 10, Color: xlsxGreen}
	statusAmberFont := excelize.Font{Size: 10, Bold: true, Color: xlsxWarning}
	warningFont := excelize.Font{Size: 10, Color: xlsxWarning}
	dangerFont := excelize.Font{Size: 10, Bold: true, Color: xlsxDanger}
	mutedFont := excelize.Font{Size: 10, Color: xlsxMuted}

	styles.Body = builder.cellStyles(normalFont, excelize.Fill{}, false)
	styles.Zebra = builder.cellStyles(normalFont, xlsxPatternFill(xlsxZebra), false)
	styles.Subtotal = builder.cellStyles(boldFont, xlsxPatternFill(xlsxSoftSlate), true)
	styles.Total = builder.cellStyles(greenFont, xlsxPatternFill(xlsxSoftGreen), true)
	styles.Paid = builder.cellStyles(statusGreenFont, xlsxPatternFill(xlsxSoftGreen), false)
	styles.Pending = builder.cellStyles(statusAmberFont, xlsxPatternFill(xlsxSoftAmber), false)
	styles.Warning = builder.cellStyles(warningFont, excelize.Fill{}, false)
	styles.Danger = builder.cellStyles(dangerFont, xlsxPatternFill(xlsxSoftRed), false)
	styles.Muted = builder.cellStyles(mutedFont, excelize.Fill{}, false)

	if builder.err != nil {
		return nil, fmt.Errorf("failed to build xlsx styles: %w", builder.err)
	}
	return styles, nil
}

// writeReportDocumentToSheet renders a full ReportDocument onto an existing
// sheet: merged title banner, metadata, ordered sections, table headers, zebra
// rows, totals, note blocks, column widths and a frozen header pane.
func writeReportDocumentToSheet(file *excelize.File, sheet string, doc *ReportDocument) (*xlsxStyleSet, error) {
	if file == nil {
		return nil, fmt.Errorf("xlsx renderer requires a workbook")
	}
	if doc == nil {
		return nil, fmt.Errorf("xlsx renderer requires a report document")
	}

	styles, err := newXLSXStyleSet(file)
	if err != nil {
		return nil, err
	}

	columns := reportDocumentColumnCount(doc)
	widths := make([]float64, columns)
	row := 1
	freezeRow := 0

	observe := func(valueText string, cell *ReportCell, column int) {
		if column < 0 || column >= len(widths) {
			return
		}
		length := float64(utf8.RuneCountInString(valueText))
		switch {
		case cell != nil && cell.Money != nil:
			if length < 14 {
				length = 14
			}
		case cell != nil && cell.Date != nil:
			if length < 11 {
				length = 11
			}
		case cell != nil && cell.Percent != nil:
			if length < 8 {
				length = 8
			}
		}
		if length < 8 {
			length = 8
		}
		if length > 45 {
			length = 45
		}
		if length > widths[column] {
			widths[column] = length
		}
	}

	setCell := func(column, atRow int, value any, styleID int) error {
		name, coordErr := excelize.CoordinatesToCellName(column, atRow)
		if coordErr != nil {
			return coordErr
		}
		if err := file.SetCellValue(sheet, name, value); err != nil {
			return err
		}
		return file.SetCellStyle(sheet, name, name, styleID)
	}

	mergeRow := func(value string, styleID int, height float64) error {
		first, coordErr := excelize.CoordinatesToCellName(1, row)
		if coordErr != nil {
			return coordErr
		}
		last, coordErr := excelize.CoordinatesToCellName(columns, row)
		if coordErr != nil {
			return coordErr
		}
		if columns > 1 {
			if err := file.MergeCell(sheet, first, last); err != nil {
				return err
			}
		}
		if err := file.SetCellValue(sheet, first, value); err != nil {
			return err
		}
		if err := file.SetCellStyle(sheet, first, last, styleID); err != nil {
			return err
		}
		if err := file.SetRowHeight(sheet, row, height); err != nil {
			return err
		}
		row++
		return nil
	}

	writeTableRow := func(cells []ReportCell, kind ReportRowKind, zebra bool) error {
		rowStyles := styles.rowStyles(kind, zebra)
		for column, cell := range cells {
			display := cell.DisplayText(doc.Currency)
			if err := setCell(column+1, row, cell.Value(), rowStyles.styleFor(cell, doc.Currency)); err != nil {
				return err
			}
			observe(display, &cell, column)
		}
		row++
		return nil
	}

	writeHeaders := func(headers []string) error {
		for column, header := range headers {
			if err := setCell(column+1, row, header, styles.Header); err != nil {
				return err
			}
			observe(header, nil, column)
		}
		if freezeRow == 0 {
			freezeRow = row
		}
		if err := file.SetRowHeight(sheet, row, 22); err != nil {
			return err
		}
		row++
		return nil
	}

	writeNote := func(note string) error {
		if strings.TrimSpace(note) == "" {
			return nil
		}
		lines := utf8.RuneCountInString(note)/95 + 1
		return mergeRow(note, styles.Note, float64(lines)*14)
	}

	// Title banner and metadata.
	if doc.Title != "" {
		if err := mergeRow(doc.Title, styles.Title, 30); err != nil {
			return nil, err
		}
	}
	if doc.Subtitle != "" {
		if err := mergeRow(doc.Subtitle, styles.Subtitle, 20); err != nil {
			return nil, err
		}
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
		if err := mergeRow(strings.Join(metaParts, " · "), styles.Meta, 18); err != nil {
			return nil, err
		}
	}
	row++

	// Sections.
	for sectionIndex, section := range doc.Sections {
		if sectionIndex > 0 {
			row++
		}
		if section.Title != "" {
			if err := mergeRow(section.Title, styles.SectionTitle, 22); err != nil {
				return nil, err
			}
		}

		if section.Kind == ReportSectionNote {
			if err := writeNote(section.Note); err != nil {
				return nil, err
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
			if err := writeHeaders(headers); err != nil {
				return nil, err
			}
		}

		for index, reportRow := range section.Rows {
			if err := writeTableRow(reportRow.Cells, reportRow.Kind, index%2 == 1); err != nil {
				return nil, err
			}
		}

		if section.Totals != nil {
			kind := section.Totals.Kind
			if kind == "" {
				kind = ReportRowTotal
			}
			if err := writeTableRow(section.Totals.Cells, kind, false); err != nil {
				return nil, err
			}
		}

		if err := writeNote(section.Note); err != nil {
			return nil, err
		}
	}

	// Column widths.
	for index, width := range widths {
		if width <= 0 {
			width = 12
		}
		columnName, coordErr := excelize.ColumnNumberToName(index + 1)
		if coordErr != nil {
			return nil, coordErr
		}
		if err := file.SetColWidth(sheet, columnName, columnName, width); err != nil {
			return nil, err
		}
	}

	// Freeze the banner and the first header row of the sheet.
	if freezeRow > 0 {
		topLeft, coordErr := excelize.CoordinatesToCellName(1, freezeRow+1)
		if coordErr != nil {
			return nil, coordErr
		}
		if err := file.SetPanes(sheet, &excelize.Panes{
			Freeze:      true,
			YSplit:      freezeRow,
			TopLeftCell: topLeft,
			ActivePane:  "bottomLeft",
		}); err != nil {
			return nil, err
		}
	}

	return styles, nil
}

// reportDocumentColumnCount returns the widest row across every section.
func reportDocumentColumnCount(doc *ReportDocument) int {
	columns := 0
	for _, section := range doc.Sections {
		if len(section.Headers) > columns {
			columns = len(section.Headers)
		}
		for _, row := range section.Rows {
			if len(row.Cells) > columns {
				columns = len(row.Cells)
			}
		}
		if section.Totals != nil && len(section.Totals.Cells) > columns {
			columns = len(section.Totals.Cells)
		}
	}
	if columns < 1 {
		columns = 1
	}
	return columns
}

// sanitizeXLSXSheetName removes characters Excel rejects and caps the name at
// 31 characters.
func sanitizeXLSXSheetName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.NewReplacer(
		"[", "", "]", "", ":", "", "*", "", "?", "", "/", "", "\\", "",
	).Replace(name)
	name = strings.Trim(name, "'")
	name = strings.TrimSpace(name)
	if name == "" {
		return "Reporte"
	}
	runes := []rune(name)
	if len(runes) > 31 {
		runes = runes[:31]
	}
	return strings.TrimSpace(string(runes))
}
