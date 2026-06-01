package output

import (
	"fmt"
	"io"
	"regexp"
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

// RenderBorderedTable writes a table with ASCII borders for dense result views
// where column boundaries matter more than the default compact gh-style table.
func RenderBorderedTable(w io.Writer, columns []Column, rows [][]string) {
	if len(columns) == 0 {
		return
	}
	widths := tableWidths(columns, rows)
	printBorder := func() {
		fmt.Fprint(w, "+")
		for _, width := range widths {
			fmt.Fprint(w, strings.Repeat("-", width+2))
			fmt.Fprint(w, "+")
		}
		fmt.Fprintln(w)
	}
	printRow := func(cells []string) {
		fmt.Fprint(w, "|")
		for i := range columns {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			fmt.Fprintf(w, " %s |", padRightVisible(cell, widths[i]))
		}
		fmt.Fprintln(w)
	}

	headers := make([]string, len(columns))
	for i, column := range columns {
		headers[i] = strings.ToUpper(column.Header)
	}
	printBorder()
	printRow(headers)
	printBorder()
	for _, row := range rows {
		printRow(row)
	}
	printBorder()
}

func tableWidths(columns []Column, rows [][]string) []int {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = visibleLen(c.Header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && visibleLen(cell) > widths[i] {
				widths[i] = visibleLen(cell)
			}
		}
	}
	return widths
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padRightVisible(s string, width int) string {
	if visibleLen(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen(s))
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visibleLen(s string) int {
	return len(ansiEscapePattern.ReplaceAllString(s, ""))
}
