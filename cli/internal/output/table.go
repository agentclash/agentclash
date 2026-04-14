package output

import (
	"fmt"
	"io"
	"strings"
)

// Column defines a table column.
type Column struct {
	Header string
	Align  int // 0 = left (default), 1 = right, 2 = center
}

// RenderTable writes a clean, minimal table to w (gh-style, no borders).
func RenderTable(w io.Writer, columns []Column, rows [][]string) {
	if len(columns) == 0 {
		return
	}

	// Calculate column widths.
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c.Header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header.
	for i, c := range columns {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, padRight(strings.ToUpper(c.Header), widths[i]))
	}
	fmt.Fprintln(w)

	// Print rows.
	for _, row := range rows {
		for i := range columns {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprint(w, padRight(cell, widths[i]))
		}
		fmt.Fprintln(w)
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
