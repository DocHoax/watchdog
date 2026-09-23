package util

import (
	"fmt"
	"io"
	"strings"
)

// TableWriter formats rows and columns into cleanly aligned CLI text.
type TableWriter struct {
	headers []string
	rows    [][]string
}

// NewTableWriter creates a new table writer.
func NewTableWriter(headers ...string) *TableWriter {
	return &TableWriter{
		headers: headers,
		rows:    make([][]string, 0),
	}
}

// Append adds a row of values.
func (t *TableWriter) Append(cols ...string) {
	t.rows = append(t.rows, cols)
}

// Render writes the formatted table to w.
func (t *TableWriter) Render(w io.Writer) {
	colCount := len(t.headers)
	colWidths := make([]int, colCount)

	for i, h := range t.headers {
		colWidths[i] = len(h)
	}

	for _, row := range t.rows {
		for i, col := range row {
			if i < colCount && len(col) > colWidths[i] {
				colWidths[i] = len(col)
			}
		}
	}

	// Print headers
	for i, h := range t.headers {
		fmt.Fprintf(w, "%-*s", colWidths[i]+2, strings.ToUpper(h))
	}
	fmt.Fprintln(w)

	// Print separator
	for i := range t.headers {
		fmt.Fprintf(w, "%s  ", strings.Repeat("-", colWidths[i]))
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range t.rows {
		for i := 0; i < colCount; i++ {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			fmt.Fprintf(w, "%-*s", colWidths[i]+2, val)
		}
		fmt.Fprintln(w)
	}
}
