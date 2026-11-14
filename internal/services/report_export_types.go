package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ReportDocument is the neutral, renderer-agnostic representation of a
// financial report. Report builders produce one document per report and the
// XLSX, CSV and PDF renderers consume it, so every format shows the same
// metadata, sections and totals.
type ReportDocument struct {
	Title    string
	Subtitle string
	// Period is the human-readable period label, e.g. "Octubre 2026".
	Period string
	// PeriodKey is the filename-safe period, e.g. "2026-10". When empty the
	// GeneratedAt month is used instead.
	PeriodKey string
	// BaseName is the Spanish filename base without the period or extension,
	// e.g. "reporte_operaciones". See Filename.
	BaseName    string
	GeneratedAt time.Time
	// Currency is the document-level default (HNL or USD) used by money cells
	// that do not carry their own currency.
	Currency string
	Sections []ReportSection
}

// Filename returns the download filename for the document, combining the
// Spanish base name and the period, e.g. "reporte_operaciones_2026-10.xlsx".
// The extension is normalised to lower case and a leading dot is optional.
func (d *ReportDocument) Filename(ext string) string {
	if d == nil {
		return ""
	}

	base := sanitizeFilenameSegment(d.BaseName)
	if base == "" {
		base = "reporte"
	}

	period := sanitizeFilenameSegment(d.PeriodKey)
	if period == "" && !d.GeneratedAt.IsZero() {
		period = d.GeneratedAt.Format("2006-01")
	}
	if period == "" {
		period = "periodo"
	}

	name := base + "_" + period
	extension := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ext), ".")))
	if extension == "" {
		return name
	}
	return name + "." + extension
}

// ReportSection is one ordered block of a report. Summary and stats sections
// render label/value pairs, table sections render Headers + Rows, and note
// sections render free text.
type ReportSection struct {
	Title   string
	Kind    ReportSectionKind
	Headers []string
	Rows    []ReportRow
	Totals  *ReportRow
	Note    string
}

// ReportSectionKind identifies how a section should be laid out.
type ReportSectionKind string

const (
	// ReportSectionSummary is a label/value summary block.
	ReportSectionSummary ReportSectionKind = "summary"
	// ReportSectionStats is a grid of highlighted metrics.
	ReportSectionStats ReportSectionKind = "stats"
	// ReportSectionTable is a regular table with optional headers and totals.
	ReportSectionTable ReportSectionKind = "table"
	// ReportSectionNote is free-form text.
	ReportSectionNote ReportSectionKind = "note"
)

// ReportRow is one row of a section. Kind controls the visual treatment
// (normal, subtotal, total, status colors) and is shared by all renderers.
type ReportRow struct {
	Cells []ReportCell
	Kind  ReportRowKind
}

// ReportRowKind describes the semantic role and style of a table row.
type ReportRowKind string

const (
	ReportRowNormal   ReportRowKind = "normal"
	ReportRowSubtotal ReportRowKind = "subtotal"
	ReportRowTotal    ReportRowKind = "total"
	ReportRowPaid     ReportRowKind = "paid"
	ReportRowPending  ReportRowKind = "pending"
	ReportRowWarning  ReportRowKind = "warning"
	ReportRowDanger   ReportRowKind = "danger"
	ReportRowMuted    ReportRowKind = "muted"
)

// ReportAlign is the horizontal alignment of a report cell.
type ReportAlign string

const (
	AlignLeft   ReportAlign = "left"
	AlignCenter ReportAlign = "center"
	AlignRight  ReportAlign = "right"
)

// ReportCell carries exactly one typed value plus its presentation hints.
// Renderers use the typed value (numeric money stays numeric in XLSX) and the
// helper methods for text output.
type ReportCell struct {
	Text     string
	Money    *float64
	Int      *int
	Percent  *float64
	Date     *time.Time
	Currency string
	Align    ReportAlign
}

// TextCell returns a plain text cell, left aligned by default.
func TextCell(text string) ReportCell {
	return ReportCell{Text: text, Align: AlignLeft}
}

// MoneyCell returns a monetary cell in the given currency (HNL when empty).
// The amount stays numeric for XLSX and is formatted with two decimals by the
// text renderers.
func MoneyCell(amount float64, currency string) ReportCell {
	return ReportCell{Money: &amount, Currency: currency, Align: AlignRight}
}

// IntCell returns an integer cell, right aligned.
func IntCell(value int) ReportCell {
	return ReportCell{Int: &value, Align: AlignRight}
}

// PercentCell returns a percentage cell. The value is a ratio where 1.0 means
// 100%, matching Excel's "0.00%" number format.
func PercentCell(value float64) ReportCell {
	return ReportCell{Percent: &value, Align: AlignRight}
}

