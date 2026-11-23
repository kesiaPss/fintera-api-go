package services

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
)

// renderReportCSV renders a report document as plain CSV into memory. It is a
// thin wrapper over renderReportCSVTo for tests and callers that need bytes.
func renderReportCSV(doc *ReportDocument) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := renderReportCSVTo(buffer, doc); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// renderReportCSVTo renders a report document as plain CSV directly into w. It
// keeps the same metadata, sections and totals as the XLSX/PDF renderers, but
// emits Spanish headers as data rows with numeric money values (no currency
// symbol) and ISO dates. It never adds visual styles.
func renderReportCSVTo(w io.Writer, doc *ReportDocument) error {
	if doc == nil {
		return fmt.Errorf("csv renderer requires a report document")
	}

	writer := csv.NewWriter(w)

	write := func(record ...string) error {
		return writer.Write(record)
	}

	if err := write(doc.Title); err != nil {
		return err
	}
	if doc.Subtitle != "" {
		if err := write(doc.Subtitle); err != nil {
			return err
		}
	}
	meta := make([][2]string, 0, 3)
	if doc.Period != "" {
		meta = append(meta, [2]string{"Periodo", doc.Period})
	}
	if !doc.GeneratedAt.IsZero() {
		meta = append(meta, [2]string{"Generado", doc.GeneratedAt.Format("2006-01-02 15:04")})
	}
	if doc.Currency != "" {
		meta = append(meta, [2]string{"Moneda", doc.Currency})
	}
	for _, entry := range meta {
		if err := write(entry[0], entry[1]); err != nil {
			return err
		}
	}

	for _, section := range doc.Sections {
		if err := write(); err != nil {
			return err
		}
		if section.Title != "" {
			if err := write(section.Title); err != nil {
				return err
			}
		}

		if section.Kind == ReportSectionNote {
			if section.Note != "" {
				if err := write(section.Note); err != nil {
					return err
				}
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
			if err := write(headers...); err != nil {
				return err
			}
		}

		for _, row := range section.Rows {
			if err := writeReportCSVRow(writer, row.Cells); err != nil {
				return err
			}
		}
		if section.Totals != nil {
			if err := writeReportCSVRow(writer, section.Totals.Cells); err != nil {
				return err
			}
		}
		if section.Note != "" {
			if err := write(section.Note); err != nil {
				return err
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to write csv report: %w", err)
	}
	return nil
}

func writeReportCSVRow(writer *csv.Writer, cells []ReportCell) error {
	record := make([]string, 0, len(cells))
	for _, cell := range cells {
		record = append(record, cell.DisplayTextPlain())
	}
	return writer.Write(record)
}