// DateCell returns a date cell, centered.
func DateCell(value time.Time) ReportCell {
	return ReportCell{Date: &value, Align: AlignCenter}
}

// WithAlign overrides the horizontal alignment and returns the cell.
func (c ReportCell) WithAlign(align ReportAlign) ReportCell {
	c.Align = align
	return c
}

// IsZero reports whether the cell carries no value at all.
func (c ReportCell) IsZero() bool {
	return c.Text == "" && c.Money == nil && c.Int == nil && c.Percent == nil && c.Date == nil
}

// ResolveCurrency returns the cell currency, falling back to the document
// currency and finally to HNL.
func (c ReportCell) ResolveCurrency(fallback string) string {
	switch {
	case c.Currency != "":
		return c.Currency
	case fallback != "":
		return fallback
	default:
		return "HNL"
	}
}

// Value returns the typed value used by XLSX export. A cell without a value
// yields an empty string so the sheet keeps an empty but styled cell.
func (c ReportCell) Value() any {
	switch {
	case c.Money != nil:
		return *c.Money
	case c.Int != nil:
		return *c.Int
	case c.Percent != nil:
		return *c.Percent
	case c.Date != nil:
		return *c.Date
	default:
		return c.Text
	}
}

// DisplayText renders the cell as human-readable Spanish text. Money includes
// the currency symbol, dates use dd/mm/yyyy, and percentages use two decimals
// plus the percent sign.
func (c ReportCell) DisplayText(fallbackCurrency string) string {
	switch {
	case c.Money != nil:
		return FormatReportMoney(c.ResolveCurrency(fallbackCurrency), *c.Money)
	case c.Int != nil:
		return strconv.Itoa(*c.Int)
	case c.Percent != nil:
		return FormatReportPercent(*c.Percent)
	case c.Date != nil:
		return FormatReportDate(*c.Date)
	default:
		return c.Text
	}
}

// DisplayTextPlain renders the cell without a currency symbol, suitable for
// CSV output. Dates are ISO (2006-01-02) and percentages keep two decimals.
func (c ReportCell) DisplayTextPlain() string {
	switch {
	case c.Money != nil:
		return FormatReportMoneyPlain(*c.Money)
	case c.Int != nil:
		return strconv.Itoa(*c.Int)
	case c.Percent != nil:
		return FormatReportPercent(*c.Percent)
	case c.Date != nil:
		return FormatReportDateISO(*c.Date)
	default:
		return c.Text
	}
}

// FormatReportMoney renders an amount with the currency symbol and thousands
// separators, e.g. "L 1,125,000.00" or "$ 1,125,000.00". It mirrors
// ReportService.formatMoneyCurrency for use outside the service.
func FormatReportMoney(currency string, amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	raw := fmt.Sprintf("%.2f", amount)
	parts := strings.SplitN(raw, ".", 2)

	var formatted strings.Builder
	for index, char := range parts[0] {
		if index > 0 && (len(parts[0])-index)%3 == 0 {
			formatted.WriteByte(',')
		}
		formatted.WriteRune(char)
	}

	symbol := "L"
	if strings.EqualFold(currency, "USD") {
		symbol = "$"
	}
	return fmt.Sprintf("%s%s %s.%s", sign, symbol, formatted.String(), parts[1])
}

// FormatReportMoneyPlain renders an amount with two decimals and no currency
// symbol, e.g. "1250000.00". This is the CSV-safe numeric variant.
func FormatReportMoneyPlain(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// FormatReportPercent renders a ratio as a Spanish percentage with two
// decimals, e.g. 0.785 -> "78.50%".
func FormatReportPercent(value float64) string {
	return fmt.Sprintf("%.2f%%", value*100)
}

// FormatReportDate renders a date for display as dd/mm/yyyy.
func FormatReportDate(value time.Time) string {
	return value.Format("02/01/2006")
}

// FormatReportDateISO renders a date as 2006-01-02 for CSV output.
func FormatReportDateISO(value time.Time) string {
	return value.Format("2006-01-02")
}

// sanitizeFilenameSegment lowercases a value and keeps only filename-safe
// characters (a-z, 0-9, underscore and dash), folding common Spanish accents.
func sanitizeFilenameSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u",
		"ä", "a", "ë", "e", "ï", "i", "ö", "o",
	).Replace(value)

	var built strings.Builder
	lastSeparator := false
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			built.WriteRune(char)
			lastSeparator = false
		case char == '-':
			built.WriteRune(char)
			lastSeparator = false
		case char == '_' || char == ' ' || char == '.' || char == '/':
			if !lastSeparator && built.Len() > 0 {
				built.WriteByte('_')
				lastSeparator = true
			}
		}
	}
	return strings.Trim(built.String(), "_")
}
